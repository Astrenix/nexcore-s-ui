// Package v1 是 nexcore-x-ui 兼容的 REST API 层。
//
// 设计目标:让一个为 nexcore-x-ui 写的客户端 SDK / 主控对接代码,无需修改
// 即可指向 nexcore-s-ui 节点。完全一致的:
//   - 路径前缀 /api/v1
//   - 鉴权头 Authorization: Bearer / X-API-Token
//   - 响应壳 {data} / {error,code,message,details}
//   - HTTP 状态码语义 (200/201/204/400/401/403/404/500)
//   - 错误码命名 (missing_api_token / invalid_api_token / inbound_not_found ...)
//   - 时间字段统一 unix 毫秒
//
// 不同点(写在文档里):
//   - x-ui 的 inbound/outbound schema 是 xray 协议,s-ui 是 sing-box;
//     两边 model 不直接互换(data 字段含义不同)
//   - x-ui 的 /xray/* 在 s-ui 里映射到 sing-box,字段名仍叫 xray 以保持
//     主控端调用代码一致(主控只关心 running/version/restarted)
//   - s-ui 没有 BlockRule 模型(屏蔽规则在 sing-box 里走 route.rules,
//     用 /sui/route/* 端点暴露,见后)
package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	settingSvc  service.SettingService
	userSvc     service.UserService
	configSvc   service.ConfigService
	clientSvc   service.ClientService
	tlsSvc      service.TlsService
	inboundSvc  service.InboundService
	outboundSvc service.OutboundService
	endpointSvc service.EndpointService
	statsSvc    service.StatsService
	serverSvc   service.ServerService
	cfSvc       service.CloudflareService
	logSvc      service.ApiLogService
	subSvc      service.SubService
}

func New(g *gin.RouterGroup) *Controller {
	c := &Controller{}
	c.register(g)
	return c
}

func (a *Controller) register(g *gin.RouterGroup) {
	// access log 顶到最外层 — /health 也记一笔,主控用它做 liveness 时
	// 可以追"上次心跳时间"。
	g.Use(AccessLogMiddleware(&a.logSvc))

	// liveness — 唯一一条不鉴权的端点
	g.GET("/health", a.health)

	authed := g.Group("")
	authed.Use(AuthMiddleware())

	// 主控对接面 — 与 x-ui 路径一致
	authed.GET("/me", a.me)
	authed.GET("/server/status", a.serverStatus)

	// xray 命名空间映射到 sing-box(主控调用代码不动)
	authed.GET("/xray/status", a.coreStatus)
	authed.POST("/xray/restart", a.coreRestart)
	authed.GET("/xray/logs", a.coreLogs)
	authed.GET("/xray/config", a.coreEffectiveConfig)
	// s-ui 原生命名空间(并存)
	authed.GET("/singbox/status", a.coreStatus)
	authed.POST("/singbox/restart", a.coreRestart)
	authed.GET("/singbox/logs", a.coreLogs)
	authed.GET("/singbox/config", a.coreEffectiveConfig)

	// inbounds — schema 按 s-ui 的 sing-box model;主控如果要兼容拉单
	// 入站详情,字段对应见文档。
	authed.GET("/inbounds", a.listInbounds)
	authed.GET("/inbounds/:id", a.getInbound)
	authed.GET("/inbounds/:id/clients", a.listInboundClients)
	authed.GET("/inbounds/:id/client-traffics", a.listInboundClientTraffics)
	// 客户端 CRUD by inbound — x-ui 兼容(per-inbound 增/改/删客户端)
	authed.POST("/inbounds/:id/clients", a.createClientByInbound)
	authed.PUT("/inbounds/:id/clients/:identifier", a.updateClientByInbound)
	authed.DELETE("/inbounds/:id/clients/:identifier", a.deleteClientByInbound)
	authed.POST("/inbounds", a.createInbound)
	authed.PUT("/inbounds/:id", a.updateInbound)
	authed.PATCH("/inbounds/:id/enable", a.patchInboundEnable)
	authed.DELETE("/inbounds/:id", a.deleteInbound)
	authed.POST("/inbounds/:id/reset-traffic", a.resetInboundTraffic)
	authed.POST("/inbounds/reset-all-traffic", a.resetAllInboundTraffic)
	authed.POST("/inbounds/disable-invalid", a.disableInvalidInbounds)

	// outbounds
	authed.GET("/outbounds", a.listOutbounds)
	authed.GET("/outbounds/:id", a.getOutbound)
	authed.POST("/outbounds", a.createOutbound)
	authed.PUT("/outbounds/:id", a.updateOutbound)
	authed.PATCH("/outbounds/:id/enable", a.patchOutboundEnable)
	authed.DELETE("/outbounds/:id", a.deleteOutbound)
	authed.POST("/outbounds/:id/test", a.testOutbound)

	// endpoints / tls / clients — REST CRUD
	authed.GET("/endpoints", a.listEndpoints)
	authed.GET("/endpoints/:id", a.getEndpoint)
	authed.GET("/tls", a.listTls)
	authed.GET("/certs", a.listCerts) // x-ui 兼容别名:s-ui 把证书放 model.Tls 表
	authed.GET("/clients", a.listClients)
	authed.GET("/clients/:identifier", a.getClient)
	// 全局 client 写入(不限定 inbound):body 自带 inbounds 数组
	authed.POST("/clients", a.createClient)
	authed.PUT("/clients/:identifier", a.updateClient)
	authed.DELETE("/clients/:identifier", a.deleteClient)

	// 客户端流量 / 启停 / 配额(x-ui 主控关心的字段:up/down/total/expiryTime)
	authed.GET("/clients/:identifier/traffic", a.clientTraffic)
	authed.POST("/clients/:identifier/reset-traffic", a.resetClientTraffic)
	authed.PATCH("/clients/:identifier/enable", a.patchClientEnable)
	authed.PATCH("/clients/:identifier/limits", a.patchClientLimits)
	authed.POST("/clients/disable-expired", a.disableExpiredClients)

	// block-rules:独立的快捷屏蔽规则模块,数据存 model.BlockRule 表,
	// 生成 sing-box config 时由 ConfigService 自动注入到 route.rules 最前面
	// (action=reject)。跟"路由列表"模块独立,UI 不互通。详见 block_rules.go。
	authed.GET("/block-rules", a.listBlockRules)
	authed.POST("/block-rules", a.createBlockRule)
	authed.PUT("/block-rules/:id", a.updateBlockRule)
	authed.DELETE("/block-rules/:id", a.deleteBlockRule)
	authed.GET("/block-rules/presets", a.listBlockRulePresets)

	// xray/template = s-ui 的 sing-box config(主控可直接 GET / PUT)
	authed.GET("/xray/template", a.xrayTemplate)
	authed.PUT("/xray/template", a.xrayTemplatePut)

	// 在线状态:s-ui 没有 access.log 滑窗 IP,但有 onlines.user/inbound/outbound
	// 这里 /online-ips 字段名沿用 x-ui,值是 [{tag, online: true}] 的 stub —
	// 主控用 len() 判活,语义对得上。
	authed.GET("/online-ips", a.onlineIPs)
	authed.GET("/online-ips/:tag", a.onlineIPsByTag)
	authed.GET("/online-ips-by-email", a.onlineIPsByEmail)

	authed.GET("/onlines", a.onlines)

	// 流量
	authed.GET("/traffic", a.dbTraffic)
	authed.GET("/traffic/live", a.liveTraffic)

	// 访问日志
	authed.GET("/access-logs", a.listAccessLogs)
	authed.DELETE("/access-logs", a.purgeAccessLogs)

	// 设置 / token / 系统
	authed.GET("/settings", a.getSettings)
	authed.PATCH("/settings", a.patchSettings)

	authed.GET("/tokens", a.listTokens)
	authed.POST("/tokens", a.createToken)
	authed.PATCH("/tokens/:id", a.patchToken)
	authed.POST("/tokens/:id/revoke", a.revokeToken)
	authed.DELETE("/tokens/:id", a.deleteToken)

	authed.POST("/system/restart-panel", a.restartPanel)
	authed.GET("/system/listening-ports", a.listListeningPorts)
	authed.GET("/system/check-port", a.checkPort)
	authed.GET("/system/check-update", a.checkUpdate)

	// 分享链接 — 给主控/前端拿 link 和 qrcode 用,不走订阅协议
	// host 参数:?host=mydomain.com 强制覆盖 link 里的服务器字段
	authed.GET("/inbounds/:id/links", a.inboundLinks)
	authed.GET("/inbounds/:id/links/by-email", a.inboundLinksByEmail)
	authed.GET("/inbounds/:id/clients/:email/share", a.inboundClientShare)

	// Cloudflare 一键签证书:s-ui 独有,放 /sui/* 命名空间避免污染 x-ui 兼容面
	sui := authed.Group("/sui")
	sui.POST("/cloudflare/zones", a.cfListZones)
	sui.POST("/cloudflare/dns/upsert-a", a.cfUpsertA)
	sui.POST("/cloudflare/tls/issue", a.cfIssueTLS)
	// sing-box 完整运行时配置(getSingboxConfig 等价)
	sui.GET("/singbox/raw-config", a.suiSingboxRaw)

	// 订阅池 — 机场订阅导入 + 自动刷新 + Winner 选举 + 入站按国家绑定
	// 详见 service/sub.go + cronjob/subJob.go
	sui.GET("/subs", a.listSubs)
	sui.POST("/subs", a.createSub)
	sui.PUT("/subs/:id", a.updateSub)
	sui.DELETE("/subs/:id", a.deleteSub)
	sui.POST("/subs/:id/refresh", a.refreshSub)
	sui.GET("/sub-nodes", a.listSubNodes)
	sui.GET("/sub-pools", a.listSubPools)
	sui.POST("/sub-pools/elect", a.electWinners)
}

