package v1

import (
	"crypto/subtle"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

// 与 nexcore-x-ui 同款鉴权传输 — Bearer 优先,X-API-Token 兜底。
// query string `?api_token=` 永不接受(进 access log + 浏览器历史 = 泄漏)。
const (
	bearerPrefix = "Bearer "
	headerToken  = "X-API-Token"
)

func ExtractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if strings.HasPrefix(h, bearerPrefix) {
			return strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
		}
	}
	if h := c.GetHeader(headerToken); h != "" {
		return strings.TrimSpace(h)
	}
	// nexcore-s-ui 上游 v2 用 Token: <t> 头 — 兼容,这样老脚本不会因为
	// 切到 v1 端点就失效。
	if h := c.GetHeader("Token"); h != "" {
		return strings.TrimSpace(h)
	}
	return ""
}

// tokenMemo 是 v1 鉴权的内存副本,跟 ApiV2Handler.tokens 等价但独立维护 —
// 让 /api/v1 与 /apiv2 解耦,各自的 reload 节奏独立(避免一个 panic 拖另一个)。
type tokenMemo struct {
	mu     sync.RWMutex
	loaded time.Time
	items  []memoItem
}

type memoItem struct {
	Token    string
	Username string
	Desc     string
	Expiry   int64
}

var memo = &tokenMemo{}

// Reload 拉一次 model.Tokens 全量,丢内存里。鉴权热路径只读,不查 db。
//
// 调用方:web/web.go 启动时 + 每次 admin 加/删 token 后(主动)。
// fail-soft:db 故障时保留前次成功载入的副本。
func Reload() error {
	db := database.GetDB()
	if db == nil {
		return nil
	}
	now := time.Now().Unix()
	var rows []model.Tokens
	err := db.Model(&model.Tokens{}).Preload("User").
		Where("expiry == 0 OR expiry > ?", now).
		Find(&rows).Error
	if err != nil {
		return err
	}
	items := make([]memoItem, 0, len(rows))
	for _, r := range rows {
		username := ""
		if r.User != nil {
			username = r.User.Username
		}
		items = append(items, memoItem{
			Token:    r.Token,
			Username: username,
			Desc:     r.Desc,
			Expiry:   r.Expiry,
		})
	}
	memo.mu.Lock()
	memo.items = items
	memo.loaded = time.Now()
	memo.mu.Unlock()
	return nil
}

// resolveToken constant-time 比对内存中所有 token,命中即返回身份。
//
// 双模式比较:DB 里 token 列存的可能是 sha256 hash(新 token,AUDIT.md C1
// 后)或 legacy 明文(老 token,未重置过的存量)。中间件按列内容判别:
//   - 64 字符 hex → 跟 sha256(raw) 比
//   - 其它 → 直接跟 raw 比(legacy 明文,AddToken/ResetToken 后会逐渐归零)
//
// constant-time 比对保留:防止 token 长度 / 命中时序泄漏(对存量明文 token
// 尤其重要,因为它们没经过 hash 单向化)。
func resolveToken(raw string) (memoItem, bool) {
	if raw == "" {
		return memoItem{}, false
	}
	rawSha := service.HashTokenForCompare(raw)
	memo.mu.RLock()
	defer memo.mu.RUnlock()
	now := time.Now().Unix()
	for _, it := range memo.items {
		if it.Expiry > 0 && it.Expiry < now {
			continue
		}
		// 64 字符全 hex 视为 sha256 hash;长度不是 64 / 含非 hex 视为 legacy 明文
		var compare string
		if isSha256Hex(it.Token) {
			compare = rawSha
		} else {
			compare = raw
		}
		if subtle.ConstantTimeCompare([]byte(it.Token), []byte(compare)) == 1 {
			return it, true
		}
	}
	return memoItem{}, false
}

// isSha256Hex 64-char 全小写 hex 才认为是 sha256 输出。跟 service.looksLikeTokenHash
// 重复一份是为了避免 v1 中间件对 service 包做"读私有判断"的脏路径(中间件
// 已经 import service),但保持本地一份判别让模式比较的边界对中间件可见。
func isSha256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// AuthMiddleware 失败:401 + missing_api_token / invalid_api_token,与 x-ui
// 同 code,主控不用区分对端。
//
// 命中后注入 ctx:
//   api_token        = raw value
//   api_token_user   = username (s-ui 的"创建该 token 的人")
//   api_token_desc   = token 备注
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := ExtractToken(c)
		if raw == "" {
			Unauthorized(c, "missing_api_token")
			return
		}
		it, ok := resolveToken(raw)
		if !ok {
			Unauthorized(c, "invalid_api_token")
			return
		}
		c.Set("api_token", raw)
		c.Set("api_token_user", it.Username)
		c.Set("api_token_desc", it.Desc)
		c.Next()
	}
}

// AccessLogMiddleware 把 /api/v1/* 的每次调用落进 model.ApiLog,与 /apiv2
// 共用同一张表。便于审计统一。
func AccessLogMiddleware(logSvc *service.ApiLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		entry := &model.ApiLog{
			DateTime:  started.Unix(),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			LatencyMs: time.Since(started).Milliseconds(),
			RemoteIp:  remoteIP(c),
			Username:  c.GetString("api_token_user"),
			TokenDesc: c.GetString("api_token_desc"),
		}
		if len(c.Errors) > 0 {
			entry.Err = c.Errors[0].Error()
		}
		logSvc.Add(entry)
	}
}

// remoteIP 与 api.getRemoteIp 同语义:XFF 仅在直连对端为可信反代时采信,
// 否则以 TCP 层真实对端为准(防伪造头污染审计日志)。
func remoteIP(c *gin.Context) string {
	return common.ClientIP(c.Request.RemoteAddr, c.GetHeader("X-Forwarded-For"))
}
