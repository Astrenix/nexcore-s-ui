package api

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.PanelService
	service.StatsService
	service.ServerService
	service.CloudflareService
	service.FirewallService
	service.PanelSSLService
	service.SubService
}

// GetCfCredentials 返回持久化的 CF token + ACME 邮箱(供前端预填表单)。
// token 是裸值,不再 base64 — 前端拿到就能直接用。
func (a *ApiService) GetCfCredentials(c *gin.Context) {
	token, email := a.SettingService.GetCfToken()
	jsonObj(c, gin.H{
		"token": token,
		"email": email,
		"saved": token != "",
	}, nil)
}

// SetCfCredentials 持久化保存 CF token + ACME 邮箱。
// 用于"自动签发"流程的免重复输入。表单字段:token / email / clear(如非空则清空)。
func (a *ApiService) SetCfCredentials(c *gin.Context) {
	if c.Request.FormValue("clear") != "" {
		jsonMsg(c, "", a.SettingService.ClearCfToken())
		return
	}
	token := c.Request.FormValue("token")
	email := c.Request.FormValue("email")
	if token == "" {
		jsonMsg(c, "", common.NewError("token required"))
		return
	}
	jsonMsg(c, "", a.SettingService.SetCfToken(token, email))
}

// GetFirewallStatus 把 30s 缓存的 ufw / firewalld 探测结果丢给前端。
// 前端在入站列表上跟 inbound.listen_port 做差集,提示哪些端口被防火墙挡了。
func (a *ApiService) GetFirewallStatus(c *gin.Context) {
	jsonObj(c, a.FirewallService.Status(), nil)
}

// GetStatsTotals 按 resource 返回每个 tag 的累计 up/down 字节数。
// 入站/出站列表页"总流量列"靠这个一次拉全,不用每行单点 GetStats。
func (a *ApiService) GetStatsTotals(c *gin.Context) {
	resource := c.Query("resource")
	if resource == "" {
		resource = "inbound"
	}
	totals, err := a.StatsService.GetTotals(resource)
	jsonObj(c, totals, err)
}

// ResetTraffic 清掉某个 inbound/outbound/user 的累计流量样本。
// 表单字段:resource (inbound|outbound|user) + tag。
func (a *ApiService) ResetTraffic(c *gin.Context) {
	resource := c.Request.FormValue("resource")
	tag := c.Request.FormValue("tag")
	if resource == "" || tag == "" {
		jsonMsg(c, "", common.NewError("resource and tag required"))
		return
	}
	err := a.StatsService.ResetByTag(resource, tag)
	jsonMsg(c, "", err)
}

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		case "block-rules":
			blockRules, err := a.ConfigService.BlockRuleService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = blockRules
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.UserService.GetUsers()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, *users, nil)
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}
	data, err := a.StatsService.GetStats(resource, tag, limit)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

// PanelSslIssue 用 Cloudflare DNS-01 给面板域名签 ACME 证书,自动写
// webCertFile/webKeyFile/webDomain settings + 触发面板重启。复用持久化的
// cfCredentials(token + email),前端只需提供域名。
func (a *ApiService) PanelSslIssue(c *gin.Context) {
	domain := c.Request.FormValue("domain")
	if domain == "" {
		domain = c.Query("domain")
	}
	if domain == "" {
		jsonMsg(c, "", common.NewError("缺少 domain 参数"))
		return
	}
	token, email := a.SettingService.GetCfToken()
	if token == "" || email == "" {
		jsonMsg(c, "", common.NewError("先去 TLS 一键签发流程粘贴 CF Token + ACME 邮箱并保存,这里才能复用"))
		return
	}
	if err := a.PanelSSLService.IssueAndApply(&a.SettingService, &a.CloudflareService, domain, email, token); err != nil {
		jsonMsg(c, "", err)
		return
	}
	// 异步重启,先把响应送出去再关 — 否则用户连接 reset 看不到 success
	go func() {
		time.Sleep(2 * time.Second)
		_ = a.PanelService.RestartPanel(1)
	}()
	jsonObj(c, gin.H{
		"domain":   domain,
		"certFile": a.tryGet("webCertFile"),
		"keyFile":  a.tryGet("webKeyFile"),
		"hint":     "签发成功,2 秒后面板会自动重启,请用 https://" + domain + ":<port>" + a.tryGet("webPath") + " 重新登录",
	}, nil)
}

