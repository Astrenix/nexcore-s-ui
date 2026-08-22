package cronjob

// 调度判定的状态必须来自【库】,不能来自进程内存。
//
// 2026-08-22 E566/E567 的成因:面板证书续签的 6 小时节流记在包级变量里,
// 而作业在启动 2 分钟后就跑一次 —— 重启一次 = 节流清零 = 一次全新 ACME 尝试。
// 两处单看都对(单进程内节流没问题、尽早自愈也没问题),只有乘起来才是缺陷。
//
// 本包是全部定时作业的家。这里的零容忍不是洁癖:
// cronjob 的每个作业都可能带【外部代价】—— 拉上游订阅 URL、跑 ACME、
// 给运营推消息。这类代价一旦因为重启而重复付出,后果不是多花点 CPU,
// 而是烧掉上游配额(LE 同域名每周 5 张重复证书,打满后 retry-after 可达数天)。
//
// 做对了的样板就在本包:SubRefreshJob 每分钟扫 subs 表,到期判定是
// `sub.LastSyncedAt.Add(sub.RefreshInterval)` —— 两个值都是 DB 列,重启不清零。
// 它恰恰是外部代价最高的那个(每个订阅都要拉一次上游 URL)。

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// schedStateAllowed 允许的包级 var —— 目前一个都没有,刻意留空。
// 将来真要加,必须在这里写清楚"重启归零之后,紧接着那一次动作的外部代价是什么"。
var schedStateAllowed = map[string]string{}

func TestCronjobPackageHasNoProcessLocalState(t *testing.T) {
	found := scanPkgVars(t, ".")

	for _, v := range found {
		if why, ok := schedStateAllowed[v]; ok {
			t.Logf("已登记豁免: %s —— %s", v, why)
			continue
		}
		t.Errorf("cronjob 包出现包级 var %q。\n"+
			"定时作业的调度状态(上次跑过没有 / 距上次多久 / 已通知过)必须落库,"+
			"进程内变量【一重启就归零】,而作业在启动后就会跑 —— 归零那一刻正好又跑一次。\n"+
			"样板见 subJob.go:SubRefreshJob(到期判定取 sub.LastSyncedAt + sub.RefreshInterval,两个都是 DB 列)。\n"+
			"若确认归零无外部代价,把它连同理由加进 schedStateAllowed。", v)
	}
}

// 判据自证:合成一个反例,确认扫描器真的能看见包级 var。
// 零命中维度的守卫必须自证 —— 否则「本包很干净」与「扫描器根本没跑到」
// 在终端上是同一个画面。
func TestCronjobStateScannerSeesAPackageVar(t *testing.T) {
	dir := t.TempDir()
	src := "package fake\n\nimport \"time\"\n\nvar lastNotifiedAt time.Time\nvar counter int\n"
	if err := os.WriteFile(filepath.Join(dir, "fake.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("写合成反例: %v", err)
	}
	got := scanPkgVars(t, dir)
	if len(got) != 2 {
		t.Fatalf("扫描器对合成反例只认出 %d 个包级 var(%v),期望 2 个 —— "+
			"判据本身失效了,上面那条「本包干净」不成立", len(got), got)
	}
}

// scanPkgVars 列出目录下所有非测试 .go 的包级 var 名。
func scanPkgVars(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录 %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, 0)
		if err != nil {
			t.Fatalf("解析 %s: %v", n, err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, sp := range gd.Specs {
				for _, name := range sp.(*ast.ValueSpec).Names {
					if name.Name != "_" {
						out = append(out, n+":"+name.Name)
					}
				}
			}
		}
	}
	return out
}
