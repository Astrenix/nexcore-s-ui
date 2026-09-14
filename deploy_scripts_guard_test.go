package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// 部署侧有两组"同一件事的两份实现",都是分发方式逼出来的:
//
//  1. journald 容量上限 —— install.sh(新装)与 update.sh(升级)各一份。
//     两个脚本独立下载执行,没法 source 共享文件。
//  2. systemd unit 的资源约束 —— 仓库的 nexcore-s-ui.service 与
//     install.sh 里的 fallback 模板(tarball 没带 unit 时现造)各一份。
//
// 副本必然分叉,分叉的方向是【新装机器和升级机器行为不一致】,而且两边
// 各自都能正常跑起来,没有任何报错 —— 只有在对比两台机器时才看得出来。
// 这两条守卫就是钉住它们。

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", path, err)
	}
	return string(b)
}

var journalBlockRe = regexp.MustCompile(`(?s)\[Journal\]\n(.*?)\nJEOF`)

// extractJournalSettings 取出 [Journal] 段里的配置行(忽略空行)。
// 取不到时 Fatal —— 守卫读不到检查对象等于在自己失效那一刻静默通过。
func extractJournalSettings(t *testing.T, script, path string) []string {
	t.Helper()
	m := journalBlockRe.FindStringSubmatch(script)
	if m == nil {
		t.Fatalf("%s 里找不到 [Journal] 配置块 —— 要么这段被删了(那 journal 又变回无上限),"+
			"要么 heredoc 结束符不再是 JEOF,本守卫需要跟着改", path)
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		t.Fatalf("%s 的 [Journal] 段是空的", path)
	}
	return out
}

func TestJournaldSettingsMatchAcrossScripts(t *testing.T) {
	install := extractJournalSettings(t, mustRead(t, "install.sh"), "install.sh")
	update := extractJournalSettings(t, mustRead(t, "update.sh"), "update.sh")

	if strings.Join(install, "|") != strings.Join(update, "|") {
		t.Fatalf("install.sh 与 update.sh 的 journald 配置已分叉:\n  install.sh: %v\n  update.sh:  %v\n"+
			"新装机器和升级机器会有不同的 journal 上限", install, update)
	}

	// 规模闸:至少得真的设了容量上限,而不是只剩个空壳 [Journal] 段
	joined := strings.Join(install, "|")
	if !strings.Contains(joined, "SystemMaxUse=") {
		t.Fatalf("journald 配置里没有 SystemMaxUse —— 上限没设,journal 仍然无限增长:%v", install)
	}

	// install.sh 把这段包在函数里,所以还要确认函数【被调用】。
	// 同一个坑本仓已经踩过三次(PruneOlderThan / harden_journald / prune_old_backups):
	// 定义写得好好的,一个调用都没有,语法检查全过而功能根本不存在。
	assertShellFuncCalled(t, mustRead(t, "install.sh"), "harden_journald", "install.sh")
	assertShellFuncCalled(t, mustRead(t, "install.sh"), "report_disk_hogs", "install.sh")
}

// assertShellFuncCalled 确认脚本里除了函数定义之外,还有至少一处调用。
func assertShellFuncCalled(t *testing.T, script, name, where string) {
	t.Helper()
	defined, called := false, false
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == name+"() {":
			defined = true
		case trimmed == name || strings.HasPrefix(trimmed, name+" "):
			called = true
		}
	}
	if !defined {
		t.Fatalf("%s 里找不到函数 %s 的定义", where, name)
	}
	if !called {
		t.Fatalf("%s 里 %s 有定义但【没有任何调用点】—— 这段防护实际上从不执行", where, name)
	}
}

// unitResourceKeys 是 unit 里与资源约束相关的行。两份 unit 必须都有,
// 且取值一致 —— 否则 tarball 带不带 .service 会决定这台机器有没有内存兜底。
var unitResourceKeys = []string{"MemoryAccounting=", "MemoryMax="}

func TestUnitResourceLimitsMatchInstallFallback(t *testing.T) {
	unit := mustRead(t, "nexcore-s-ui.service")
	install := mustRead(t, "install.sh")

	for _, key := range unitResourceKeys {
		unitVal := findSettingLine(t, unit, key, "nexcore-s-ui.service")
		instVal := findSettingLine(t, install, key, "install.sh 的 fallback unit")
		if unitVal != instVal {
			t.Fatalf("%s 在两份 unit 里不一致:\n  nexcore-s-ui.service: %q\n  install.sh:           %q",
				key, unitVal, instVal)
		}
	}
}

func findSettingLine(t *testing.T, content, prefix, where string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return trimmed
		}
	}
	t.Fatalf("%s 里找不到 %s —— 这台机器将没有内存兜底,1G 小机器上会被 OOM killer 直接杀掉",
		where, prefix)
	return ""
}

