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
	"go/printer"
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

// ─────────────────────────────────────────────────────────────────────────────
// LastUpdate 归零之后的兜底方向:必须回去查库,不能折叠成「没变化」。
//
// 2026-08-22 E567:LastUpdate 是包级 atomic.Int64,进程一重启就是 0。
// CheckChanges 的 else 分支是 `LastUpdate.Load() > intLu` —— 0 大不过任何正数,
// 所以少了 ==0 那条分支,重启后面板前端会【永远】被告知"没有变化",
// 直到有人手动保存一次配置才恢复。没有报错、没有日志,界面只是一直显示旧配置。
//
// 现有实现是对的:==0 时回去查 Changes 表(持久化的真值),查完顺带把
// LastUpdate 置为当前时间。这条分支承重且此前无人守 —— 它长得像"冗余的
// 初始化判断",正是重构时最容易被"简化"掉的形状。
// ─────────────────────────────────────────────────────────────────────────────
func TestCheckChangesFallsBackToDBWhenLastUpdateIsZero(t *testing.T) {
	body := clFuncBodySource(t, "config.go", "CheckChanges")
	if body == "" {
		t.Fatal("没提取到 CheckChanges 的函数体 —— 判据失效了,不是代码干净")
	}

	// ① 必须存在「LastUpdate 为零」的判断
	if !strings.Contains(body, "LastUpdate.Load() == 0") {
		t.Error("CheckChanges 里找不到 `LastUpdate.Load() == 0` 分支。\n" +
			"LastUpdate 是进程内 atomic,重启即 0;而另一条分支是 `LastUpdate.Load() > intLu`,\n" +
			"0 大不过任何正数 —— 少了这条分支,重启后前端会永远被告知「没有变化」,\n" +
			"直到有人手动保存配置。无报错、无日志,只是界面一直显示旧配置。")
	}
	// ② 那条分支里必须真的去查库(而不是直接 return 一个常量)
	if !strings.Contains(body, "model.Changes{}") {
		t.Error("CheckChanges 的零值分支没有查 Changes 表。\n" +
			"归零之后唯一还靠得住的真值来源就是库;返回常量等于把「不知道」\n" +
			"折叠成一个具体答案,而两个方向各有各的坏法(恒 true = 前端空转刷新,\n" +
			"恒 false = 前端永远看不到新配置)。")
	}
}

// clFuncBodySource 取指定文件里某个函数的【函数体源码】。
//
// 🩸 承重的是【用 printer 从 AST 节点重打】,不是 parse flag。
// 直接按 Pos/End 切原始字节会把注释一起切进来,而修复注释总会引用它修的那段
// 代码 —— 本仓已因此假绿一次、假红一次。判据要看的是代码,不是关于代码的话。
//
// 不带 parser.ParseComments 是顺手为之,但【它并不承重】:printer.Fprint 打印的是
// fd.Body 这个节点,而注释挂在 ast.File 上,给不给 ParseComments 都不会被打进来。
// 这一点是 2026-08-22 E567 变异实测出来的 —— 我第一版注释把因果写在了 flag 上,
// 换成 ParseComments 之后守卫照绿(单道变异不变红只说明"有别的东西兜着"),
// 换成字节切片才红。要判某道是否必要,看的是【只留它】时成不成立。
func clFuncBodySource(t *testing.T, file, fn string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0) // 无 ParseComments
	if err != nil {
		t.Fatalf("解析 %s: %v", file, err)
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn || fd.Body == nil {
			continue
		}
		var sb strings.Builder
		if err := printer.Fprint(&sb, fset, fd.Body); err != nil {
			t.Fatalf("重打 %s 的函数体: %v", fn, err)
		}
		return sb.String()
	}
	return ""
}

// 判据自证:注释里写着断言要找的字符串时,必须【仍然】判为缺失。
// 这一条直接钉住上面那个「不带 ParseComments」的决定 —— 换成带注释解析就会红。
func TestClFuncBodySourceStripsComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.go")
	src := "package fake\n\nfunc Victim() {\n" +
		"\t// 这里本该有 LastUpdate.Load() == 0 的兜底,但其实没写\n" +
		"\tprintln(1)\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("写合成反例: %v", err)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	body := clFuncBodySource(t, "fake.go", "Victim")
	if body == "" {
		t.Fatal("合成反例里没提到函数体 —— helper 失效")
	}
	if strings.Contains(body, "LastUpdate.Load() == 0") {
		t.Fatal("函数体里带上了注释内容 —— 判据会被「关于代码的话」满足," +
			"上面那条 CheckChanges 守卫的绿是假的")
	}
}
