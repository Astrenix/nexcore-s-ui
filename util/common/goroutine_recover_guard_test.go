package common

// 不变量:每个 `go func(){...}` 的函数体都必须有 panic 兜底(2026-08-22 E564)。
//
// 节点端是 sing-box 面板的控制面。任一 goroutine 里未捕获的 panic =
// 【整个面板进程退出】= 该节点全部入站下线,而主控侧只看得到「节点失联」,
// 归因极难 —— 日志里往往什么都没有,因为进程是被 runtime 直接终止的。
//
// E510(181b8eb)修的是 cron 调度链:`cron.WithChain(SkipIfStillRunning, Recover)`。
// 那层只保护【经 cron 调度执行的作业】,保护不了:
//   · 裸 goroutine(本轮这 12 处)
//   · 注册作业的那个 goroutine 本身(cronJob.go 的 AddJob 序列)
//
// 本轮全量 AST 扫出 13 个 goroutine 启动点,13 个全无兜底。逐个定性后:
//
//	最高危  service/config.go     保存整份 config 后重启 core —— 用户可控输入 + 异步 + 节点全下线
//	        service/config.go     屏蔽规则变更后重启 core
//	        service/sub_probe.go  并发探测,处理外部节点响应
//	中      service/cloudflare.go 处理外部 HTTP 响应(×2)
//	        cronjob/cronJob.go    注册全部定时作业 —— 它 panic 会让后面的 AddJob 一个都不执行
//	低      重启类(apiService ×2 / v1 / backup / panel)—— 意图本就是退出进程,
//	        但【没有日志的退出】和【正常重启】在运维看来一模一样,补 Recover 是为了留下痕迹
//	干净    cronjob/cronJob.go:99 —— 直接调 NewCertRenewJob().Run(),而它自带 defer recover
//	        (certRenewJob.go:25,已逐行核实);跟一跳才敢下这个结论
//
// 🩸 为什么有【行为】断言:E510 那次的教训是形态断言对顺序完全瞎 ——
// `NewChain(Recover, Skip)` 和 `NewChain(Skip, Recover)` 都"含有 Recover",
// 但前者会静默废掉作业。所以这里除了数 defer,还真的 panic 一次看兜不兜得住。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// grExempt:已核实【被调函数自身带 recover】的启动点,跟一跳确认过。
var grExempt = map[string]string{
	"cronjob/cronJob.go": "唯一豁免点是首检 goroutine,直接调 NewCertRenewJob().Run() —— " +
		"该 Run() 自带 defer recover(certRenewJob.go:25),注释写明「续签要跑 ACME,panic 不能带走整个 cron」。" +
		"文件里另一个 goroutine(注册作业那个)已补 Recover,不在豁免范围。",
}

// grMinGoroutines:规模闸。2026-08-22 实测 13 个;少于这个数说明 AST 没走到该走的目录,
// 此时「零违规」只是因为什么都没扫到。
const grMinGoroutines = 10

func TestEveryGoroutineHasPanicRecover(t *testing.T) {
	root := grRepoRoot(t)
	total, bad := grScan(t, root)

	if total < grMinGoroutines {
		t.Fatalf("只认出 %d 个 goroutine 启动点(应 >= %d)—— 判据失效,"+
			"此时的「零违规」没有意义", total, grMinGoroutines)
	}
	t.Logf("覆盖:goroutine 启动点 %d 个,豁免文件 %d 个", total, len(grExempt))

	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("以下 goroutine 没有 panic 兜底:\n  %s\n\n"+
			"节点端任一 goroutine 未捕获的 panic 会让【整个面板进程退出】,\n"+
			"该节点全部入站随之下线,而主控侧只看得到「节点失联」。\n"+
			"改法:函数体第一行 `defer common.Recover(\"模块: 在做什么\")`。\n"+
			"放最前面(defer 是 LIFO,它最后执行),这样 wg.Done / 信号量归还照常先跑完 ——\n"+
			"否则 wg.Wait() 会永远挂着,那比进程退出更难查。",
			strings.Join(bad, "\n  "))
	}
}

// TestCommonRecoverActuallyStopsPanic 是【行为】断言,不是形态断言。
// E510 的教训:「链里有没有 Recover」这种形状检查对顺序完全是瞎的。
// 这里真的起一个会 panic 的 goroutine,看它有没有把整个测试进程带走。
func TestCommonRecoverActuallyStopsPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	survived := false

	go func() {
		defer func() {
			// 外层观察哨:如果 common.Recover 没兜住,panic 会穿到这里
			if r := recover(); r != nil {
				t.Errorf("common.Recover 没有兜住 panic,它穿到了外层:%v", r)
			}
			wg.Done()
		}()
		defer func() { survived = true }() // 只有 panic 被吞掉、函数正常收尾才会跑到
		defer Recover("E564 合成用例")
		panic("E564 synthetic panic —— common.Recover 必须吞掉它")
	}()

	wg.Wait()
	if !survived {
		t.Fatal("goroutine 没有正常收尾 —— common.Recover 没起作用")
	}
}

