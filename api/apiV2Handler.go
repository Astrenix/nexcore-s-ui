package api

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	apiv1 "github.com/alireza0/s-ui/api/v1"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type TokenInMemory struct {
	Token    string
	Expiry   int64
	Username string
	Desc     string
}

type APIv2Handler struct {
	ApiService
	tokensMu sync.RWMutex
	tokens   *[]TokenInMemory
	logSvc   service.ApiLogService
}

func NewAPIv2Handler(g *gin.RouterGroup) *APIv2Handler {
	a := &APIv2Handler{}
	a.ReloadTokens()
	a.initRouter(g)
	return a
}

func (a *APIv2Handler) initRouter(g *gin.RouterGroup) {
	// 鉴权 + 调用记录:成功失败都记一条,这是 API 审计的基线
	g.Use(func(c *gin.Context) {
		started := time.Now()
		token := c.GetHeader("Token")
		if token == "" {
			// 同时接受 Authorization: Bearer <token> 形式 - 这是 HTTP 标准
			// 写法,大量 SDK / curl 用户的肌肉记忆都在这。
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
				c.Request.Header.Set("Token", token)
			}
		}

		username, desc := a.identifyToken(token)
		if username == "" {
			a.logSvc.Add(&model.ApiLog{
				DateTime: started.Unix(),
				Method:   c.Request.Method,
				Path:     c.Request.URL.Path,
				Status:   401,
				RemoteIp: getRemoteIp(c),
				Err:      "invalid or expired token",
			})
			jsonMsg(c, "", common.NewError("invalid token"))
			c.Abort()
			return
		}
		c.Set("apiv2_username", username)
		c.Set("apiv2_token_desc", desc)

		c.Next()

		// 记录调用结束 - body 不存,占空间;只存 status / latency / error。
		entry := &model.ApiLog{
			DateTime:  started.Unix(),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			LatencyMs: time.Since(started).Milliseconds(),
			RemoteIp:  getRemoteIp(c),
			Username:  username,
			TokenDesc: desc,
		}
		// 把 c.Errors 第一条作为 err 文本(jsonMsg 失败时未必走 c.Error,
		// 这里 best-effort)
		if len(c.Errors) > 0 {
			entry.Err = c.Errors[0].Error()
		}
		a.logSvc.Add(entry)
	})
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIv2Handler) postHandler(c *gin.Context) {
	username := a.findUsername(c)
	action := c.Param("postAction")

	switch action {
	case "save":
		a.ApiService.Save(c, username)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	// v1 等价补齐 - 让外部 SDK 用 Token 也能做面板上能做的所有事。
	case "changePass":
		a.ApiService.ChangePass(c)
	case "addToken":
		a.ApiService.addTokenForUser(c, username)
		a.ReloadTokens()
		_ = apiv1.Reload()
	case "deleteToken":
		a.ApiService.DeleteToken(c)
		a.ReloadTokens()
		_ = apiv1.Reload()
	case "resetToken":
		a.ApiService.ResetToken(c)
		a.ReloadTokens()
		_ = apiv1.Reload()
	case "setting":
		a.ApiService.UpdateSettingsAPI(c)
	case "clearApiLogs":
		err := a.logSvc.Clear()
		jsonMsg(c, "clearApiLogs", err)
	case "cfListZones":
		a.ApiService.CFListZones(c)
	case "cfUpsertA":
		a.ApiService.CFUpsertA(c)
	case "cfIssueTls":
		a.ApiService.CFIssueTLS(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "tls", "clients", "config", "block-rules":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	case "tokens":
		a.ApiService.getTokensForUser(c, a.findUsername(c))
	case "singbox-config":
		a.ApiService.GetSingboxConfig(c)
	case "apiLogs":
		a.handleApiLogs(c)
	case "me":
		// 让调用方用一次轻量请求确认 token 有效 + 当前身份
		jsonObj(c, gin.H{
			"username":  c.GetString("apiv2_username"),
			"tokenDesc": c.GetString("apiv2_token_desc"),
		}, nil)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIv2Handler) handleApiLogs(c *gin.Context) {
	method := c.Query("method")
	path := c.Query("path")
	username := c.Query("username")
	var since, until int64
	json.Unmarshal([]byte(c.DefaultQuery("since", "0")), &since)
	json.Unmarshal([]byte(c.DefaultQuery("until", "0")), &until)
	var limit, offset int
	json.Unmarshal([]byte(c.DefaultQuery("limit", "200")), &limit)
	json.Unmarshal([]byte(c.DefaultQuery("offset", "0")), &offset)

	logs, total, err := a.logSvc.List(method, path, username, since, until, limit, offset)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, gin.H{"logs": logs, "total": total}, nil)
}

// findUsername 兼容老调用 - 从 ctx 缓存读;回退到 token 表(若中间件没注入)。
func (a *APIv2Handler) findUsername(c *gin.Context) string {
	if u := c.GetString("apiv2_username"); u != "" {
		return u
	}
	u, _ := a.identifyToken(c.GetHeader("Token"))
	return u
}

// identifyToken 把 raw token 解析成 (username, desc),命中失败返回 ("","")。
// 不阻塞 — 只读内存副本,DB 调整由 ReloadTokens 异步刷。
//
// 比对走 service.MatchToken(sha256/legacy 双模式 + constant-time),跟 api/v1
// 中间件同一套语义。历史上这里是裸 `t.Token == token`,而内存表来自把 token
// 掩码成 "****" 的 LoadTokens —— 合法 token 全部失配,且发 "****" 即可命中,
// 是完整的认证绕过。现在数据源改为 LoadTokensForAuth(真实值)。
func (a *APIv2Handler) identifyToken(token string) (string, string) {
	if token == "" {
		return "", ""
	}
	a.tokensMu.RLock()
	defer a.tokensMu.RUnlock()
	if a.tokens == nil {
		return "", ""
	}
	now := time.Now().Unix()
	for _, t := range *a.tokens {
		if t.Expiry > 0 && t.Expiry < now {
			continue
		}
		if service.MatchToken(t.Token, token) {
			return t.Username, t.Desc
		}
	}
	return "", ""
}

func (a *APIv2Handler) ReloadTokens() {
	tokens, err := a.ApiService.LoadTokensForAuth()
	if err != nil {
		logger.Error("unable to load tokens: ", err)
		return
	}
	newTokens := make([]TokenInMemory, 0, len(tokens))
	for _, t := range tokens {
		username := ""
		if t.User != nil {
			username = t.User.Username
		}
		newTokens = append(newTokens, TokenInMemory{
			Token:    t.Token,
			Expiry:   t.Expiry,
			Username: username,
			Desc:     t.Desc,
		})
	}
	a.tokensMu.Lock()
	a.tokens = &newTokens
	a.tokensMu.Unlock()
}
