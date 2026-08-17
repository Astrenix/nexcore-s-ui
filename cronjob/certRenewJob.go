package cronjob

// 面板证书自愈作业 —— 见 service/panel_ssl_renew.go 头部的事故背景。
//
// 只做三件事:问 service 层"该续吗" → 续成了就重启面板加载新证 → 失败写日志。
// 判定与签发逻辑全部在 service 层(可单测),这里只负责调度与重启副作用。

import (
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

type CertRenewJob struct {
	panelService service.PanelService
}

func NewCertRenewJob() *CertRenewJob {
	return new(CertRenewJob)
}

func (j *CertRenewJob) Run() {
	// 续签要跑 ACME(DNS-01 传播等待可达数分钟),panic 不能带走整个 cron
	defer func() {
		if r := recover(); r != nil {
			logger.Error("CertRenewJob panic: ", r)
		}
	}()

	renewed, err := service.RenewPanelCertIfExpiring()
	if err != nil {
		// 该续没续成 —— 证书还是旧的。这条必须显眼:它是"三个月后又全站 offline"的唯一预警
		logger.Error("CertRenewJob: 面板证书续签失败(证书仍为旧证,下轮重试): ", err)
		return
	}
	if !renewed {
		return
	}
	// 新证书已写入 settings,重启面板才会加载。延迟 5s 让本轮日志落盘。
	logger.Info("CertRenewJob: 证书已更新,5 秒后重启面板加载")
	if err := j.panelService.RestartPanel(5 * time.Second); err != nil {
		logger.Error("CertRenewJob: 重启面板失败,新证书要等下次重启才生效: ", err)
	}
}