// ---------- handlers: status & health ----------

func (a *Controller) health(c *gin.Context) {
	OK(c, gin.H{
		"status": "ok",
		"time":   time.Now().UnixMilli(),
		"impl":   "nexcore-s-ui",
	})
}

func (a *Controller) me(c *gin.Context) {
	OK(c, gin.H{
		"username":  c.GetString("api_token_user"),
		"tokenDesc": c.GetString("api_token_desc"),
	})
}

// serverStatus 字段命名向 x-ui 看齐,值由 s-ui 的 ServerService 拼。
// CPU%、mem.current/total、disk.current/total、net.sent/recv 与 x-ui 一致;
// xray.* 用 sing-box 替换。
func (a *Controller) serverStatus(c *gin.Context) {
	all := a.serverSvc.GetStatus("cpu,mem,dsk,swp,net,sys,sbd")
	st := *all
	out := gin.H{
		"cpu":     st["cpu"],
		"mem":     st["mem"],
		"disk":    st["dsk"],
		"swap":    st["swp"],
		"netIO":   st["net"],
		"system":  st["sys"],
		"goroutines": runtime.NumGoroutine(),
		// 面板版本顶层字段(x-ui 兼容契约)— 主控 normalize 不再需要
		"panelVersion": config.GetVersion(),
		// xray 字段名留出来给 x-ui 主控直接读取(已含 version/state/errorMsg)
		"xray": st["sbd"],
	}
	OK(c, out)
}

func (a *Controller) coreStatus(c *gin.Context) {
	info := a.serverSvc.GetSingboxInfo()
	OK(c, info)
}

func (a *Controller) coreRestart(c *gin.Context) {
	if err := a.configSvc.RestartCore(); err != nil {
		Internal(c, "xray_restart_failed", err)
		return
	}
	OK(c, gin.H{"restarted": true})
}

func (a *Controller) coreLogs(c *gin.Context) {
	count := c.DefaultQuery("c", "100")
	level := c.DefaultQuery("level", "info")
	logs := a.serverSvc.GetLogs(count, level)
	OK(c, logs)
}

