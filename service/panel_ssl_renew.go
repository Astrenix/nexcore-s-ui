package service

// 面板 HTTPS 证书自动续签 —— 2026-08-17 生产事故的根治。
//
// 事故经过:面板证书是 ACME 签的 90 天证,但**续签只有手动入口**(设置页的一键续签,
// 走 api/panelSslRenew)。tw1 / ca1 的证书 8/8 到期后没人点,主控调节点 API 全部
// x509 失败 → 节点判定 offline → 用户买了流量包却一条线路都开不出来,直到有人翻
// 数据库才发现。修证书本身只是止血:同一张新证 11/15 又会到期,不加自动续签就是
// 三个月后原样复发。
//
// 与手动入口的关键差异:**本 job 绝不碰 DNS**。
// PanelSSLService.IssueAndApply 会顺带 UpsertARecord(..., proxied=false) ——
// 那是首次签发时"域名还没解析"的补救,但对已经挂在 Cloudflare 橙云后面的节点
// (sg1/jp1/kr1/us1 都是 proxied=true),自动把橙云关掉等于把源站 IP 暴露出去。
// 所以这里只做 ACME 签发 + 写 settings,DNS 拓扑保持运营配置的原样。

import (
	"sync"
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

const (
	// PanelCertRenewThresholdDays 剩余有效期低于该天数就续签。
	// Let's Encrypt 证书 90 天,30 天窗口给足了重试余量:即使每天失败也有 30 次机会。
	PanelCertRenewThresholdDays = 30

	// panelCertRenewMinInterval 两次**尝试**之间的最小间隔。
	// 防的是 LE 速率限制:同一域名每周 5 张重复证书,失败重试打满会把配额烧光,
	// 而且 8/6 那次事故里 crash-loop 反复撞 429、retry-after 被无限顺延,
	// 恢复窗口反而被自己毒化。宁可慢,不可密。
	panelCertRenewMinInterval = 6 * time.Hour
)

var (
	panelCertRenewMu      sync.Mutex
	panelCertRenewLastTry time.Time
)

// shouldRenewPanelCert 纯判定:这张证书该不该续。
//
// 抽成独立函数是为了能脱离 db / ACME 单测 —— 阈值判断错一次的代价就是这次事故。
// **已过期也要续**:DaysLeft 为负时同样返回 true,别写成 `DaysLeft <= 阈值 && DaysLeft > 0`
// 那种"过期太久反而不修"的边界。
func shouldRenewPanelCert(info PanelCertInfo) bool {
	if !info.Configured || info.Error != "" {
		return false
	}
	return info.Expired || info.DaysLeft <= PanelCertRenewThresholdDays
}

// RenewPanelCertIfExpiring 检查面板证书,快到期就续签。
//
// 返回 (renewed, err):
//   - (false, nil) 无需续签 / 距上次尝试太近 —— 正常路径,调用方不必告警
//   - (true, nil)  已签发新证并写入 settings;**调用方需要重启面板**才会加载新证书
//   - (false, err) 该续但没续成 —— 证书仍是旧的,调用方记录错误即可,下轮重试
//
// 前置条件不满足(没配域名 / 没存 CF 凭据)一律返回 error 而不是静默跳过:
// "自动续签开着但其实从来没生效"是最坏的一种失败,必须在日志里可见。
func RenewPanelCertIfExpiring() (bool, error) {
	panelCertRenewMu.Lock()
	defer panelCertRenewMu.Unlock()

	settingSvc := SettingService{}
	sslSvc := PanelSSLService{}

	certFile, err := settingSvc.GetCertFile()
	if err != nil {
		return false, common.NewError("读取 webCertFile 失败: ", err.Error())
	}
	if certFile == "" {
		// 面板跑在 HTTP 上(没配证书)——这是合法配置,不是故障
		return false, nil
	}

	info := sslSvc.GetPanelCertInfo(certFile)
	if info.Error != "" {
		return false, common.NewError("解析面板证书失败: ", info.Error)
	}
	if !shouldRenewPanelCert(info) {
		return false, nil
	}

	// 到这里已经确定"该续了"。节流只挡重复**尝试**,不挡判定 —— 判定结果要能进日志。
	if !panelCertRenewLastTry.IsZero() && time.Since(panelCertRenewLastTry) < panelCertRenewMinInterval {
		logger.Info("PanelSSLRenew: 证书剩余 ", info.DaysLeft, " 天需续签,但距上次尝试不足 ",
			panelCertRenewMinInterval, ",本轮跳过")
		return false, nil
	}
	panelCertRenewLastTry = time.Now()

	domain, err := settingSvc.GetWebDomain()
	if err != nil {
		return false, common.NewError("读取 webDomain 失败: ", err.Error())
	}
	if domain == "" {
		return false, common.NewError("证书剩余 ", info.DaysLeft, " 天需续签,但没有配置 webDomain —— 无法自动续,请到设置页走一次一键签发")
	}
	token, email := settingSvc.GetCfToken()
	if token == "" || email == "" {
		return false, common.NewError("证书剩余 ", info.DaysLeft, " 天需续签,但 Cloudflare Token / ACME 邮箱未保存 —— 无法自动续")
	}

	logger.Info("PanelSSLRenew: 证书剩余 ", info.DaysLeft, " 天(阈值 ", PanelCertRenewThresholdDays,
		"),开始为 ", domain, " 续签")

	// 只签发,不动 A 记录 —— 见文件头注释。
	certPath, keyPath, err := sslSvc.IssuePanelSSL(domain, email, token)
	if err != nil {
		return false, common.NewError("ACME 续签失败: ", err.Error())
	}
	for _, kv := range [][2]string{
		{"webCertFile", certPath},
		{"webKeyFile", keyPath},
	} {
		if err := settingSvc.saveSetting(kv[0], kv[1]); err != nil {
			// 证书文件已经签出来了,只是 settings 没指过去 —— 报错让运维知道要手动改,
			// 不能当成成功(否则面板重启后仍然加载旧证书,而日志说"续签成功")
			return false, common.NewError("续签已签发但写 setting ", kv[0], " 失败: ", err.Error())
		}
	}
	logger.Info("PanelSSLRenew: ", domain, " 续签成功,证书已更新,待面板重启加载")
	return true, nil
}
