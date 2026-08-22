package service

// 「没探测」与「探测出来是死的」必须在数据上可区分。
//
// 2026-08-22 E568。ProbeNodes 有两条路径会一条节点都探不成:
//   ① core 不在跑(CheckCoreJob 每 5s 自愈,SubRefreshJob 每 1min 跑,会撞上)
//   ② 调用方给的时间预算耗尽(SubRefreshJob 给 5 分钟,而 subMaxNodes=1000、
//      并发 8、单节点三次代理往返 —— 大订阅必然撞上)
//
// 两条路径原本都只留下 ProbeOutcome.Alive 的零值 false,applyOutcomes 随即
// 把 alive=false 写进库。而 ElectWinners 是 `Where("alive = ?", true)` ——
// 某国家的节点若都落在没探到的那批里,就一个 winner 都选不出来,
// pool-{cc} 出站被摘,路由到该国家的用户直接断线。
//
// 更糟的是它稳定复发:节点顺序来自订阅解析,每轮被跳过的是同一批尾部节点。

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// ① 行为断言:core 不在跑时,每一条都必须是 Skipped,且一条都不能是 Alive。
// 走的是真的 ProbeNodes,不是复刻一份逻辑。
func TestProbeNodesMarksSkippedWhenCoreNotRunning(t *testing.T) {
	saved := corePtr
	t.Cleanup(func() { corePtr = saved })
	corePtr = nil // core 不在跑

	nodes := []ParsedNode{
		{Remark: "a", Type: "vless", Server: "1.2.3.4", ServerPort: 443},
		{Remark: "b", Type: "vless", Server: "5.6.7.8", ServerPort: 443},
	}
	out := ProbeNodes(context.Background(), nodes)

	if len(out) != len(nodes) {
		t.Fatalf("outcomes 数 %d != 节点数 %d", len(out), len(nodes))
	}
	for i, o := range out {
		if !o.Skipped {
			t.Errorf("out[%d] 没标 Skipped —— core 不在跑时一条都没探成,"+
				"不标的话 applyOutcomes 会把【整个订阅】写成 alive=false,"+
				"ElectWinners 随即摘掉所有 pool-{cc} 出站", i)
		}
		if o.Alive {
			t.Errorf("out[%d] 居然 Alive=true —— 根本没探过", i)
		}
	}
}

// ② ctx 预算耗尽那条路径必须走同一个助手(它是唯一会设 Skipped 的地方)。
func TestProbeNodesChecksContextBudget(t *testing.T) {
	body := psFuncBody(t, "sub_probe.go", "ProbeNodes")
	if body == "" {
		t.Fatal("没提取到 ProbeNodes 函数体 —— 判据失效了,不是代码干净")
	}
	if !strings.Contains(body, "ctx.Err() != nil") {
		t.Error("ProbeNodes 的派发循环里没有 ctx.Err() 检查。\n" +
			"ctx 一旦过期,probeOne 里所有派生的 WithTimeout 立刻 done,\n" +
			"剩余节点会在毫秒内全部「探测失败」→ 写成 alive=false → winner 选不出来。")
	}
	if strings.Count(body, "markRemainingSkipped") < 2 {
		t.Errorf("ProbeNodes 里 markRemainingSkipped 只出现 %d 次,应为 2 次"+
			"(core 不在跑 / 预算耗尽)。少一处 = 那条路径上「没测」又被写成「测出来是死的」。",
			strings.Count(body, "markRemainingSkipped"))
	}
}

// ③ 最关键的一条:skipped 那批的 upsert 【不得】覆盖探测列。
// 前两条都只保证 Skipped 这个标记被设上;真正阻止数据被毁的是这里。
func TestSkippedRowsDoNotOverwriteProbeColumns(t *testing.T) {
	cols := psSkippedBranchUpdateColumns(t)
	if len(cols) == 0 {
		t.Fatal("没在 applyOutcomes 的 skippedRows 分支里找到 AssignmentColumns —— " +
			"判据失效了,不是代码干净")
	}
	forbidden := []string{"alive", "exit_ip", "latency_ms", "last_error", "last_check_at"}
	for _, c := range cols {
		for _, f := range forbidden {
			if c == f {
				t.Errorf("没探测的节点被写了 %q 列。\n"+
					"这批节点本轮压根没被探过,写探测列 = 用零值覆盖上一轮的真实结果。\n"+
					"其中 alive 会直接让 ElectWinners 选不出 winner;\n"+
					"last_check_at 则会谎报「刚探过」,让人以为这个 dead 是新鲜的。", c)
			}
		}
	}
}

// psSkippedBranchUpdateColumns 只取 applyOutcomes 里 `if len(skippedRows) > 0`
// 【那个分支体内】的 AssignmentColumns 参数。
//
// 取值范围必须贴着锚点:整个函数体里有两处 AssignmentColumns,取宽了会把
// 正常那批的列表(本来就含 alive)也算进来,断言必然误红;取窄了又会漏掉
// 写在别处的赋值。以 IfStmt 的 Body 为界是这两者之间唯一站得住的边界。
func psSkippedBranchUpdateColumns(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "sub.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 sub.go: %v", err)
	}
	var cols []string
	ast.Inspect(f, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "applyOutcomes" || fd.Body == nil {
			return true
		}
		ast.Inspect(fd.Body, func(m ast.Node) bool {
			ifs, ok := m.(*ast.IfStmt)
			if !ok || ifs.Body == nil {
				return true
			}
			var cond strings.Builder
			ast.Inspect(ifs.Cond, func(c ast.Node) bool {
				if id, ok := c.(*ast.Ident); ok {
					cond.WriteString(id.Name)
				}
				return true
			})
			if !strings.Contains(cond.String(), "skippedRows") {
				return true
			}
			ast.Inspect(ifs.Body, func(b ast.Node) bool {
				call, ok := b.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "AssignmentColumns" || len(call.Args) == 0 {
					return true
				}
				lit, ok := call.Args[0].(*ast.CompositeLit)
				if !ok {
					return true
				}
				for _, e := range lit.Elts {
					if bl, ok := e.(*ast.BasicLit); ok {
						cols = append(cols, strings.Trim(bl.Value, `"`))
					}
				}
				return true
			})
			return true
		})
		return false
	})
	return cols
}

// psFuncBody 用 printer 从不带注释的 AST 重打函数体(理由见
// core_lifecycle_error_guard_test.go 的 clFuncBodySource)。
func psFuncBody(t *testing.T, file, fn string) string {
	t.Helper()
	return clFuncBodySource(t, file, fn)
}