// prune_old_backups 是 update.sh 里唯一防止安装目录被历史二进制撑爆的东西
// (生产实测:3 个 sui.bak 占 246M,比二进制本身还多,且没有任何清理机制)。
//
// 它的失效方式【完全静默】——一个文件都不删,不报错,不打日志,
// 只有几个月后磁盘满了才发现。所以这里真的把函数抠出来用 bash 跑一遍。
//
// 夹具刻意让【文件名序】与【mtime 序】相反:按名字排和按时间排会保留
// 不同的文件。这一点必须由夹具保证 —— 第一版夹具两种排序恰好同解,
// 把实现换成 sort -r 照样全绿。
func TestUpdateScriptPrunesOldBackups(t *testing.T) {
	script := mustRead(t, "update.sh")

	fn := extractShellFunc(t, script, "prune_old_backups")
	if !strings.Contains(fn, "ls -t") {
		t.Fatalf("prune_old_backups 不再按 mtime(ls -t)排序 —— "+
			"历史备份有两种命名格式(20260510-172744-pre1717 与 1778434600-pre1718),"+
			"按文件名排会保留错的那个。当前实现:\n%s", fn)
	}

	// 🩸 光有函数不算数,还得真的被调用。
	// 这个仓库里"写好了却零调用方"已经出现过三次:ApiLogService.PruneOlderThan
	// (注释写着"后台 cron 用"、全仓零调用)、install.sh 的 harden_journald、
	// 以及这里。三次的共同点是【编译/语法全过,功能整个不存在】。
	callers := 0
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "prune_old_backups ") {
			callers++
		}
	}
	if callers == 0 {
		t.Fatal("prune_old_backups 有定义但【没有任何调用点】—— " +
			"函数写得再对也不会执行,安装目录照样被历史二进制撑大")
	}

	dir := t.TempDir()
	// 名字倒序第一的给最老 mtime;mtime 最新的那个名字倒序排最后
	fixtures := []string{
		"sui.bak.20260914-120000",
		"sui.bak.20260901-101010",
		"sui.bak.20260510-172744-pre1717",
		"sui.bak.20260510-164839-pre1715",
		"sui.bak.1778434600-pre1718", // ← mtime 最新,名字倒序最后
	}
	for i, name := range fixtures {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(fmt.Sprintf("payload%d", i+1)), 0o644); err != nil {
			t.Fatalf("造夹具 %s 失败: %v", name, err)
		}
		mt := time.Date(2026, 9, i+1, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatalf("设 mtime 失败: %v", err)
		}
	}

	runner := filepath.Join(dir, "run.sh")
	body := "#!/bin/bash\nset -u\n" + fn + "\nprune_old_backups 1 \"$1\"/sui.bak.*\n"
	if err := os.WriteFile(runner, []byte(body), 0o755); err != nil {
		t.Fatalf("写 runner 失败: %v", err)
	}
	out, err := exec.Command("bash", runner, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("跑 prune 失败: %v\n%s", err, out)
	}

	left, err := filepath.Glob(filepath.Join(dir, "sui.bak.*"))
	if err != nil {
		t.Fatalf("列剩余文件失败: %v", err)
	}
	if len(left) != 1 {
		t.Fatalf("keep=1 时应只剩 1 个备份,实得 %d 个:%v\n"+
			"轮转没生效 = 安装目录会被历史二进制无限撑大", len(left), left)
	}
	content, err := os.ReadFile(left[0])
	if err != nil {
		t.Fatalf("读剩余文件失败: %v", err)
	}
	// payload5 = mtime 最新的那个。留下别的说明排序用错了维度。
	if string(content) != "payload5" {
		t.Fatalf("保留的不是 mtime 最新的那个:留下了 %s(内容 %s),应留 sui.bak.1778434600-pre1718",
			filepath.Base(left[0]), content)
	}
}

// extractShellFunc 从脚本里抠出一个顶层 shell 函数(定义到第一个顶格 } 为止)。
func extractShellFunc(t *testing.T, script, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\(\) \{$`)
	loc := re.FindStringIndex(script)
	if loc == nil {
		t.Fatalf("update.sh 里找不到函数 %s —— 要么被删了(那备份就不再轮转了),"+
			"要么定义格式变了,本守卫需要跟着改", name)
	}
	rest := script[loc[0]:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("函数 %s 没有顶格的结束括号,无法提取", name)
	}
	return rest[:end+3]
}

// update.sh 里这几个函数都属于"定义了但忘了调用就完全失效"的类型,
// 而且失效时不报错:预检不跑 = 盘满时在写二进制的途中挂掉(服务起不来),
// 轮转不跑 = 安装目录被历史备份撑大。统一钉住它们的调用点。
func TestUpdateScriptCallsItsGuards(t *testing.T) {
	script := mustRead(t, "update.sh")
	for _, fn := range []string{"require_space", "prune_old_backups"} {
		assertShellFuncCalled(t, script, fn, "update.sh")
	}
}
