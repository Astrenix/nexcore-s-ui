package cronjob

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type CheckCoreJob struct {
	service.ConfigService
}

func NewCheckCoreJob() *CheckCoreJob {
	return &CheckCoreJob{}
}

func (s *CheckCoreJob) Run() {
	// 🩸 E565:这是 core 挂掉之后的【自愈作业】—— 最后一道防线。
	// 此前是裸调用,返回值直接扔了:自愈失败时 cron 看起来一切正常,
	// 而节点已经没有任何入站,没人知道防线本身也倒了。
	//
	// ⚠️ 注意 StartCore 的两个「返回 nil 但没启动」的分支,拿到 nil 不等于 core 在跑:
	//   · corePtr.IsRunning() 已在跑 → nil(正常)
	//   · 上次启动失败还在 cooldown 内 → nil(刻意的,避免疯狂重试;它记 Info)
	// 所以这里只负责把【真错误】喊出来,不去替它判断 core 到底起没起来。
	if err := s.ConfigService.StartCore(); err != nil {
		logger.Error("CheckCoreJob 自愈失败,该节点可能已无入站:", err.Error())
	}
}