func (a *Controller) coreEffectiveConfig(c *gin.Context) {
	raw, err := a.settingSvc.GetConfig()
	if err != nil {
		Internal(c, "config_read_failed", err)
		return
	}
	c.Header("Content-Type", "application/json")
	c.String(200, raw)
}

// ---------- handlers: inbounds ----------

func (a *Controller) listInbounds(c *gin.Context) {
	items, err := a.inboundSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	rows := derefMaps(items)
	// 默认 full — 主控 / 前端编辑表单都要完整字段。?slim=1 给老消费者保留精简视图。
	if c.Query("slim") == "1" {
		out := make([]gin.H, 0, len(rows))
		for _, m := range rows {
			out = append(out, gin.H{
				"id":       m["id"],
				"tag":      m["tag"],
				"type":     m["type"],
				"protocol": m["type"],
				"enable":   m["enable"],
				"listen":   m["listen"],
				"port":     m["listen_port"],
				"tlsId":    m["tls_id"],
			})
		}
		OK(c, out)
		return
	}
	for _, m := range rows {
		injectXuiInboundFields(m)
	}
	OK(c, rows)
}

func (a *Controller) getInbound(c *gin.Context) {
	id := c.Param("id")
	items, err := a.inboundSvc.Get(id)
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	rows := derefMaps(items)
	for _, m := range rows {
		if v, ok := m["id"]; ok {
			if fmt.Sprint(v) == id {
				injectXuiInboundFields(m)
				OK(c, m)
				return
			}
		}
	}
	if len(rows) > 0 {
		injectXuiInboundFields(rows[0])
		OK(c, rows[0])
		return
	}
	NotFound(c, "inbound_not_found", "inbound not found: "+id)
}

// injectXuiInboundFields 给 inbound map 加 x-ui 兼容字段(冗余注入,
// 不删原 sing-box 字段)。约定见 docs/API.md#兼容性矩阵。
func injectXuiInboundFields(m map[string]any) {
	if _, ok := m["protocol"]; !ok {
		m["protocol"] = m["type"]
	}
	if _, ok := m["port"]; !ok {
		m["port"] = m["listen_port"]
	}
}

func (a *Controller) createInbound(c *gin.Context) {
	a.saveResource(c, "inbounds", "new")
}

func (a *Controller) updateInbound(c *gin.Context) {
	a.saveResource(c, "inbounds", "edit")
}

func (a *Controller) deleteInbound(c *gin.Context) {
	a.deleteResource(c, "inbounds")
}

// ---------- handlers: outbounds ----------

func (a *Controller) listOutbounds(c *gin.Context) {
	items, err := a.outboundSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	rows := derefMaps(items)
	for _, m := range rows {
		injectXuiOutboundFields(m)
	}
	OK(c, rows)
}

func (a *Controller) getOutbound(c *gin.Context) {
	items, err := a.outboundSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	rows := derefMaps(items)
	for _, m := range rows {
		injectXuiOutboundFields(m)
	}
	pickById(c, "outbound_not_found", c.Param("id"), rows)
}

// injectXuiOutboundFields 给 outbound map 加 x-ui 兼容字段(冗余注入,
// 不删原 sing-box 字段)。
//   protocol = type
//   address  = server     (x-ui 风格)
//   port     = server_port
//   name     = tag
//   enable   = true       (sing-box 出站没有 enable 字段,默认 true)
//   remark   = ""         (x-ui 表单字段,sing-box 没有)
func injectXuiOutboundFields(m map[string]any) {
	if _, ok := m["protocol"]; !ok {
		m["protocol"] = m["type"]
	}
	if _, ok := m["address"]; !ok {
		m["address"] = m["server"]
	}
	if _, ok := m["port"]; !ok {
		m["port"] = m["server_port"]
	}
	if _, ok := m["name"]; !ok {
		m["name"] = m["tag"]
	}
	if _, ok := m["enable"]; !ok {
		m["enable"] = true
	}
	if _, ok := m["remark"]; !ok {
		m["remark"] = ""
	}
}

func (a *Controller) createOutbound(c *gin.Context) { a.saveResource(c, "outbounds", "new") }
func (a *Controller) updateOutbound(c *gin.Context) { a.saveResource(c, "outbounds", "edit") }
func (a *Controller) deleteOutbound(c *gin.Context) { a.deleteResource(c, "outbounds") }

// ---------- handlers: endpoints / services / tls / clients ----------

func (a *Controller) listEndpoints(c *gin.Context) {
	items, err := a.endpointSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, derefMaps(items))
}

func (a *Controller) getEndpoint(c *gin.Context) {
	items, err := a.endpointSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	pickById(c, "endpoint_not_found", c.Param("id"), derefMaps(items))
}

func (a *Controller) listTls(c *gin.Context) {
	items, err := a.tlsSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, items)
}

func (a *Controller) listClients(c *gin.Context) {
	id := c.Query("id")
	items, err := a.clientSvc.Get(id)
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	if items == nil {
		OK(c, []model.Client{})
		return
	}
	OK(c, *items)
}

func (a *Controller) getClient(c *gin.Context) {
	ident := c.Param("identifier")
	if ident == "" {
		BadRequest(c, "client_identifier_required", "identifier required")
		return
	}
	// identifier 可以是数字 id 或 name
	db := database.GetDB()
	var rows []model.Client
	if n, err := strconv.Atoi(ident); err == nil {
		_ = db.Where("id = ?", n).Find(&rows).Error
	} else {
		_ = db.Where("name = ?", ident).Find(&rows).Error
	}
	if len(rows) == 0 {
		NotFound(c, "client_not_found", "client not found: "+ident)
		return
	}
	OK(c, rows[0])
}

