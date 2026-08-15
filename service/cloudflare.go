package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

const cfAPIBase = "https://api.cloudflare.com/client/v4"

type CloudflareService struct{}

type CFZone struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type cfResp struct {
	Success bool              `json:"success"`
	Errors  []json.RawMessage `json:"errors"`
	Result  json.RawMessage   `json:"result"`
}

// sanitizeToken 把用户从 CF Dashboard 复制粘贴时常见的脏字符洗掉,避免
// HTTP header 带换行 / 引号触发 CF 6111 "Invalid format for Authorization header"。
// 涵盖:首尾空白(含换行)、首尾的英文/中文引号、整体被 `Bearer ` 前缀包裹的情况。
func sanitizeToken(token string) string {
	t := strings.TrimSpace(token)
	// 用户有时直接复制 CF 文档里的 `Bearer XXXXX` 整段
	t = strings.TrimPrefix(t, "Bearer ")
	t = strings.TrimPrefix(t, "bearer ")
	// 去掉成对引号
	t = strings.Trim(t, `"'` + "“”‘’")
	return strings.TrimSpace(t)
}

// httpJSON 不是 best-of-class 客户端,但够用 - CF API 老老实实的 JSON,无 cursor/pagination 复杂度。
// token 可以是 Global API Key + email(老式)或 API Token(推荐),这里只支持 Bearer Token。
func (s *CloudflareService) httpJSON(method, path, token string, body interface{}) (*cfResp, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, cfAPIBase+path, bodyReader)
	if err != nil {
		return nil, err
	}
	clean := sanitizeToken(token)
	if clean == "" {
		return nil, common.NewError("cloudflare token is empty after sanitize — 检查粘贴是否完整")
	}
	req.Header.Set("Authorization", "Bearer "+clean)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var r cfResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, common.NewError("cloudflare API non-JSON (HTTP ", res.StatusCode, "): ", string(raw))
	}
	if !r.Success {
		// 把常见的 CF 错误码翻译成用户能看懂的中文,原始 JSON 收口到 debug 文本里。
		// 6003/6111: Authorization header 格式问题(典型:粘贴带空格/引号)
		// 9109 / 9106: 无权访问该 zone(token 未授予该 zone 权限)
		// 7003: 路径不存在 / zone id 不存在
		// 1001: token 已禁用 / 已撤销
		errStr := string(raw)
		if strings.Contains(errStr, `"code":6003`) || strings.Contains(errStr, `"code":6111`) {
			return &r, common.NewError("Cloudflare 拒绝认证(6003/6111):API Token 格式不对。请重新到 https://dash.cloudflare.com/profile/api-tokens 复制完整 Token,粘贴时不要带空格 / 引号 / 换行")
		}
		if strings.Contains(errStr, `"code":1001`) || strings.Contains(errStr, `"code":1000`) {
			return &r, common.NewError("Cloudflare API Token 无效或已撤销(1000/1001),请去 dashboard 重新生成")
		}
		if strings.Contains(errStr, `"code":9109`) || strings.Contains(errStr, `"code":9106`) {
			return &r, common.NewError("Cloudflare API Token 没有此 zone 的权限(9106/9109),请在 token 编辑页加上 Zone:Read + Zone:DNS:Edit + Zone Resources 选中目标域名")
		}
		return &r, common.NewError("cloudflare API failed: ", errStr)
	}
	return &r, nil
}

// VerifyToken 检查 token 是否合法。CF 的 /user/tokens/verify 是 token 自检入口。
func (s *CloudflareService) VerifyToken(token string) error {
	if strings.TrimSpace(token) == "" {
		return common.NewError("empty cloudflare token")
	}
	_, err := s.httpJSON("GET", "/user/tokens/verify", token, nil)
	return err
}

