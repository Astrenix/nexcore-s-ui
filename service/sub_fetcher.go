package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alireza0/s-ui/logger"
)

const (
	subFetchTimeout = 30 * time.Second
	subUserAgent    = "v2rayN/6.42" // 多数机场看 UA:clash → YAML,v2rayN/sing-box → base64 URI list(我们要后者)
	subMaxBodyBytes = 2 << 20       // 2 MB 上限,防止误下整张大文件
	subMaxNodes     = 1000          // 单订阅节点数上限:2MB body 可展开上万条 URI,
	//                                 每条都要探测(最多两次 8s)+ 全程持 subOpsMu,
	//                                 不设限单次刷新可跑数十分钟并锁死整个订阅子系统
)

// FetchSub 拉取订阅 URL,自动识别格式并解析为 ParsedNode 列表。
//
// 支持的格式:
//  1. base64 编码的纯文本(机场标准)— 整体 base64,decode 后逐行 URI
//  2. 纯文本 URI 列表 — 直接逐行解析
//  3. Clash YAML — 暂不支持(返回错误,提示用户)
//  4. sing-box JSON — 暂不支持(返回错误,提示用户)
//
// 解析容错:
//   - 单条 URI 失败跳过,不让整次 fetch 全废
//   - 返回 (nodes, ok parse count, total candidates, error)
func FetchSub(url string) ([]ParsedNode, int, int, error) {
	body, err := httpGetText(url)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("fetch: %w", err)
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, 0, 0, errors.New("empty body")
	}

	// 格式识别(简单优先级)
	if strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[") {
		return nil, 0, 0, errors.New("sing-box JSON 格式订阅暂未支持,请选择 base64 / URI list 格式")
	}
	if strings.HasPrefix(body, "proxies:") || strings.HasPrefix(body, "port:") {
		return nil, 0, 0, errors.New("Clash YAML 格式订阅暂未支持,请选择 base64 / URI list 格式")
	}

	// 看起来已经是裸 URI 文本(行首有 anytls/vless/...://)
	if looksLikeURIList(body) {
		return parseURILines(body)
	}

	// 尝试 base64 decode
	decoded, err := b64decode(body)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("unrecognized format (not base64, not URI list): %v", err)
	}
	return parseURILines(string(decoded))
}

// looksLikeURIList:首行/前几行有典型 URI scheme 前缀就当裸 list。
func looksLikeURIList(s string) bool {
	head := s
	if len(head) > 200 {
		head = head[:200]
	}
	prefixes := []string{"anytls://", "vless://", "vmess://", "trojan://", "ss://", "hysteria2://", "hy2://", "tuic://"}
	for _, p := range prefixes {
		if strings.Contains(head, p) {
			return true
		}
	}
	return false
}

// parseURILines:按行(支持 \n / \r\n)解析,空行跳过。
func parseURILines(text string) ([]ParsedNode, int, int, error) {
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })
	var nodes []ParsedNode
	total := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		total++
		n, err := ParseLink(line)
		if err != nil {
			continue // 单条失败不让全废
		}
		nodes = append(nodes, *n)
		if len(nodes) >= subMaxNodes {
			// 超上限就截断并告警,不静默丢——避免"以为全导入了实际只导了一部分"
			logger.Warningf("subscription truncated at %d nodes (limit reached)", subMaxNodes)
			break
		}
	}
	if len(nodes) == 0 {
		return nil, 0, total, fmt.Errorf("0 valid links parsed from %d candidates", total)
	}
	return nodes, len(nodes), total, nil
}

// httpGetText:HTTP GET 拉文本,follow redirect,带 UA + 短超时。
//
// 注意:这台开发机走 SOCKS/HTTP 代理出网,Go 默认 net/http 会读 HTTP_PROXY env
// 自动走代理,这正是我们想要的(订阅服务器在国外,本机能直连就直连,被墙就代理)。
func httpGetText(url string) (string, error) {
	// scheme 白名单 + 内网/回环/metadata 地址拦截(SSRF 防护),详见 ssrf_guard.go
	if _, err := validateFetchURL(url); err != nil {
		return "", err
	}
	client := newSSRFSafeClient(subFetchTimeout)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", subUserAgent)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	lim := io.LimitReader(resp.Body, subMaxBodyBytes+1)
	b, err := io.ReadAll(lim)
	if err != nil {
		return "", err
	}
	if len(b) > subMaxBodyBytes {
		return "", fmt.Errorf("body too large (>%d bytes)", subMaxBodyBytes)
	}
	return string(b), nil
}