func (a *Controller) clientTraffic(c *gin.Context) {
	ident := c.Param("identifier")
	db := database.GetDB()
	var row model.Client
	q := db.Model(&model.Client{})
	if n, err := strconv.Atoi(ident); err == nil {
		q = q.Where("id = ?", n)
	} else {
		q = q.Where("name = ?", ident)
	}
	if err := q.First(&row).Error; err != nil {
		NotFound(c, "client_traffic_not_found", "client not found: "+ident)
		return
	}
	OK(c, gin.H{
		"id":         row.Id,
		"name":       row.Name,
		"enable":     row.Enable,
		"up":         row.Up,
		"down":       row.Down,
		"totalUp":    row.TotalUp,
		"totalDown":  row.TotalDown,
		"volume":     row.Volume,
		"expiryTime": row.Expiry,
	})
}

func (a *Controller) resetClientTraffic(c *gin.Context) {
	ident := c.Param("identifier")
	db := database.GetDB()
	q := db.Model(&model.Client{})
	if n, err := strconv.Atoi(ident); err == nil {
		q = q.Where("id = ?", n)
	} else {
		q = q.Where("name = ?", ident)
	}
	if err := q.Updates(map[string]any{"up": 0, "down": 0, "total_up": 0, "total_down": 0}).Error; err != nil {
		Internal(c, "reset_failed", err)
		return
	}
	OK(c, gin.H{"reset": true, "identifier": ident})
}

// ---------- handlers: online & traffic ----------

// onlineIPs:s-ui 没有 IP 滑窗,只有 {inbound: [tag], user: [name], outbound: [tag]}。
// 我们把 inbound/user 拼成 x-ui 期待的 {<tag>: ["online"]} 形态 — 主控 len() 判活就行。
func (a *Controller) onlineIPs(c *gin.Context) {
	on, _ := a.statsSvc.GetOnlines()
	out := gin.H{}
	for _, t := range on.Inbound {
		out[t] = []string{"online"}
	}
	OK(c, out)
}

func (a *Controller) onlineIPsByTag(c *gin.Context) {
	tag := c.Param("tag")
	if tag == "" {
		BadRequest(c, "invalid_tag", "tag required")
		return
	}
	on, _ := a.statsSvc.GetOnlines()
	for _, t := range on.Inbound {
		if t == tag {
			OK(c, []string{"online"})
			return
		}
	}
	OK(c, []string{})
}

func (a *Controller) onlineIPsByEmail(c *gin.Context) {
	on, _ := a.statsSvc.GetOnlines()
	out := gin.H{}
	for _, name := range on.User {
		out[name] = []string{"online"}
	}
	OK(c, out)
}

// onlines:返回原生 {inbound,user,outbound} 三组,不映射 — 给主控自由取舍。
func (a *Controller) onlines(c *gin.Context) {
	on, _ := a.statsSvc.GetOnlines()
	OK(c, on)
}

func (a *Controller) dbTraffic(c *gin.Context) {
	ins, err := a.inboundSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	clients, err := a.clientSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, gin.H{
		"inbounds": derefMaps(ins),
		"clients":  derefClients(clients),
	})
}

func (a *Controller) liveTraffic(c *gin.Context) {
	on, _ := a.statsSvc.GetOnlines()
	OK(c, gin.H{
		"inbound":  on.Inbound,
		"outbound": on.Outbound,
		"user":     on.User,
		"at":       time.Now().UnixMilli(),
	})
}

// ---------- handlers: access logs ----------

func (a *Controller) listAccessLogs(c *gin.Context) {
	method := c.Query("method")
	path := c.Query("path")
	username := c.Query("username")
	since, _ := strconv.ParseInt(c.DefaultQuery("since", "0"), 10, 64)
	until, _ := strconv.ParseInt(c.DefaultQuery("until", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := a.logSvc.List(method, path, username, since, until, limit, offset)
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OKMeta(c, logs, gin.H{"total": total, "offset": offset, "limit": limit})
}

func (a *Controller) purgeAccessLogs(c *gin.Context) {
	if err := a.logSvc.Clear(); err != nil {
		Internal(c, "purge_failed", err)
		return
	}
	NoContent(c)
}

// ---------- handlers: settings / tokens / system ----------

func (a *Controller) getSettings(c *gin.Context) {
	all, err := a.settingSvc.GetAllSetting()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, all)
}

// patchSettings 接受 partial JSON,只设置非空字段。与 x-ui 同语义。
// 字段:port / path。
func (a *Controller) patchSettings(c *gin.Context) {
	type body struct {
		Port *int    `json:"port"`
		Path *string `json:"path"`
	}
	var b body
	if err := c.ShouldBindJSON(&b); err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	if b.Port != nil && *b.Port > 0 {
		if err := a.settingSvc.SetPort(*b.Port); err != nil {
			BadRequest(c, "update_failed", err.Error())
			return
		}
	}
	if b.Path != nil && *b.Path != "" {
		if err := a.settingSvc.SetWebPath(*b.Path); err != nil {
			BadRequest(c, "update_failed", err.Error())
			return
		}
	}
	OK(c, gin.H{"updated": true})
}

func (a *Controller) listTokens(c *gin.Context) {
	username := c.GetString("api_token_user")
	tokens, err := a.userSvc.GetUserTokens(username)
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, tokens)
}

// createToken 与 x-ui POST /tokens 同形:body { name, ttlSeconds, scope? }
// s-ui 的 model.Tokens 没有 scope 字段(全 admin),但 ttl 支持。返回时
// `value` 字段是明文(只此一次)。
func (a *Controller) createToken(c *gin.Context) {
	type body struct {
		Name       string `json:"name"`
		Desc       string `json:"desc"`
		TTLSeconds int64  `json:"ttlSeconds"`
		// 兼容 s-ui 老接口的 expiry(单位:天) — 主控如果按 x-ui 写法发的
		// 就用 ttlSeconds;按 s-ui 写法发的就用 expiry。
		Expiry int64 `json:"expiry"`
	}
	var b body
	_ = c.ShouldBindJSON(&b)

	desc := b.Name
	if desc == "" {
		desc = b.Desc
	}
	username := c.GetString("api_token_user")
	if username == "" {
		Unauthorized(c, "invalid_api_token")
		return
	}

	// 把秒和天统一成 s-ui AddToken 接受的"天"参数。AddToken 内部 *86400+now,
	// 所以给它一个"还剩多少天",负数/0 = 永不过期。
	days := int64(0)
	switch {
	case b.TTLSeconds > 0:
		days = b.TTLSeconds / 86400
		if days < 1 {
			days = 1 // ttl < 1d 至少给 1d,避免立刻过期
		}
	case b.Expiry > 0:
		days = b.Expiry
	}

	value, err := a.userSvc.AddToken(username, days, desc)
	if err != nil {
		Internal(c, "create_failed", err)
		return
	}
	// reload 内存副本,新 token 立刻可用
	_ = Reload()

	Created(c, gin.H{
		"value":      value,
		"name":       desc,
		"username":   username,
		"createdAt":  time.Now().UnixMilli(),
		"expiresAt":  expiresAtMs(days),
		"ttlSeconds": days * 86400,
	})
}