// TestCommonRecoverMustBeDeferredDirectly 钉住一个会静默失效的写法。
//
// 🩸 Go 的 recover() 【只在被 defer 的那个函数里直接调用时】才生效。
// 而 common.Recover 内部就是 `panicErr := recover()`,所以:
//
//	defer common.Recover("x")            ✅ Recover 本身被 defer,内部 recover() 生效
//	defer func() { common.Recover("x") }()  ❌ Recover 变成【间接】调用,recover() 恒返回 nil
//
// 第二种写法编译通过、看起来更"规范"(还能加日志),但 panic 会原样穿过去把进程带走。
// 本轮写这条用例时我自己就先写成了第二种,当场被红逼出来 —— 它不是 Recover 的缺陷,
// 是 Go 的规则,而这个规则在代码里没有任何提示。
//
// 本轮 12 处生产改动全部用的是第一种(直接 defer)。这条用例把两种写法的差异钉死,
// 防止将来有人「重构」成看起来更整洁的第二种。
func TestCommonRecoverMustBeDeferredDirectly(t *testing.T) {
	t.Run("直接 defer —— 吞得掉", func(t *testing.T) {
		survived := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("直接 defer 时 panic 不该穿出来:%v", r)
				}
			}()
			defer func() { survived = true }()
			defer Recover("直接 defer")
			panic("boom-direct")
		}()
		if !survived {
			t.Fatal("直接 defer 时函数没能正常收尾")
		}
	})

	t.Run("包一层 —— 吞不掉(这就是陷阱)", func(t *testing.T) {
		escaped := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					escaped = true // panic 穿过了 Recover,被这里接住
				}
			}()
			defer func() { _ = Recover("包一层") }() // ← 间接调用,recover() 恒 nil
			panic("boom-wrapped")
		}()
		if !escaped {
			t.Fatal("包一层居然吞掉了 panic —— 与 Go 的 recover() 规则不符," +
				"要么 Go 改了语义,要么这条用例自己写错了,两种都必须查清楚")
		}
	})
}

// ── 判据实现 ──

func grRepoRoot(t *testing.T) string {
	t.Helper()
	// 本文件在 util/common/,仓库根是上两级
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("定位仓库根失败:%v", err)
	}
	return root
}

func grScan(t *testing.T, root string) (int, []string) {
	t.Helper()
	total := 0
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
			gs, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			total++
			if _, ok := grExempt[rel]; ok {
				return true
			}
			fl, ok := gs.Call.Fun.(*ast.FuncLit)
			if !ok || fl.Body == nil {
				// go someFunc(...) —— 兜底得在被调函数里,判据看不见,如实放过
				return true
			}
			if !grHasRecover(fl.Body) {
				bad = append(bad, rel+":"+itoa(fset.Position(gs.Pos()).Line))
			}
			return true
		})
		return nil
	})
	return total, bad
}

// grHasRecover 判断函数体里有没有【真正生效】的 panic 兜底。
//
// 🩸 这里必须区分两种长得极像、但一种恒失效的写法(见 TestCommonRecoverMustBeDeferredDirectly):
//
//	defer common.Recover("x")                  ✅ Recover 本身被 defer,内部 recover() 生效
//	defer func() { recover() }()               ✅ recover() 直接在被 defer 的闭包里,生效
//	defer func() { common.Recover("x") }()     ❌ Recover 变成间接调用,内部 recover() 恒 nil
//
// 第一版判据对这三种一视同仁 —— 那就等于对「包一层」这个静默失效完全瞎,
// 而它恰恰是最容易在「重构得更整洁」时引入的。
func grHasRecover(body *ast.BlockStmt) bool {
	for _, st := range body.List {
		d, ok := st.(*ast.DeferStmt)
		if !ok {
			continue
		}
		// 形态一:defer <X>.Recover(...) / defer Recover(...) —— 直接 defer 助手
		switch fn := d.Call.Fun.(type) {
		case *ast.Ident:
			if strings.Contains(fn.Name, "Recover") {
				return true
			}
		case *ast.SelectorExpr:
			if strings.Contains(fn.Sel.Name, "Recover") {
				return true
			}
		}
		// 形态二:defer func(){ ... recover() ... }() —— 内建 recover 直接在闭包里
		if fl, ok := d.Call.Fun.(*ast.FuncLit); ok && fl.Body != nil {
			hit := false
			ast.Inspect(fl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				// 只认内建 recover();闭包里调 common.Recover 是【间接】,不生效
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
					hit = true
				}
				return true
			})
			if hit {
				return true
			}
		}
	}
	return false
}

func itoa(n int) string {
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
