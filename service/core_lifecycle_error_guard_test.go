package service

// 不变量:sing-box core 的【生命周期函数】返回的错误不许被丢弃(2026-08-22 E565)。
//
//	StartCore / StopCore / RestartCore / restartCoreWithConfig
//
// 这四个都是「先 Stop 再 Start」或其组成部分。Start 那半失败 =
// core 已经停了、起不来 = 该节点【全部入站下线】。
// 而触发它们的往往是用户的一次保存操作,响应早就返回了「成功」。
//
// —— 本轮修之前的实际状态 ——
// 同一个 RestartCore(),两条路径守法不同:
//
//	api/apiService.go  RestartSb    err := ...; jsonMsg(c, "restartSb", err)   ✅ 交给用户
//	api/v1/v1.go       coreRestart  if err != nil { Internal(...) }            ✅ 交给主控
//	service/config.go  block-rules 保存后   _ = s.RestartCore()                ❌
//	service/config.go  整份 config 保存后   _ = s.restartCoreWithConfig(...)   ❌
//
// 两条异步路径直接吞。这是 E234/E235 那族「成对路径只守住一半」在节点仓的复现。
//
// 更糟的是其中一条【完全无声】:RestartCore 自己只 return err 不记日志,
// 而 StartCore 的两个失败分支此前只有一个记日志 —— corePtr.Start() 失败记 Error,
// 但 GetConfig("") 失败是裸 return err。于是:
//
//	StopCore() 成功 → 记 "sing-box stopped"
//	StartCore() 的 GetConfig 失败 → 无声
//	err 被 `_ =` 吞掉
//	⇒ 现场只剩一行 "sing-box stopped",看起来像有人主动停的
//
// 排查会被这行日志直接带偏,而真相是节点已经没有任何入站了。
//
// —— 判据钉什么 ——
// 「调用这四个函数时不许用 `_ =` 丢弃返回值」。零容忍。
// 不去管「拿到 err 之后做什么」(记日志 / 返给用户 / 重试都可以)——
// 那是各调用点的语境决定的;但【连拿都不拿】没有任何语境能justify。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// clCoreFuncs:core 生命周期函数。失败即「节点可能已无入站」。
var clCoreFuncs = map[string]bool{
	"StartCore":             true,
	"StopCore":              true,
	"RestartCore":           true,
	"restartCoreWithConfig": true,
}

// clMinCallSites:规模闸。2026-08-22 实测 6 个调用点(2 同步 + 2 异步 + RestartCore 内部 2 个)。
// 少于这个数说明判据没走到该走的目录,此时「零违规」只是什么都没扫到。
const clMinCallSites = 5

func TestCoreLifecycleErrorsAreNeverDiscarded(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("定位仓库根失败:%v", err)
	}
	sites, bad := clScan(t, root)

	if sites < clMinCallSites {
		t.Fatalf("只找到 %d 个 core 生命周期调用点(应 >= %d)—— 判据失效,"+
			"此时的「零违规」没有意义", sites, clMinCallSites)
	}
	t.Logf("覆盖:core 生命周期调用点 %d 个", sites)

	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("以下调用把 core 生命周期函数的错误【丢弃】了:\n  %s\n\n"+
			"这四个函数都是「先 Stop 再 Start」。Start 失败 = core 已停且起不来 =\n"+
			"该节点全部入站下线,而触发它的那次用户操作早已返回「成功」。\n"+
			"至少要 `if err := ...; err != nil { logger.Error(...) }`,\n"+
			"让「节点没了」这件事在日志里留下痕迹 —— 否则现场只剩一行 \"sing-box stopped\",\n"+
			"看起来像有人主动停的。",
			strings.Join(bad, "\n  "))
	}
}

func clScan(t *testing.T, root string) (int, []string) {
	t.Helper()
	sites := 0
	var bad []string
	fset := token.NewFileSet()

	_ = filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case "frontend", ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)

		ast.Inspect(f, func(n ast.Node) bool {
			// 只看赋值语句:`_ = X.StartCore()` 这种形态
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, rhs := range as.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok {
					continue
				}
				name := clCalleeName(call)
				if !clCoreFuncs[name] {
					continue
				}
				sites++
				// 左值全是 `_` 才算丢弃;`err := X()` 是正常接收
				discarded := true
				for _, lhs := range as.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || id.Name != "_" {
						discarded = false
					}
				}
				if discarded {
					bad = append(bad, rel+":"+clItoa(fset.Position(call.Pos()).Line)+"  _ = "+name+"()")
				}
			}
			return true
		})

		// 裸调用语句 `X.StartCore()`(连 _ = 都没有)同样是丢弃
		ast.Inspect(f, func(n ast.Node) bool {
			es, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := clCalleeName(call)
			if !clCoreFuncs[name] {
				return true
			}
			sites++
			bad = append(bad, rel+":"+clItoa(fset.Position(call.Pos()).Line)+"  裸调用 "+name+"()(返回值直接扔了)")
			return true
		})
		return nil
	})
	return sites, bad
}

func clCalleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func clItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