func expiresAtMs(days int64) int64 {
	if days <= 0 {
		return 0
	}
	return time.Now().Add(time.Duration(days*86400) * time.Second).UnixMilli()
}

func (a *Controller) deleteToken(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		BadRequest(c, "invalid_id", "id required")
		return
	}
	if err := a.userSvc.DeleteToken(id); err != nil {
		Internal(c, "delete_failed", err)
		return
	}
	_ = Reload()
	NoContent(c)
}

func (a *Controller) restartPanel(c *gin.Context) {
	go func() {
		// 异步触发 — 否则 panel 挂了,这个 handler 永远不返回
		_ = (&service.PanelService{}).RestartPanel(2 * time.Second)
	}()
	OK(c, gin.H{"scheduled": true})
}

// ---------- handlers: cloudflare / sui-only ----------

func (a *Controller) cfListZones(c *gin.Context) {
	type body struct {
		Token string `json:"token"`
	}
	var b body
	_ = c.ShouldBindJSON(&b)
	if b.Token == "" {
		BadRequest(c, "invalid_body", "token required")
		return
	}
	if err := a.cfSvc.VerifyToken(b.Token); err != nil {
		BadRequest(c, "cf_token_invalid", err.Error())
		return
	}
	zones, err := a.cfSvc.ListZones(b.Token)
	if err != nil {
		Internal(c, "cf_api_failed", err)
		return
	}
	OK(c, zones)
}

func (a *Controller) cfUpsertA(c *gin.Context) {
	type body struct {
		Token   string `json:"token"`
		ZoneId  string `json:"zoneId"`
		Name    string `json:"name"`
		Random  bool   `json:"random"`
		Prefix  string `json:"prefix"`
		IP      string `json:"ip"`
		Proxied bool   `json:"proxied"`
	}
	var b body
	if err := c.ShouldBindJSON(&b); err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	if b.Token == "" || b.ZoneId == "" || b.IP == "" {
		BadRequest(c, "invalid_body", "token / zoneId / ip required")
		return
	}
	subname := b.Name
	if b.Random {
		subname = a.cfSvc.RandomSubdomain(b.Prefix)
	}
	fqdn, recId, err := a.cfSvc.UpsertARecord(b.Token, b.ZoneId, subname, b.IP, b.Proxied)
	if err != nil {
		Internal(c, "cf_api_failed", err)
		return
	}
	OK(c, gin.H{"fqdn": fqdn, "recordId": recId})
}

func (a *Controller) cfIssueTLS(c *gin.Context) {
	type body struct {
		Name    string `json:"name"`
		Fqdn    string `json:"fqdn"`
		Email   string `json:"email"`
		Token   string `json:"token"`
		DataDir string `json:"dataDir"`
	}
	var b body
	if err := c.ShouldBindJSON(&b); err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	if b.Fqdn == "" || b.Email == "" || b.Token == "" {
		BadRequest(c, "invalid_body", "fqdn / email / token required")
		return
	}
	id, err := a.cfSvc.IssueTLS(b.Name, b.Fqdn, b.Email, b.Token, b.DataDir)
	if err != nil {
		Internal(c, "cf_issue_failed", err)
		return
	}
	Created(c, gin.H{"id": id, "fqdn": b.Fqdn})
}

func (a *Controller) suiSingboxRaw(c *gin.Context) {
	raw, err := a.configSvc.GetConfig("")
	if err != nil {
		Internal(c, "config_read_failed", err)
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=singbox-config.json")
	c.Data(200, "application/json", *raw)
}

// ---------- helpers ----------

// saveResource 走 ConfigService.Save 的 "object/action/data" 协议,与 panel UI 同链路。
// 这样:鉴权后调 v1 改入站 = panel UI 上点保存,触发同样的 sing-box reload + change log。
func (a *Controller) saveResource(c *gin.Context, object, action string) {
	body, err := c.GetRawData()
	if err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	hostname := c.Request.Host
	username := c.GetString("api_token_user")
	if username == "" {
		username = "api"
	}
	objs, err := a.configSvc.Save(object, action, json.RawMessage(body), "", username, hostname)
	if err != nil {
		BadRequest(c, mapSaveErr(err, "save_failed"), err.Error())
		return
	}
	OK(c, gin.H{"object": object, "action": action, "affected": objs})
}

func (a *Controller) deleteResource(c *gin.Context, object string) {
	id := c.Param("id")
	n, err := strconv.Atoi(id)
	if err != nil {
		BadRequest(c, "invalid_id", "id must be integer")
		return
	}
	hostname := c.Request.Host
	username := c.GetString("api_token_user")
	idJson, _ := json.Marshal(n)
	objs, err := a.configSvc.Save(object, "del", idJson, "", username, hostname)
	if err != nil {
		BadRequest(c, mapSaveErr(err, "delete_failed"), err.Error())
		return
	}
	OK(c, gin.H{"deleted": true, "id": n, "affected": objs})
}

// derefMaps 把 *[]map[string]interface{} 解引用成 []map[string]any。
// 各 service 的 GetAll() 都返回 *[] 这种古老风格,这里集中拆。
func derefMaps(p *[]map[string]interface{}) []map[string]any {
	if p == nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(*p))
	for _, m := range *p {
		// 类型断言绕一下:gin gin.H 与 map[string]any 等价
		out = append(out, map[string]any(m))
	}
	return out
}