// tryGet 静默拿单个 setting,失败返回空字符串(供 UI 展示用)
func (a *ApiService) tryGet(key string) string {
	if all, err := a.SettingService.GetAllSetting(); err == nil && all != nil {
		if v, ok := (*all)[key]; ok {
			return v
		}
	}
	return ""
}

// GetConnStats 返回 sing-box 当前活跃连接数(TCP / UDP 分别)。给前端面板
// "网络速率"区域 5-10s 拉一次显示并发情况。
func (a *ApiService) GetConnStats(c *gin.Context) {
	tcp, udp := 0, 0
	if box := a.ConfigService.CoreInstance(); box != nil {
		tcp, udp = box.ConnTracker().CountByNetwork()
	}
	jsonObj(c, gin.H{"tcp": tcp, "udp": udp}, nil)
}

// GetOnlineIPs 返回单个 tag(inbound 或 user)当前活跃的 source IP 列表。
// 查询参数:resource (inbound|user) + tag (inbound 的 tag 或 client name)。
// 用于"限制 IP 数"功能取数据,以及客户端管理界面"看现在哪些 IP 在用我账号"。
// user 跨入站汇总:同一账号在 N 个 inbound 用,IP 列表合一去重。
func (a *ApiService) GetOnlineIPs(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	if resource == "" || tag == "" {
		jsonMsg(c, "", common.NewError("resource and tag required"))
		return
	}
	ips := a.StatsService.GetOnlineIPs(resource, tag)
	jsonObj(c, gin.H{"ips": ips, "count": len(ips)}, nil)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("c")
	level := c.Query("l")
	logs := a.ServerService.GetLogs(count, level)
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=s-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

func (a *ApiService) Login(c *gin.Context) {
	remoteIP := getRemoteIp(c)
	loginUser, err := a.UserService.Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}

	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	err = SetLoginUser(c, loginUser, sessionMaxAge)
	if err == nil {
		logger.Info("user ", loginUser, " login success")
	} else {
		logger.Warning("login failed: ", err)
	}

	jsonMsg(c, "", nil)
}

func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.UserService.ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		logger.Info("change user credentials success")
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "restartSb", err)
}

func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

func (a *ApiService) ImportDb(c *gin.Context) {
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	jsonMsg(c, "", err)
}

func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.UserService.LoadTokens()
}

func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.UserService.GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(loginUser, expiryInt, desc)
	jsonObj(c, token, err)
}

func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.UserService.DeleteToken(tokenId)
	jsonMsg(c, "", err)
}

func (a *ApiService) ResetToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	token, err := a.UserService.ResetToken(tokenId)
	jsonObj(c, token, err)
}

// addTokenForUser - v2 入口:用调用方持有的 token 推断出 username,而不是 session。
// 表单字段同 v1 (expiry / desc),返回新 token 字符串。
func (a *ApiService) addTokenForUser(c *gin.Context, username string) {
	if username == "" {
		jsonMsg(c, "", errEmpty("username"))
		return
	}
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(username, expiryInt, desc)
	jsonObj(c, token, err)
}

func (a *ApiService) getTokensForUser(c *gin.Context, username string) {
	if username == "" {
		jsonMsg(c, "", errEmpty("username"))
		return
	}
	tokens, err := a.UserService.GetUserTokens(username)
	jsonObj(c, tokens, err)
}