// ListZones 列出 token 可见的 zone。Global API Token 一般可见全部 zone,
// 普通 scoped token 受 token 权限限制 — 这是用户应当感知的边界。
func (s *CloudflareService) ListZones(token string) ([]CFZone, error) {
	r, err := s.httpJSON("GET", "/zones?per_page=50", token, nil)
	if err != nil {
		return nil, err
	}
	var zones []CFZone
	if err := json.Unmarshal(r.Result, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}

type cfDnsRecord struct {
	Id      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// UpsertARecord 在 zone 下保证存在一条 A 记录指向 ip;若已有同名 A 记录,
// 改为目标 ip(避免重复创建)。返回最终 fqdn 与 record id。
func (s *CloudflareService) UpsertARecord(token, zoneId, name, ip string, proxied bool) (string, string, error) {
	zoneName, err := s.zoneName(token, zoneId)
	if err != nil {
		return "", "", err
	}
	fqdn := name
	if name == "" || name == "@" {
		fqdn = zoneName
		name = "@"
	} else if !strings.HasSuffix(name, zoneName) {
		fqdn = name + "." + zoneName
	}

	listURL := fmt.Sprintf("/zones/%s/dns_records?type=A&name=%s", zoneId, fqdn)
	listResp, err := s.httpJSON("GET", listURL, token, nil)
	if err != nil {
		return "", "", err
	}
	var existing []cfDnsRecord
	if err := json.Unmarshal(listResp.Result, &existing); err != nil {
		return "", "", err
	}

	rec := cfDnsRecord{
		Type:    "A",
		Name:    fqdn,
		Content: ip,
		TTL:     1, // automatic
		Proxied: proxied,
	}

	if len(existing) > 0 {
		recId := existing[0].Id
		updURL := fmt.Sprintf("/zones/%s/dns_records/%s", zoneId, recId)
		_, err := s.httpJSON("PUT", updURL, token, rec)
		if err != nil {
			return "", "", err
		}
		return fqdn, recId, nil
	}
	createURL := fmt.Sprintf("/zones/%s/dns_records", zoneId)
	cresp, err := s.httpJSON("POST", createURL, token, rec)
	if err != nil {
		return "", "", err
	}
	var created cfDnsRecord
	if err := json.Unmarshal(cresp.Result, &created); err != nil {
		return "", "", err
	}
	return fqdn, created.Id, nil
}

func (s *CloudflareService) zoneName(token, zoneId string) (string, error) {
	r, err := s.httpJSON("GET", "/zones/"+zoneId, token, nil)
	if err != nil {
		return "", err
	}
	var z CFZone
	if err := json.Unmarshal(r.Result, &z); err != nil {
		return "", err
	}
	return z.Name, nil
}

// DetectPublicIP 在「服务器」上向多个 echo-IP 服务并发查询,返回第一个成功的。
// 必须在后端跑 — 前端 fetch 拿到的是用户浏览器的出口 IP,不是面板服务器的
// 公网 IP,签到 DNS 上是错的。
func (s *CloudflareService) DetectPublicIP() string {
	apis := []string{
		"https://api64.ipify.org",
		"https://ip.sb",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
		"https://checkip.amazonaws.com",
	}
	type result struct {
		ip  string
		err error
	}
	ch := make(chan result, len(apis))
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 3 * time.Second}

	for _, api := range apis {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			resp, err := client.Get(url)
			if err != nil {
				ch <- result{"", err}
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				ch <- result{"", err}
				return
			}
			ch <- result{string(body), nil}
		}(api)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err == nil && res.ip != "" {
			return strings.TrimSpace(res.ip)
		}
	}
	return ""
}

// RandomSubdomain 给一个 8 字符 hex 前缀;给运营人员"我懒得想前缀"的快捷出口。
func (s *CloudflareService) RandomSubdomain(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	tail := hex.EncodeToString(b)
	if prefix == "" {
		return "n-" + tail
	}
	return strings.TrimSpace(prefix) + "-" + tail
}

