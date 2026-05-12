package cronjob

import (
	"context"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

// SubRefreshJob 每分钟跑一次,遍历 subs 表,凡 last_synced_at + refresh_interval 已过期
// 的订阅,调用 SubService.RefreshSub。多订阅之间靠 SubService 的 subOpsMu 串行化,
// 不会同时几路并发触发 sing-box reload。
//
// 之所以 cron 周期固定 1min(而不是按每订阅自己的 interval 注册 N 个 cron),
// 是因为订阅可以动态增删,N 个 cron 句柄管理麻烦;统一 1min tick 内部判到期即可。
type SubRefreshJob struct{}

func NewSubRefreshJob() *SubRefreshJob { return &SubRefreshJob{} }

func (j *SubRefreshJob) Run() {
	var subs []model.Sub
	db := database.GetDB()
	if db == nil {
		return
	}
	if err := db.Where("enable = ?", true).Find(&subs).Error; err != nil {
		logger.Warning("SubRefreshJob: list subs failed: ", err)
		return
	}
	now := time.Now()
	ss := &service.SubService{}
	for _, sub := range subs {
		// RefreshInterval=0 表示禁用自动刷新,只手动触发
		if sub.RefreshInterval <= 0 {
			continue
		}
		dueAt := sub.LastSyncedAt.Add(time.Duration(sub.RefreshInterval) * time.Minute)
		if now.Before(dueAt) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		res, err := ss.RefreshSub(ctx, sub.Id)
		cancel()
		if err != nil {
			logger.Warning("SubRefreshJob: sub#", sub.Id, " refresh failed: ", err)
			continue
		}
		logger.Info("SubRefreshJob: sub#", sub.Id, " refreshed total=", res.Total,
			" parsed=", res.Parsed, " alive=", res.Alive)
	}
}

// SubWinnerCheckJob 每 5min 跑一次,巡检所有 pool-{cc} 出站当前 winner 是否还活,
// 死了立刻 re-elect 同国家次优节点;详见 service.SubService.CheckWinners。
type SubWinnerCheckJob struct{}

func NewSubWinnerCheckJob() *SubWinnerCheckJob { return &SubWinnerCheckJob{} }

func (j *SubWinnerCheckJob) Run() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ss := &service.SubService{}
	if err := ss.CheckWinners(ctx); err != nil {
		logger.Warning("SubWinnerCheckJob: ", err)
	}
}
