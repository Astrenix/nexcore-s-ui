package cronjob

import (
	"github.com/alireza0/s-ui/util/common"
	"time"

	"github.com/alireza0/s-ui/logger"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
}

// cronChainOptions 返回 cron 的 wrapper 链。
//
// 🩸 E510:Recover 必须在链的【最内层】—— 顺序反了会静默废掉这个 job。
//
// NewChain(m1,m2).Then(job) 展开成 m1(m2(job))。写成 (Recover, Skip) 看起来"更安全"
// (先兜住一切),实际是错的,因为 robfig/cron v3.0.1 的 SkipIfStillRunning 是:
//
//	case v := <-ch:
//	    j.Run()      // panic 从这里抛出
//	    ch <- v      // ← 不是 defer,panic 时【永远不执行】
//
// panic 穿过它 → 令牌永久丢失 → 该 job 之后每一次触发都落进 default 分支被 skip。
// 进程是活下来了,但那个作业从此再也不跑,而且日志里只有平静的 "skip"。
// 反过来 Skip(Recover(job)) 让 Recover 先吞掉 panic,j.Run() 正常返回,令牌归还。
//
// 这条是【行为】守卫当场逼出来的:我第一版就写成了 (Recover, Skip),
// 而"链里有没有 cron.Recover"这种形态断言对顺序完全是瞎的。
//
// 此前链里只有 SkipIfStillRunning,而 robfig/cron v3 默认【不】兜 panic:
// 任何一个 job panic 都会带走整个面板进程 → 主控调节点 API 全部失败 → 节点 offline,
// 与 2026-08-17 那次「证书静默过期让节点从主控视角消失」是同一类后果。
// 8 个 job 里只有 CertRenewJob 自己 defer 了 recover(它的注释就写着「panic 不能带走
// 整个 cron」)—— 规矩被写下来了,却只落实在写它的那一个 job 上。
// 挂在 chain 上而不是逐个 job 加,是因为后者对【将来新增的 job】无效。
//
// 抽成函数是为了让守卫能拿到【生产实际用的那条链】去跑真实的 panic job ——
// 在测试里另写一条链等于什么都没验。
func cronChainOptions() cron.Option {
	return cron.WithChain(cron.SkipIfStillRunning(cronLogger{}), cron.Recover(cronLogger{}))
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

// cronLogger 把 robfig/cron 的日志接到项目 logger。只在作业被跳过 / 出错时有输出,
// 正常调度不刷屏。
type cronLogger struct{}

func (cronLogger) Info(msg string, keysAndValues ...interface{}) {
	// SkipIfStillRunning 跳过重入作业时走这里 —— 用 Warning 让"作业堆积"可见
	logger.Warning(append([]interface{}{"cron: ", msg, " "}, keysAndValues...)...)
}

func (cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	logger.Error(append([]interface{}{"cron: ", msg, " ", err.Error(), " "}, keysAndValues...)...)
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	// SkipIfStillRunning:上一轮同名作业还在跑就跳过本轮,不叠 goroutine。
	// 关键防护——大订阅刷新(可能几分钟)期间,每分钟一次的 SubRefreshJob 不会
	// 堆积成一片阻塞在同一把 subOpsMu 上的 goroutine。快作业(StatsJob 等)几乎
	// 瞬时完成,不受影响。
	c.cron = cron.New(
		cron.WithLocation(loc),
		cron.WithSeconds(),
		cronChainOptions(),
	)
	c.cron.Start()

	go func() {
		// 🩸 E564:这个 goroutine 负责【注册全部定时作业】。它 panic 不只是带走进程,
		// 而是在此之前就已经让后面的 AddJob 一个都没执行 —— 统计、配额、证书续签全不跑。
		// cron chain 的 Recover(E510)只保护【已注册作业的执行】,保护不了注册过程本身。
		defer common.Recover("cronjob: 注册定时作业")
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob(trafficAge > 0))
		// 客户端 expiry/quota — Basic Auth 协议(mixed/socks/http/naive)
		// 也走 clients 表,所以用同一个 DepleteJob 一并处理,无需独立 cron
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Start deleting old stats
		if trafficAge > 0 {
			c.cron.AddJob("@daily", NewDelStatsJob(trafficAge))
		}
		// Start core if it is not running
		c.cron.AddJob("@every 5s", NewCheckCoreJob())
		// database WAL checkpoint
		c.cron.AddJob("@every 10m", NewWALCheckpointJob())
		// 订阅自动刷新 — 每 1min 扫一次 subs 表看哪个到期(refresh_interval 单位分钟,
		// 默认 60min);到期就 fetch → parse → probe → upsert sub_nodes → re-elect winners
		c.cron.AddJob("@every 1m", NewSubRefreshJob())
		// 订阅 winner 巡检 — 每 5min 检查所有 pool-{cc} 出站当前 winner 是否还活;
		// 死了立刻从 sub_nodes 同国家次优 alive 节点 re-elect
		c.cron.AddJob("@every 5m", NewSubWinnerCheckJob())
		// 面板证书自愈 — 每 6h 查一次 webCertFile,剩余 < 30 天自动 ACME 续签 + 重启加载。
		// 2026-08-17:此前续签只有设置页的手动按钮,证书 8/8 静默过期 → 主控调节点 API
		// 全部 x509 失败 → 节点 offline、用户买了流量包一条线路都开不出来。
		// 首检延后 2min:面板刚起来时 DNS / 网络可能还没就绪,而这作业会跑 ACME。
		go func() {
			time.Sleep(2 * time.Minute)
			NewCertRenewJob().Run()
		}()
		c.cron.AddJob("@every 6h", NewCertRenewJob())
	}()

	return nil
}

// Stop 等待飞行中作业完成 — `c.cron.Stop()` 返回 ctx,Done 表示所有作业 goroutine 已收尾。
//
// AUDIT.md H4:之前 fire-and-forget 调 Stop(),不等飞行中事务。如果 panel 关停瞬间
// StatsJob 正在持有写事务,后续 DB.Close 拿到关闭信号但事务还没 commit,
// SQLite 可能留 -wal/-shm 残留,启动还得做一次 recovery。等 Done 是干净退出。
//
// 给 5s 上限避免某个挂住的 job 让 panel Stop 永远不返回(reload 卡死场景)。
func (c *CronJob) Stop() {
	if c.cron == nil {
		return
	}
	stopCtx := c.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(5 * time.Second):
		// 超时只 log,不阻塞 panel 主流程退出
	}
}