// IssueTLS 不真签证书 — sing-box 自己内置 ACME(with_acme build tag),
// 只要把 acme 块写入 model.Tls.Server,sing-box 启动时会用 dns01_challenge
// 走 Cloudflare 取证书。这里我们只负责落库一条 model.Tls 记录,把 cf
// 的 api_token 嵌进去。
//
// 入参:
//   - name:    TLS 记录在面板里的名字
//   - fqdn:    要签证书的域名(已通过 UpsertARecord 解析到本机 IP)
//   - email:   ACME 注册邮箱
//   - cfToken: dns01 用的 Cloudflare API Token
//   - dataDir: ACME 缓存目录(每个 TLS 一个,免冲突)
func (s *CloudflareService) IssueTLS(name, fqdn, email, cfToken, dataDir string) (uint, error) {
	isWildcard := strings.HasPrefix(fqdn, "*.")
	apex := fqdn
	if isWildcard {
		apex = fqdn[2:] // *.jp1.nicevpn.org → jp1.nicevpn.org
	}

	if dataDir == "" {
		// 通配符域名(*.x.example.com)落到 ./acme/_wildcard.x.example.com,
		// 文件系统不接受 '*' 字符
		safe := strings.ReplaceAll(fqdn, "*", "_wildcard")
		dataDir = "./acme/" + safe
	}

	// ACME 端配置,机场场景默认值:
	//   - 私钥类型走 sing-box / certmagic 默认 P256(ECDSA),跟旧版 key_type=ec256
	//     等价;sing-box 1.13.5+ 移除了 key_type 字段,显式传会 unknown field
	//   - disable_http_challenge / disable_tls_alpn_challenge:已有 DNS-01,
	//     堵住兜底端口挑战,避免占 80/443 跟入站冲突
	acme := map[string]interface{}{
		"domain":                     []string{fqdn},
		"data_directory":             dataDir,
		"default_server_name":        apex, // 通配符模式落到 apex,DNS-01 实际用不到
		"email":                      email,
		"provider":                   "letsencrypt",
		"disable_http_challenge":     true,
		"disable_tls_alpn_challenge": true,
		"dns01_challenge": map[string]interface{}{
			"provider":  "cloudflare",
			"api_token": cfToken,
		},
	}

	server := map[string]interface{}{
		"enabled":     true,
		"alpn":        []string{"h2", "http/1.1"},
		"min_version": "1.3", // 抗主动探测/降版本指纹,2026 没有客户端只能 1.2
		"acme":        acme,
	}
	// 通配符模式不能给 server.server_name 塞 "*.xxx" 字面量(sing-box 当 SNI 字符串
	// 比对会失败)。留空 → sing-box 接受任意 SNI,用 ClientHello SNI 去 SAN 选证书。
	// 这正是机场场景"一证多入站不同 SNI"的核心。
	if !isWildcard {
		server["server_name"] = fqdn
	}

	// 客户端默认指纹用 chrome:JA3/JA4 流量分布最大,墙单独识别成本最高。
	// 用户在编辑器里仍可手动改。通配符模式 client.server_name 留空,
	// 由每个入站的分享链接生成时填具体子 SNI。
	clientCfg := map[string]interface{}{
		"enabled": true,
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}
	if !isWildcard {
		clientCfg["server_name"] = fqdn
	}

	srvBytes, err := json.Marshal(server)
	if err != nil {
		return 0, err
	}
	cliBytes, err := json.Marshal(clientCfg)
	if err != nil {
		return 0, err
	}

	if name == "" {
		name = fqdn
	}
	tls := model.Tls{
		Name:   name,
		Server: srvBytes,
		Client: cliBytes,
	}

	// 必须用事务包 Create + Changes + LastUpdate,跟 ConfigService.Save 一致。
	// 否则 api/load 的增量(CheckChanges 看 model.Changes)拿不到这条新 TLS,
	// 前端 store 里 tlsConfigs 不会刷新,入站编辑页 TLS 下拉就是空的。
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Create(&tls).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	dt := time.Now().Unix()
	chgPayload, _ := json.Marshal(map[string]interface{}{
		"id":   tls.Id,
		"name": tls.Name,
		"src":  "cloudflare-wizard",
	})
	if err := tx.Create(&model.Changes{
		DateTime: dt,
		Actor:    "cf-wizard",
		Key:      "tls",
		Action:   "new",
		Obj:      chgPayload,
	}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	LastUpdate.Store(dt)
	return tls.Id, nil
}