// UpdateSettingsAPI - v2 入口:与 v1 的 setting CLI / sui setting 等价的写接口。
// 这里只覆盖 panel 端口与路径,够脚本化部署用;前端继续走 save 走 ConfigService.Save 改 settings 表。
func (a *ApiService) UpdateSettingsAPI(c *gin.Context) {
	type req struct {
		Port *int    `json:"port" form:"port"`
		Path *string `json:"path" form:"path"`
	}
	var r req
	_ = c.ShouldBind(&r)

	if r.Port != nil && *r.Port > 0 {
		if err := a.SettingService.SetPort(*r.Port); err != nil {
			jsonMsg(c, "setting", err)
			return
		}
	}
	if r.Path != nil && *r.Path != "" {
		if err := a.SettingService.SetWebPath(*r.Path); err != nil {
			jsonMsg(c, "setting", err)
			return
		}
	}
	jsonMsg(c, "setting", nil)
}

func errEmpty(field string) error {
	type emptyErr struct{ s string }
	return &simpleErr{msg: field + " can not be empty"}
}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

// ---------- Cloudflare ----------

func (a *ApiService) CFListZones(c *gin.Context) {
	token := c.Request.FormValue("token")
	if token == "" {
		token = c.Query("token")
	}
	if err := a.CloudflareService.VerifyToken(token); err != nil {
		jsonMsg(c, "", err)
		return
	}
	zones, err := a.CloudflareService.ListZones(token)
	jsonObj(c, zones, err)
}

func (a *ApiService) CFUpsertA(c *gin.Context) {
	type req struct {
		Token   string `form:"token" json:"token"`
		ZoneId  string `form:"zoneId" json:"zoneId"`
		Name    string `form:"name" json:"name"`
		Random  bool   `form:"random" json:"random"`
		Prefix  string `form:"prefix" json:"prefix"`
		IP      string `form:"ip" json:"ip"`
		Proxied bool   `form:"proxied" json:"proxied"`
	}
	var r req
	if err := c.ShouldBind(&r); err != nil {
		jsonMsg(c, "", err)
		return
	}
	if r.Token == "" || r.ZoneId == "" || r.IP == "" {
		jsonMsg(c, "", errEmpty("token / zoneId / ip"))
		return
	}
	subname := r.Name
	if r.Random {
		subname = a.CloudflareService.RandomSubdomain(r.Prefix)
	}
	fqdn, recId, err := a.CloudflareService.UpsertARecord(r.Token, r.ZoneId, subname, r.IP, r.Proxied)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, gin.H{"fqdn": fqdn, "recordId": recId}, nil)
}

// CFDetectIP 在「服务器」上探测公网 IP,返回给前端写到 A 记录里。
// 前端不能自己调 ipify — 那拿到的是用户浏览器(可能在家、可能在跳板)
// 的出口 IP,签到 DNS 上是错的。
func (a *ApiService) CFDetectIP(c *gin.Context) {
	ip := a.CloudflareService.DetectPublicIP()
	if ip == "" {
		jsonMsg(c, "", errEmpty("public ip"))
		return
	}
	jsonObj(c, gin.H{"ip": ip}, nil)
}

func (a *ApiService) CFIssueTLS(c *gin.Context) {
	type req struct {
		Name    string `form:"name" json:"name"`
		Fqdn    string `form:"fqdn" json:"fqdn"`
		Email   string `form:"email" json:"email"`
		Token   string `form:"token" json:"token"`
		DataDir string `form:"dataDir" json:"dataDir"`
	}
	var r req
	if err := c.ShouldBind(&r); err != nil {
		jsonMsg(c, "", err)
		return
	}
	if r.Fqdn == "" || r.Email == "" || r.Token == "" {
		jsonMsg(c, "", errEmpty("fqdn / email / token"))
		return
	}
	id, err := a.CloudflareService.IssueTLS(r.Name, r.Fqdn, r.Email, r.Token, r.DataDir)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, gin.H{"id": id, "fqdn": r.Fqdn}, nil)
}

func (a *ApiService) GetSingboxConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetConfig("")
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetCheckOutbound(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckOutbound(tag, link)
	jsonObj(c, result, nil)
}
