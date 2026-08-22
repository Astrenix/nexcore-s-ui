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

	// panelCertRenewLastTrySettingKey 上次续签【尝试】时间(RFC3339),持久化在 settings 表。
	//
	// 🩸 为什么不能只靠上面那个内存变量:本作业在面板启动 2 分钟后就会跑一次
	// (cronjob/cronJob.go 的首检 goroutine),而进程内变量【一重启就归零】——
	// 于是"6 小时最小间隔"对重启完全无效。
	// 而重启恰恰是本节流要防的场景:上面写的 8/6 事故形态就是 crash-loop 反复撞 429;
	// 运维在证书出问题时最自然的动作也是重启面板,每重启一次就多烧一张 LE 配额
	// (同域名每周 5 张重复证书),打满之后 429 的 retry-after 可达数天 ——
	// 恢复窗口被自己毒化,正是那次事故的形态。
	//
	// 不走 getString/defaultValueMap:那张表会被 setting.go:92 整表遍历成设置页 payload,
	// 内部记账键不该出现在面板界面上。直接 getSetting + saveSetting。
	panelCertRenewLastTrySettingKey = "panelCertRenewLastTry"
)

var (
	panelCertRenewMu      sync.Mutex
	panelCertRenewLastTry time.Time
)

// loadPanelCertRenewLastTry 读持久化的上次尝试时间。
//
// 读不到一律返回零值(= 节流放行)。**这个方向是刻意的**:
// fail-open 的代价是"多试一次 ACME",fail-closed 的代价是"证书到期了却因为
// 读不到一个时间戳而永远不续" —— 后者正是本文件开头那次事故本身。
func loadPanelCertRenewLastTry(settingSvc *SettingService) time.Time {
	st, err := settingSvc.getSetting(panelCertRenewLastTrySettingKey)
	if err != nil || st == nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, st.Value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// shouldThrottlePanelCertRenew 纯判定:这次尝试该不该被节流挡下。
//
// 与 shouldRenewPanelCert 同样抽成纯函数,理由见那里 —— 这条判断错一次的代价是
// 烧光 LE 配额(见 panelCertRenewMinInterval 的注释),而它涉及两个时间来源,
// 光看调用点读不出「重启后还挡不挡得住」。
//
// 取 mem / persisted 里更晚的那个:
//   - persisted 负责跨重启(内存那份一重启就归零,而重启正是要防的场景)
//   - mem 负责同进程内的快速路径,并且在 saveSetting 失败时仍然挡得住本进程
func shouldThrottlePanelCertRenew(mem, persisted, now time.Time) bool {
	lastTry := mem
	if persisted.After(lastTry) {
		lastTry = persisted
	}
	if lastTry.IsZero() {
		return false
	}
	return now.Sub(lastTry) < panelCertRenewMinInterval
}

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
		// 面板跑在 HTTP 上(没配证书)——这是合法配置,不是故障。
		// 仍然打一条:否则"自动续签没在跑"与"没有证书要续"在日志里长得一模一样。
		logger.Info("PanelSSLRenew: 未配置面板证书(webCertFile 为空),跳过")
		return false, nil
	}

	info := sslSvc.GetPanelCertInfo(certFile)
	if info.Error != "" {
		return false, common.NewError("解析面板证书失败: ", info.Error)
	}
	if !shouldRenewPanelCert(info) {
		// 🩸 心跳:这条 info 是"自动续签确实在跑"的唯一凭据。
		// 上一次同类事故(APIAbuseGuard 在生产从未运行过一秒)就是因为防御性代码
		// 平时零输出 —— "没在跑"和"在跑但没到阈值"外观完全一致,而等到真该它干活时
		// 你会先去调阈值。每 6h 一条,不算刷屏。
		logger.Info("PanelSSLRenew: 证书剩余 ", info.DaysLeft, " 天(阈值 ",
			PanelCertRenewThresholdDays, "),无需续签")
		return false, nil
	}

	// 到这里已经确定"该续了"。节流只挡重复**尝试**,不挡判定 —— 判定结果要能进日志。
	if shouldThrottlePanelCertRenew(panelCertRenewLastTry, loadPanelCertRenewLastTry(&settingSvc), time.Now()) {
		logger.Info("PanelSSLRenew: 证书剩余 ", info.DaysLeft, " 天需续签,但距上次尝试不足 ",
			panelCertRenewMinInterval, ",本轮跳过")
		return false, nil
	}
	panelCertRenewLastTry = time.Now()
	// 先落盘再跑 ACME:顺序反了的话,ACME 挂住/进程被杀期间这次尝试就没被记下来,
	// 重启后又是一次全新尝试 —— 那正是要防的 crash-loop 形态。
	if err := settingSvc.saveSetting(panelCertRenewLastTrySettingKey,
		panelCertRenewLastTry.Format(time.RFC3339)); err != nil {
		// 记不下来 = 重启后节流失效。不阻断本次续签(证书要紧),但必须出声。
		logger.Warning("PanelSSLRenew: 无法持久化续签尝试时间,重启后节流会失效: ", err.Error())
	}

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
