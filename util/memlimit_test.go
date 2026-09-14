package util

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("写 %s 失败: %v", name, err)
	}
	return p
}

// 探测顺序不能反:容器里的进程读 /proc/meminfo 拿到的是【宿主机】总内存,
// 按它算会得出一个远超实际配额的上限 —— 等于没设,而且看起来一切正常
// (日志里会打印一个很体面的数字)。cgroup 必须优先。
func TestDetectPrefersCgroupOverProcMeminfo(t *testing.T) {
	dir := t.TempDir()
	// 宿主机 64 GiB,cgroup 配额 512 MiB —— 两者差 128 倍,选错一眼看得出
	hostBytes := int64(64) << 30
	quotaBytes := int64(512) << 20

	origV2, origV1, origMem := cgroupV2MemMaxPath, cgroupV1MemMaxPath, procMemInfoPath
	t.Cleanup(func() {
		cgroupV2MemMaxPath, cgroupV1MemMaxPath, procMemInfoPath = origV2, origV1, origMem
	})
	procMemInfoPath = writeFile(t, dir, "meminfo",
		"MemTotal:       67108864 kB\nMemFree:  123 kB\n")
	cgroupV1MemMaxPath = filepath.Join(dir, "nonexistent-v1")

	// 1) cgroup v2 在场 → 必须用它
	cgroupV2MemMaxPath = writeFile(t, dir, "memory.max", "536870912\n")
	got, src := detectAvailableMemory()
	if got != quotaBytes {
		t.Fatalf("应取 cgroup v2 配额 %d,实得 %d(来源 %s)", quotaBytes, got, src)
	}

	// 2) cgroup v2 写 "max"(无限制)→ 跳过它,落到 /proc/meminfo
	cgroupV2MemMaxPath = writeFile(t, dir, "memory.max.unlimited", "max\n")
	got, src = detectAvailableMemory()
	if got != hostBytes {
		t.Fatalf("cgroup 无限制时应回退 /proc/meminfo(%d),实得 %d(来源 %s)", hostBytes, got, src)
	}
	if src != "/proc/meminfo" {
		t.Fatalf("来源应是 /proc/meminfo,实得 %s", src)
	}

	// 3) cgroup v1 用巨大数字表示无限制,同样要跳过
	cgroupV2MemMaxPath = filepath.Join(dir, "nonexistent-v2")
	cgroupV1MemMaxPath = writeFile(t, dir, "limit_in_bytes", "9223372036854771712\n")
	got, _ = detectAvailableMemory()
	if got != hostBytes {
		t.Fatalf("cgroup v1 哨兵值应被当作无限制,期望回退到 %d,实得 %d", hostBytes, got)
	}
}

// ratio / floor / ceiling 三条边界。floor 那条尤其重要:把 GOMEMLIMIT 设得
// 过低会让 Go 陷入持续 GC,那比不设限更糟 —— 所以小机器上宁可不设。
func TestComputeMemLimitBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		total int64
		want  int64
	}{
		// 期望值是独立算出来的字面量,不是用被测公式反推 ——
		// 反推的话把 ratio 改成 0.9 这条用例照样绿。
		// 1 GiB = 1073741824 × 0.6 = 644245094.4 → 截断 644245094
		{"1G 机器取 60%", 1 << 30, 644245094},
		// 2 GiB = 2147483648 × 0.6 = 1288490188.8 → 截断 1288490188
		{"2G 机器取 60%", 2 << 30, 1288490188},
		{"过小的机器不设(低于 floor)", 200 << 20, 0},
		{"探测失败不设", 0, 0},
		{"负数不设", -1, 0},
		{"大机器封顶到 ceiling", 64 << 30, memLimitCeiling},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeMemLimit(tc.total); got != tc.want {
				t.Fatalf("total=%d: 期望 %d,实得 %d", tc.total, tc.want, got)
			}
		})
	}

	// floor 的两侧:刚好不够 与 刚刚够,必须给出不同结论。
	// 只测一侧的话,把 floor 判断整个删掉也不会红。
	// floor(128 MiB)÷ ratio(0.6)≈ 223696213 字节,是"刚好够得着 floor"的 total。
	const floorTotal = 223696213
	justBelow := int64(floorTotal - 16<<20)
	justAbove := int64(floorTotal + 16<<20)
	if got := computeMemLimit(justBelow); got != 0 {
		t.Fatalf("floor 下方应返回 0,total=%d 实得 %d", justBelow, got)
	}
	if got := computeMemLimit(justAbove); got == 0 {
		t.Fatalf("floor 上方应返回非 0,total=%d 却返回 0", justAbove)
	}
}

// 运维显式设了 GOMEMLIMIT 就是明确意图,程序不能覆盖。
func TestRespectsExplicitEnv(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "256MiB")
	limit, src := ConfigureMemoryLimit()
	if limit != 0 {
		t.Fatalf("环境变量已设时不该再设,实得 %d", limit)
	}
	if src == "" {
		t.Fatal("应返回说明文字,便于日志排查")
	}
}

// 用生产节点上取来的真实样本,而不是编造的格式。
// 编造的夹具容易恰好避开真实数据的形状(空格数、单位大小写、字段个数),
// 于是解析写错了测试也全绿。
//
// 样本来自 tw1(2026-09-14 实测):
//
//	MemTotal:        2014860 kB
//
// 同机 cgroup v2 / v1 的 memory 限制文件【都不存在】(裸机 VM),
// 所以生产上真正走的是 /proc/meminfo 这条回退路径 —— 也就是说
// 这个用例覆盖的才是主路径,cgroup 那两条反而是容器场景的备用。
func TestParsesRealProdMeminfo(t *testing.T) {
	dir := t.TempDir()
	origV2, origV1, origMem := cgroupV2MemMaxPath, cgroupV1MemMaxPath, procMemInfoPath
	t.Cleanup(func() {
		cgroupV2MemMaxPath, cgroupV1MemMaxPath, procMemInfoPath = origV2, origV1, origMem
	})
	cgroupV2MemMaxPath = filepath.Join(dir, "no-v2")
	cgroupV1MemMaxPath = filepath.Join(dir, "no-v1")
	procMemInfoPath = writeFile(t, dir, "meminfo",
		"MemTotal:        2014860 kB\nMemFree:          131072 kB\nMemAvailable:    1484800 kB\n")

	got, src := detectAvailableMemory()
	const want = int64(2014860) * 1024 // 2063216640
	if got != want {
		t.Fatalf("真实 meminfo 解析错误:期望 %d,实得 %d", want, got)
	}
	if src != "/proc/meminfo" {
		t.Fatalf("来源应是 /proc/meminfo,实得 %s", src)
	}
	// 这台机器上该设多少:2014860 kB × 0.6 ≈ 1237929984 字节
	if limit := computeMemLimit(got); limit != 1237929984 {
		t.Fatalf("2G 机器的软上限应为 1237929984(约 1180 MiB),实得 %d", limit)
	}
}