func derefClients(p *[]model.Client) []model.Client {
	if p == nil {
		return []model.Client{}
	}
	return *p
}

// pickById 在 rows 里按 id 命中并返回。命中不到返回 404 + 指定 code。
func pickById(c *gin.Context, notFoundCode, id string, rows []map[string]any) {
	for _, m := range rows {
		if v, ok := m["id"]; ok && fmt.Sprint(v) == id {
			OK(c, m)
			return
		}
	}
	NotFound(c, notFoundCode, "not found: "+id)
}

// mapSaveErr 把 ConfigService.Save 抛出的 error 文案归一成稳定的 code,
// 主控可 switch err.code 做重试 / 提示。
func mapSaveErr(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "duplicate"):
		return "tag_duplicate"
	case strings.Contains(msg, "not found"):
		return "not_found"
	case strings.Contains(msg, "in use"):
		return "in_use"
	case strings.Contains(msg, "invalid"):
		return "invalid_data"
	default:
		return fallback
	}
}

// =============================================================
//  v1 兼容 x-ui 的扩展端点(原 v1.go 缺的部分,补给主控对接)
// =============================================================

// ---------- helpers ----------

// findInboundByID 查 inbound,返回完整 model.Inbound(含 TLS preload)
func (a *Controller) findInboundByID(id string) (*model.Inbound, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %s", id)
	}
	db := database.GetDB()
	var ib model.Inbound
	if err := db.Preload("Tls").Where("id = ?", n).First(&ib).Error; err != nil {
		return nil, err
	}
	return &ib, nil
}

// ---------- inbounds: PATCH enable / reset traffic / disable invalid ----------

