package service

// 手动事务的 Commit 错误必须被接住 —— 零容忍。
//
// 2026-08-22 E569。节点仓有 6 处手动事务(`tx := db.Begin()` + defer 里
// Commit/Rollback)。其中 provision.go / cloudflare.go 写的是
// `if err := tx.Commit().Error; err != nil` —— 做对了;
// 而 config.go / client.go / stats.go / cmd/migration 是裸 `tx.Commit()`。
//
// 裸调用的后果不是"少记一条日志",是【把失败报成成功】:
//   - config.Save:用户在面板点保存 → 提交失败 → 返回 nil error → 界面显示成功,
//     而 RestartInbounds 已经在事务里把 sing-box reload 成新配置了。
//     运行态是新的、库是旧的,下次读库又变回去,没有任何信号。
//   - client.DepleteClients:配额扣减白跑一轮。
//   - stats.SaveStats:每 10 秒一次、是最频繁的写者,因此最可能撞上写锁;
//     它提交失败 = 那一轮流量统计凭空消失,而计费按它算。
//
// SQLite 是【整库串行写】。DSN 里 _busy_timeout=30000 + _txlock=immediate 是
// 止血(见 database/db.go 的注释:Save 路径包含 corePtr.AddInbound,
// sing-box 复杂入站 reload 3-8 秒,期间写锁一直被持有)。止血不等于不会失败 ——
// 磁盘满、I/O 错误、超过 30 秒的堆积,都会让 Commit 真的返回错误。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitErrorsAreNeverDiscarded(t *testing.T) {
	bare := scanBareCommits(t, "..")
	for _, b := range bare {
		t.Errorf("裸 tx.Commit() —— %s\n"+
			"提交失败时调用方拿到的是「成功」。正确写法(本仓 provision.go / cloudflare.go 就是):\n"+
			"    if cerr := tx.Commit().Error; cerr != nil { err = cerr }\n"+
			"函数要用具名返回值,defer 才改得动它。", b)
	}
	if len(bare) == 0 {
		t.Logf("全仓零裸 Commit")
	}
}

// 判据自证:合成一个裸 Commit,确认扫描器真的看得见。
// 零命中的守卫必须自证 —— 否则「全仓干净」与「扫描器没跑到」在终端上一样绿。
func TestBareCommitScannerSeesASyntheticOne(t *testing.T) {
	dir := t.TempDir()
	src := "package fake\n\nfunc F(tx interface{ Commit() error }) {\n\ttx.Commit()\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "fake.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("写合成反例: %v", err)
	}
	if got := scanBareCommits(t, dir); len(got) != 1 {
		t.Fatalf("扫描器对合成反例认出 %d 处(%v),期望 1 处 —— "+
			"判据本身失效了,上面那条「全仓干净」不成立", len(got), got)
	}
	// 反向:正确写法不得被误报(否则守卫会逼人改掉唯一对的写法)
	ok := "package fake\n\nfunc G(tx interface{ Commit() error }) (err error) {\n" +
		"\tif cerr := tx.Commit(); cerr != nil {\n\t\terr = cerr\n\t}\n\treturn\n}\n"
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "ok.go"), []byte(ok), 0o600); err != nil {
		t.Fatalf("写合成正例: %v", err)
	}
	if got := scanBareCommits(t, dir2); len(got) != 0 {
		t.Fatalf("扫描器把正确写法误报成裸 Commit(%v)—— 守卫会逼人改掉唯一对的写法", got)
	}
}

// scanBareCommits 找出所有【作为独立语句】调用的 .Commit() —— 也就是返回值被丢弃的那些。
// 判据钉在 ExprStmt 上:一旦 Commit 出现在赋值、if 初始化、return 里,
// 它的返回值就有去处,不算丢弃。
func scanBareCommits(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			b := filepath.Base(p)
			if b == "frontend" || b == ".git" || b == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, e := parser.ParseFile(fset, p, nil, 0)
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		ast.Inspect(f, func(n ast.Node) bool {
			es, ok := n.(*ast.ExprStmt)
			if !ok {
				return true
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Commit" {
				return true
			}
			out = append(out, rel+":"+itoaLine(fset.Position(call.Pos()).Line))
			return true
		})
		return nil
	})
	return out
}

func itoaLine(n int) string {
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

// 第二条:开了手动事务的函数里,不得【直接 return 一个 error 表达式】。
//
// 与上面那条是不同的缺陷类,上面的判据看不见它:
//
//	func F() (err error) {
//	    tx := db.Begin()
//	    defer func(){ if err == nil { tx.Commit() } else { tx.Rollback() } }()
//	    ...
//	    return tx.Create(&x).Error   // ← err 仍是 nil!defer 去 Commit 了
//	}
//
// 后果是【半截提交】:前面所有语句照常落库,只有最后这条没写进去,
// 而调用方拿到的是错误。两边说法不一致。stats.go 的 SaveStats 原本就是这样,
// 每 10 秒跑一次,提交的是流量累加、丢的是明细行 —— 计费按前者算。
//
// 正确写法:先 `err = <expr>` 再 `return err`,让 defer 看得见。
func TestManualTxFuncsDoNotReturnRawErrorExpressions(t *testing.T) {
	fset := token.NewFileSet()
	var bad []string
	filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			b := filepath.Base(p)
			if b == "frontend" || b == ".git" || b == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, e := parser.ParseFile(fset, p, nil, 0)
		if e != nil {
			return nil
		}
		rel, _ := filepath.Rel("..", p)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !hasManualBegin(fd.Body) {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				// 内层闭包(defer func(){...})有自己的返回语义,不在此判据内
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				rs, ok := n.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				for _, r := range rs.Results {
					// 🩸 只认【调用链末端是 .Error】这一种形态。
					// 第一版认「任何调用链」,把 `return nil, common.NewErrorf(...)`
					// 也算进去了 —— 那是【已经在 err != nil 分支里】重新包装错误文案,
					// defer 照样 Rollback,完全正确。判据宽一格就会逼人改掉对的写法。
					// 危险的是 `return tx.Create(&x).Error`:err 变量仍是 nil,
					// defer 去 Commit,而调用方拿到错误。
					if isGormErrorChain(r) {
						bad = append(bad, rel+":"+itoaLine(fset.Position(r.Pos()).Line)+
							" (func "+fd.Name.Name+")")
					}
				}
				return true
			})
		}
		return nil
	})
	for _, b := range bad {
		t.Errorf("手动事务函数里直接 return 了一个表达式 —— %s\n"+
			"defer 里的 Commit/Rollback 判的是 err 变量,而这条 return 绕过了它:\n"+
			"表达式非 nil 时,事务【仍会被 Commit】,前面的语句半截落库。\n"+
			"改成:err = <表达式>; return err", b)
	}
}

func hasManualBegin(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Begin" {
			found = true
		}
		return true
	})
	return found
}

// isGormErrorChain 判断返回值是不是 `<调用链>.Error` 这种形态。
// return err / return nil / return common.NewErrorf(...) 都不算。
func isGormErrorChain(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Error" {
		return false
	}
	// .Error 前面必须是一个调用(tx.Create(...) / db.Exec(...) 之类)
	inner := sel.X
	for {
		switch v := inner.(type) {
		case *ast.CallExpr:
			return true
		case *ast.SelectorExpr:
			inner = v.X
		default:
			return false
		}
	}
}
