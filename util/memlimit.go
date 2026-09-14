package util

import (
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// 节点常跑在 1 核 / 1G 的小机器上,而 Go 默认【没有】内存上限:堆涨到哪算哪,
// 由内核 OOM killer 决定谁死。对节点来说那等于整台机器随机掉线,而且
// systemd Restart=on-failure 拉起来之后还会再涨一次 —— 症状是周期性 offline,
// 日志里什么都看不到(被 SIGKILL 的进程写不出遗言)。
//
// GOMEMLIMIT 是软上限:接近它时 Go 会更积极地 GC 而不是继续申请内存。
// 设一个【远高于常态、低于危险线】的值,平时完全不影响性能(实测节点常态
// RSS 128MB),只在真要失控时把代价从"被杀"换成"多花点 CPU 做 GC"——
// 而这台机器最不缺的就是 CPU(实测 load 0.00)。
const (
	// memLimitRatio 取可用内存的 60%。剩下的 40% 不是浪费:
	//   - sqlite 走 CGO,它的页缓存在 C 侧,【不计入】Go heap 也不受 GOMEMLIMIT 管
	//   - sing-box 的每连接 buffer、goroutine 栈同样在 Go heap 之外还有份额
	//   - 系统本身、以及 OOM 前的缓冲
	memLimitRatio = 0.6
	// memLimitFloor 是下限。把 GOMEMLIMIT 设得过低会让 Go 陷入持续 GC
	// (官方称之为 death spiral),那比不设限更糟。小于这个值就不值得设。
	memLimitFloor = 128 << 20 // 128 MiB
	// memLimitCeiling 是上限。大机器上没必要把软限推得很高 —— 这个值的作用
	// 是兜底失控,不是给正常负载用的配额。
	memLimitCeiling = 4 << 30 // 4 GiB
)

// 探测路径提成变量,测试里替换。生产值就是下面这三个常规位置。
var (
	cgroupV2MemMaxPath = "/sys/fs/cgroup/memory.max"
	cgroupV1MemMaxPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	procMemInfoPath    = "/proc/meminfo"
)

// computeMemLimit 把"可用内存"换算成要设的软上限。返回 0 表示不该设。
// 单独拆出来是为了让 ratio / floor / ceiling 三条边界能被直接测到 ——
// 它们混在 ConfigureMemoryLimit 里就只能靠真实机器的内存值去撞。
func computeMemLimit(total int64) int64 {
	if total <= 0 {
		return 0
	}
	limit := int64(float64(total) * memLimitRatio)
	if limit < memLimitFloor {
		return 0
	}
	if limit > memLimitCeiling {
		return memLimitCeiling
	}
	return limit
}

// ConfigureMemoryLimit 按本机可用内存设置 Go 软内存上限。
//
// 返回实际设置的字节数与来源描述;返回 0 表示没有设置(调用方据此决定是否记日志)。
// 已经显式设了 GOMEMLIMIT 环境变量时【不覆盖】—— 那是运维的明确意图。
func ConfigureMemoryLimit() (int64, string) {
	if v := strings.TrimSpace(os.Getenv("GOMEMLIMIT")); v != "" {
		// 环境变量已生效(Go runtime 启动时就读了),这里只是不去动它
		return 0, "GOMEMLIMIT 环境变量已设置(" + v + "),不覆盖"
	}

	total, src := detectAvailableMemory()
	if total <= 0 {
		return 0, "无法探测本机内存,跳过"
	}

	limit := computeMemLimit(total)
	if limit == 0 {
		// 机器太小,强设会 GC 抖动;不如交给内核
		return 0, "可用内存过小(" + strconv.FormatInt(total>>20, 10) + "MiB),跳过以免 GC 抖动"
	}

	debug.SetMemoryLimit(limit)
	return limit, src
}

// detectAvailableMemory 依次尝试 cgroup v2 → cgroup v1 → /proc/meminfo。
//
// 顺序不能反:容器/cgroup 里的进程看到的 /proc/meminfo 是【宿主机】的总内存,
// 按它算会得出一个远超实际配额的上限,等于没设。
func detectAvailableMemory() (int64, string) {
	// cgroup v2
	if v, ok := readCgroupBytes(cgroupV2MemMaxPath); ok {
		return v, "cgroup v2"
	}
	// cgroup v1
	if v, ok := readCgroupBytes(cgroupV1MemMaxPath); ok {
		return v, "cgroup v1"
	}
	if v, ok := readMemTotal(procMemInfoPath); ok {
		return v, "/proc/meminfo"
	}
	return 0, ""
}

// cgroupUnlimitedThreshold:cgroup v1 用一个巨大的数字表示"无限制"
// (通常是 PAGE_COUNTER_MAX)。超过这个量级就当作没有配额,继续往下探。
const cgroupUnlimitedThreshold = 1 << 50 // 1 PiB

func readCgroupBytes(path string) (int64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "" || s == "max" { // cgroup v2 的"无限制"
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 || v >= cgroupUnlimitedThreshold {
		return 0, false
	}
	return v, true
}

func readMemTotal(path string) (int64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		f := strings.Fields(line)
		// 形如:MemTotal:  1966904 kB
		if len(f) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseInt(f[1], 10, 64)
		if err != nil || kb <= 0 {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}
