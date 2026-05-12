package cronjob

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithSeconds())
	c.cron.Start()

	go func() {
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