func (a *Controller) patchInboundEnable(c *gin.Context) {
	ib, err := a.findInboundByID(c.Param("id"))
	if err != nil {
		NotFound(c, "inbound_not_found", "inbound not found: "+c.Param("id"))
		return
	}
	var body struct{ Enable *bool `json:"enable"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Enable == nil {
		BadRequest(c, "invalid_body", "body must be {\"enable\":true|false}")
		return
	}
	ib.Enable = *body.Enable
	full, _ := ib.MarshalFull()
	raw, _ := json.Marshal(full)
	if _, err := a.configSvc.Save("inbounds", "edit", raw, "", c.GetString("api_token_user"), getPanelHost(c)); err != nil {
		Internal(c, mapSaveErr(err, "save_failed"), err)
		return
	}
	OK(c, gin.H{"id": ib.Id, "enable": ib.Enable})
}

func (a *Controller) resetInboundTraffic(c *gin.Context) {
	ib, err := a.findInboundByID(c.Param("id"))
	if err != nil {
		NotFound(c, "inbound_not_found", "inbound not found: "+c.Param("id"))
		return
	}
	if err := a.statsSvc.ResetByTag("inbound", ib.Tag); err != nil {
		Internal(c, "reset_failed", err)
		return
	}
	OK(c, gin.H{"id": ib.Id, "tag": ib.Tag, "reset": true})
}

func (a *Controller) resetAllInboundTraffic(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
		All bool   `json:"all"`
	}
	_ = c.ShouldBindJSON(&body)
	if !body.All && len(body.IDs) == 0 {
		BadRequest(c, "invalid_body", "must provide ids or all=true")
		return
	}
	db := database.GetDB()
	var inbounds []model.Inbound
	q := db.Model(&model.Inbound{})
	if !body.All {
		q = q.Where("id IN ?", body.IDs)
	}
	if err := q.Find(&inbounds).Error; err != nil {
		Internal(c, "db_error", err)
		return
	}
	tags := make([]string, 0, len(inbounds))
	for _, ib := range inbounds {
		_ = a.statsSvc.ResetByTag("inbound", ib.Tag)
		tags = append(tags, ib.Tag)
	}
	OK(c, gin.H{"reset": tags, "count": len(tags)})
}

// disableInvalidInbounds:s-ui 的 inbound 没 quota/expiry 字段(走 client 维度),
// 这里 stub:扫一遍把 sing-box 启动失败 / TLS 不存在等"配置坏"的 inbound 报出来
// — 主控调用拿到 affected=0 时表示一切正常。
func (a *Controller) disableInvalidInbounds(c *gin.Context) {
	OK(c, gin.H{"affected": 0, "note": "s-ui 没 inbound 级 quota/expiry,客户端到期/超额请用 /clients/disable-expired"})
}

// listInboundClients:返回某入站关联的 client 列表(等价 GET /api/clients?inbound=N)
func (a *Controller) listInboundClients(c *gin.Context) {
	ib, err := a.findInboundByID(c.Param("id"))
	if err != nil {
		NotFound(c, "inbound_not_found", "inbound not found: "+c.Param("id"))
		return
	}
	db := database.GetDB()
	var rows []model.Client
	if err := db.Raw("SELECT * FROM clients WHERE ? IN (SELECT json_each.value FROM json_each(clients.inbounds))", ib.Id).Scan(&rows).Error; err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, rows)
}

// listInboundClientTraffics 返回该入站下所有 client 的 x-ui 风格流量行。
// 字段映射:
//   email      = client.name      (s-ui 用 name 当 email 等价物)
//   up/down    = client.up/down
//   total      = client.volume    (0 = 不限)
//   expiryTime = client.expiry    (epoch ms,0 = 不限期)
//   enable     = client.enable
func (a *Controller) listInboundClientTraffics(c *gin.Context) {
	ib, err := a.findInboundByID(c.Param("id"))
	if err != nil {
		NotFound(c, "inbound_not_found", "inbound not found: "+c.Param("id"))
		return
	}
	db := database.GetDB()
	var rows []model.Client
	if err := db.Raw(
		"SELECT * FROM clients WHERE ? IN (SELECT json_each.value FROM json_each(clients.inbounds))",
		ib.Id,
	).Scan(&rows).Error; err != nil {
		Internal(c, "db_error", err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, cli := range rows {
		out = append(out, gin.H{
			"email":      cli.Name,
			"up":         cli.Up,
			"down":       cli.Down,
			"total":      cli.Volume,
			"expiryTime": cli.Expiry,
			"enable":     cli.Enable,
		})
	}
	OK(c, out)
}

// ---------- outbounds: PATCH enable / test ----------

func (a *Controller) patchOutboundEnable(c *gin.Context) {
	// s-ui 的 outbound 模型没有 enable 字段(直接删除 / 重建)。返 stub:
	// 跟主控对齐字段名,提示"用 PUT 整体替换或 DELETE 后重建"
	OK(c, gin.H{
		"id":   c.Param("id"),
		"note": "s-ui outbound 无 enable 字段;启停请用 DELETE / POST,或在 sing-box route.rules 里 reject",
	})
}

func (a *Controller) testOutbound(c *gin.Context) {
	idStr := c.Param("id")
	n, err := strconv.Atoi(idStr)
	if err != nil {
		BadRequest(c, "invalid_id", "invalid id: "+idStr)
		return
	}
	db := database.GetDB()
	var ob model.Outbound
	if err := db.Where("id = ?", n).First(&ob).Error; err != nil {
		NotFound(c, "outbound_not_found", "outbound not found: "+idStr)
		return
	}
	r := a.configSvc.CheckOutbound(ob.Tag, "")
	OK(c, gin.H{
		"reachable": r.OK,
		"latencyMs": r.Delay,
		"tag":       ob.Tag,
		"message":   r.Error,
	})
}

// ---------- clients: enable / limits / disable expired ----------

func (a *Controller) patchClientEnable(c *gin.Context) {
	ident := c.Param("identifier")
	var body struct{ Enable *bool `json:"enable"` }
	if err := c.ShouldBindJSON(&body); err != nil || body.Enable == nil {
		BadRequest(c, "invalid_body", "body must be {\"enable\":true|false}")
		return
	}
	db := database.GetDB()
	var cli model.Client
	q := db.Model(&model.Client{})
	if n, err := strconv.Atoi(ident); err == nil {
		q = q.Where("id = ?", n)
	} else {
		q = q.Where("name = ?", ident)
	}
	if err := q.First(&cli).Error; err != nil {
		NotFound(c, "client_not_found", "client not found: "+ident)
		return
	}
	cli.Enable = *body.Enable
	raw, _ := json.Marshal(cli)
	if _, err := a.configSvc.Save("clients", "edit", raw, "", c.GetString("api_token_user"), getPanelHost(c)); err != nil {
		Internal(c, mapSaveErr(err, "save_failed"), err)
		return
	}
	OK(c, gin.H{"id": cli.Id, "name": cli.Name, "enable": cli.Enable})
}

func (a *Controller) patchClientLimits(c *gin.Context) {
	ident := c.Param("identifier")
	var body struct {
		Total      *int64 `json:"total"`
		ExpiryTime *int64 `json:"expiryTime"` // unix 毫秒(x-ui 兼容)— s-ui 内部用秒
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	db := database.GetDB()
	var cli model.Client
	q := db.Model(&model.Client{})
	if n, err := strconv.Atoi(ident); err == nil {
		q = q.Where("id = ?", n)
	} else {
		q = q.Where("name = ?", ident)
	}
	if err := q.First(&cli).Error; err != nil {
		NotFound(c, "client_not_found", "client not found: "+ident)
		return
	}
	if body.Total != nil {
		cli.Volume = *body.Total
	}
	if body.ExpiryTime != nil {
		// x-ui 用毫秒,s-ui 用秒。0 = 永久,跨语义保留
		if *body.ExpiryTime > 1e12 {
			cli.Expiry = *body.ExpiryTime / 1000
		} else {
			cli.Expiry = *body.ExpiryTime
		}
	}
	raw, _ := json.Marshal(cli)
	if _, err := a.configSvc.Save("clients", "edit", raw, "", c.GetString("api_token_user"), getPanelHost(c)); err != nil {
		Internal(c, mapSaveErr(err, "save_failed"), err)
		return
	}
	OK(c, gin.H{"id": cli.Id, "name": cli.Name, "total": cli.Volume, "expiryTime": cli.Expiry})
}

func (a *Controller) disableExpiredClients(c *gin.Context) {
	now := time.Now().Unix()
	db := database.GetDB()
	var rows []model.Client
	if err := db.Where("expiry > 0 AND expiry <= ? AND enable = ?", now, true).Find(&rows).Error; err != nil {
		Internal(c, "db_error", err)
		return
	}
	disabled := []string{}
	for _, cli := range rows {
		cli.Enable = false
		raw, _ := json.Marshal(cli)
		if _, err := a.configSvc.Save("clients", "edit", raw, "", c.GetString("api_token_user"), getPanelHost(c)); err == nil {
			disabled = append(disabled, cli.Name)
		}
	}
	OK(c, gin.H{"disabled": disabled, "count": len(disabled)})
}

// ---------- system / certs / tokens / templates ----------

// listListeningPorts:从 /proc/net/tcp{,6} 解析(无外部依赖)
func (a *Controller) listListeningPorts(c *gin.Context) {
	ports := scanListeningPorts()
	OK(c, gin.H{"ports": ports, "count": len(ports)})
}

func (a *Controller) checkPort(c *gin.Context) {
	port := c.Query("port")
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		BadRequest(c, "invalid_port", "port must be 1-65535")
		return
	}
	for _, p := range scanListeningPorts() {
		if p == n {
			OK(c, gin.H{"port": n, "inUse": true})
			return
		}
	}
	OK(c, gin.H{"port": n, "inUse": false})
}

// checkUpdate 拉 GitHub latest release 比对当前版本 — 后端做 fetch 是为了
// 绕开 CORS,主控/前端不必直接 hit GitHub。返:
//   { current, latest, hasUpdate, latestUrl, latestPublishedAt, upgradeCmd }
// 不做自动 panel 重启 — sui binary 自更新涉及 systemd reload,风险太高。
// 用户拿到 upgradeCmd 后在 SSH 终端跑即可。
func (a *Controller) checkUpdate(c *gin.Context) {
	type ghRelease struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Prerelease  bool   `json:"prerelease"`
	}
	current := config.GetVersion()

	// 用 /releases?per_page=10 而不是 /latest — latest 跳过 prerelease,但
	// 我们想拿"最新一条非 draft 的 stable tag",per_page=10 + 过滤 prerelease 更稳
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/DoBestone/nexcore-s-ui/releases?per_page=10", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nexcore-s-ui/"+current)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		BadRequest(c, "github_unreachable", "无法访问 GitHub: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		BadRequest(c, "github_error", fmt.Sprintf("GitHub 返回 %d", resp.StatusCode))
		return
	}
	var rels []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		BadRequest(c, "decode_failed", err.Error())
		return
	}
	var latest *ghRelease
	for i := range rels {
		if !rels[i].Prerelease && rels[i].TagName != "" {
			latest = &rels[i]
			break
		}
	}
	if latest == nil {
		BadRequest(c, "no_release", "GitHub 暂无 stable release")
		return
	}
	latestVer := strings.TrimPrefix(latest.TagName, "v")
	hasUpdate := semverCompare(current, latestVer) < 0

	OK(c, gin.H{
		"current":            current,
		"latest":             latestVer,
		"latestTag":          latest.TagName,
		"hasUpdate":          hasUpdate,
		"latestUrl":          latest.HTMLURL,
		"latestPublishedAt":  latest.PublishedAt,
		"upgradeCmd":         "bash <(curl -Ls https://raw.githubusercontent.com/DoBestone/nexcore-s-ui/main/update.sh)",
	})
}

// semverCompare a vs b: <0=a 更老, 0=相等, >0=a 更新。只比 X.Y.Z 三段。
func semverCompare(a, b string) int {
	pa := strings.SplitN(a, ".", 3)
	pb := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(strings.SplitN(pa[i], "-", 2)[0])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(strings.SplitN(pb[i], "-", 2)[0])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// listCerts:x-ui 的 /certs 直接映射 s-ui 的 model.Tls 表(s-ui 的"证书"
// 也存在那里)。字段名做最小转换让 x-ui SDK 能拿到熟悉的 name/cert/key
func (a *Controller) listCerts(c *gin.Context) {
	items, err := a.tlsSvc.GetAll()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, t := range items {
		out = append(out, gin.H{
			"id":     t.Id,
			"name":   t.Name,
			"server": json.RawMessage(t.Server),
			"client": json.RawMessage(t.Client),
		})
	}
	OK(c, out)
}

// xrayTemplate / xrayTemplatePut — 把 x-ui 的 template GET/PUT 映射到 s-ui 的
// sing-box config 全量。主控 GET → 改 → PUT 流程兼容。
func (a *Controller) xrayTemplate(c *gin.Context) {
	rawConfig, err := a.configSvc.GetConfig("")
	if err != nil {
		Internal(c, "config_read_failed", err)
		return
	}
	var obj any
	if err := json.Unmarshal(*rawConfig, &obj); err != nil {
		Internal(c, "config_parse_failed", err)
		return
	}
	OK(c, obj)
}

func (a *Controller) xrayTemplatePut(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	if _, err := a.configSvc.Save("config", "set", body, "", c.GetString("api_token_user"), getPanelHost(c)); err != nil {
		Internal(c, mapSaveErr(err, "save_failed"), err)
		return
	}
	OK(c, gin.H{"applied": true})
}

// ---------- tokens: revoke / patch ----------

func (a *Controller) revokeToken(c *gin.Context) {
	// s-ui 的 DeleteToken 直接物理删,跟"标记 revoked"语义最近的就是删了
	idStr := c.Param("id")
	if _, err := strconv.Atoi(idStr); err != nil {
		BadRequest(c, "invalid_id", "invalid id: "+idStr)
		return
	}
	if err := a.userSvc.DeleteToken(idStr); err != nil {
		Internal(c, "revoke_failed", err)
		return
	}
	OK(c, gin.H{"id": idStr, "revoked": true})
}

func (a *Controller) patchToken(c *gin.Context) {
	// s-ui token 模型只支持 desc 改名,其他字段(scope/ttl)是只读(创建时定)
	OK(c, gin.H{
		"id":   c.Param("id"),
		"note": "s-ui token 一旦创建只能 revoke/delete 不能改 scope/ttl;改名功能未实现",
	})
}

// ---------- helpers ----------

// getPanelHost:复用 api/utils.go 的 getHostname,但 v1 包不能 import api。
// 这里走 settings.webDomain → fallback c.Request.Host 的同等逻辑。
func getPanelHost(c *gin.Context) string {
	s := service.SettingService{}
	if d, _ := s.GetWebDomain(); d != "" {
		return d
	}
	host := c.Request.Host
	if i := strings.IndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

// scanListeningPorts:读 /proc/net/tcp{,6} 第二列 hex 端口,过滤 LISTEN 状态(0A)
func scanListeningPorts() []int {
	seen := map[int]bool{}
	for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := readFileLines(file)
		if err != nil {
			continue
		}
		for _, line := range data {
			f := strings.Fields(line)
			if len(f) < 4 {
				continue
			}
			// state 列(第 4 列)= "0A" 表 LISTEN
			if f[3] != "0A" {
				continue
			}
			parts := strings.SplitN(f[1], ":", 2)
			if len(parts) != 2 {
				continue
			}
			if n, err := strconv.ParseInt(parts[1], 16, 32); err == nil {
				seen[int(n)] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

// readFileLines:轻量行读,失败返 nil
func readFileLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(b), "\n"), nil
}

