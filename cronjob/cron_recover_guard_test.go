package cronjob

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

// 编译期断言:cronLogger 必须满足 cron.Logger,否则 cronChainOptions 根本构造不出来。
var _ cron.Logger = cronLogger{}

// e510PanicJob 每次 Run 都 panic。用它来验证生产链真的兜得住。
type e510PanicJob struct{ ran *atomic.Int32 }

func (j e510PanicJob) Run() {
	j.ran.Add(1)
	panic("E510 synthetic panic —— 这条 panic 必须被 cron chain 的 Recover 兜住")
}

// TestE510_CronChainRecoversJobPanic 是【行为】断言,不是形态断言。
//
// 为什么不检查源码里有没有 "cron.Recover":那种写法挡不住能编译的退化
// (比如 Recover 被挪到链的内层、或换成一个不 recover 的自制 wrapper)。
// 这里直接拿【生产用的那条链】cronChainOptions() 跑一个必然 panic 的 job:
//
//   - 链里有 Recover  → panic 被吞,调度继续,job 能跑到第 2 次 → 绿
//   - 链里没有 Recover → goroutine panic 未捕获 → 整个测试【进程】崩溃 → 红
//
// 后一种情况的红是进程级 crash 而不是优雅的 FAIL,这正是生产上会发生的事:
// 面板进程退出 → 主控调节点 API 全部失败 → 该节点上所有用户的线路开不出来。
func TestE510_CronChainRecoversJobPanic(t *testing.T) {
	c := cron.New(cron.WithSeconds(), cronChainOptions())
	var ran atomic.Int32
	if _, err := c.AddJob("@every 1s", e510PanicJob{&ran}); err != nil {
		t.Fatalf("AddJob 失败:%v", err)
	}
	c.Start()
	defer c.Stop()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && ran.Load() < 2 {
		time.Sleep(100 * time.Millisecond)
	}
	// 能执行到这里,本身就说明进程没被第一次 panic 带走。
	if n := ran.Load(); n < 2 {
		t.Fatalf("panic job 只跑了 %d 次(期望 >=2)—— 第一次 panic 之后调度就没再触发,"+
			"说明链没有恢复能力,或 SkipIfStillRunning 把它永久判成「仍在运行」", n)
	}
}

// TestE510_EveryCronJobIsRegisteredThroughChain 钉住扫描面:
// 所有 job 都必须经由 c.cron.AddJob 注册,才会走 cronChainOptions 那条链。
// 若将来有人用 time.AfterFunc / 裸 go func 起周期任务,它拿不到这层保护 ——
// 本断言不试图机械枚举那些形态(那是「只认一种写法」的老坑),
// 只钉住 CronJob 结构体确实只有 cron 这一个调度器字段,新增调度入口会让它编译不过。
func TestE510_CronJobHasSingleScheduler(t *testing.T) {
	j := NewCronJob()
	if j == nil {
		t.Fatal("NewCronJob 返回 nil")
	}
	// 反射字段数:CronJob 目前只有 cron 一个字段。加了第二个调度器就必须重新审视
	// 「所有周期任务都在 Recover 链下」这个前提。
	if got := reflectNumField(j); got != 1 {
		t.Fatalf("CronJob 有 %d 个字段(基线 1)—— 新增的那个如果是另一个调度器,"+
			"它不走 cronChainOptions,panic 会重新变成进程级崩溃。请确认后更新本基线。", got)
	}
}

func reflectNumField(v interface{}) int {
	return reflect.Indirect(reflect.ValueOf(v)).NumField()
}
