package core

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	urltest "github.com/sagernet/sing-box/common/urltest"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const probeTimeout = 8 * time.Second

// LatencyProbeURLs:用于 urltest 测延迟的多个 204 端点。任一通即可。
//
// 跨供应商:cloudflare / google / 华为 / 苹果。这样某家被特定 ISP 屏蔽 / 风控
// 仍有兜底。同时各家用不同 SNI/IP,机场端按 SNI 黑名单的也能绕过去。
//
// 必须返 204 No Content(urltest 默认期望),所以这些 URL 经过精挑细选 — 它们
// 都是真实在用的 OS 级 captive portal 检测端点,响应稳定。
var LatencyProbeURLs = []string{
	"http://cp.cloudflare.com/generate_204",                        // cloudflare 官方 anycast(全球通用)
	"http://www.gstatic.com/generate_204",                          // google Android 默认(在美/欧/拉美 IDC 稳)
	"http://connectivitycheck.platform.hicloud.com/generate_204",   // 华为(中国出口友好,但海外也可达)
	"http://captive.apple.com/hotspot-detect.html",                 // 苹果(SNI captive.apple.com 跟前几个完全不重)
}

// ExitIPProbeTargets 多个 IP 回显端点 + 解析方式。任一通即可拿到落地 IP。
//
// 跨域名 / 跨 AS / 跨格式:覆盖不同的 SNI 黑名单 + IP 信誉风控场景。
//   - trace 格式:cloudflare cdn-cgi/trace,返 `ip=X.X.X.X` 多行 key=value
//   - plain 格式:返单行纯 IP 文本
//   - json 格式:返 {"ip":"X.X.X.X", ...}
type exitIPProbe struct {
	URL  string
	Kind string // "trace" | "plain" | "json"
}

var ExitIPProbeTargets = []exitIPProbe{
	{"http://cp.cloudflare.com/cdn-cgi/trace", "trace"}, // cloudflare anycast
	{"http://1.1.1.1/cdn-cgi/trace", "trace"},           // cloudflare 1.1.1.1 直 IP(不同 SNI)
	{"http://api.ipify.org/", "plain"},                  // ipify(AWS hosted,完全独立 AS)
	{"http://ifconfig.me/ip", "plain"},                  // ifconfig.me(独立运营)
	{"http://icanhazip.com/", "plain"},                  // cloudflare 旗下但 hostname 不同(SNI 不集中)
	{"http://api.myip.com/", "json"},                    // myip.com(JSON 格式,异构 fallback)
}

// ProbeResult 探测一条候选节点的结果。
type ProbeResult struct {
	ExitIP    string // 落地 IP
	LatencyMs int    // urltest 测出的延迟(ms)
	OK        bool
	Error     string // 最后一次失败的原因(用于诊断)
	Target    string // 命中的目标 URL(成功时填,失败时填最后一次试的)
}

// ProbeOutboundByTag 通过已经存在于 outbound_manager 的出站测延迟 + 拿落地 IP。
//
// 多目标策略:
//   1. 延迟阶段:轮换 LatencyProbeURLs(cloudflare / google / huawei / apple),
//      第一个 urltest 成功的目标决定整组测延迟用哪个,避免某家被特定 IP 段封锁
//      整批挂(用户反馈"非得盯一个站点儿,机场可能就对这个网站不通")
//   2. 暖连接:同一目标再跑一次 urltest 拿稳定 RTT(冷启动 ≈ 5-6 RTT,暖后 ≈ 1 RTT)
//   3. 出口 IP:轮换 ExitIPProbeTargets(cloudflare trace / ipify / ifconfig.me /
//      icanhazip / api.myip.com),格式不一,parser 各自处理。任一通即拿到 IP
//
// 失败定义:LatencyProbeURLs 全跑过都失败 → 节点 dead;延迟通了但 exit IP 全失败
// → 仍判 dead(winner 选举依赖 exit IP 去重)
func ProbeOutboundByTag(ctx context.Context, tag string) ProbeResult {
	res := ProbeResult{}
	if outbound_manager == nil {
		res.Error = "core not running"
		return res
	}
	ob, ok := outbound_manager.Outbound(tag)
	if !ok {
		res.Error = "outbound not found: " + tag
		return res
	}

	// 第一步:延迟 — 轮换多个 204 目标找第一个通的。
	//
	// 优化:dial 层失败(anytls 握手 EOF、TCP timeout、connection refused 等)
	// 立刻判 dead,不浪费时间换 URL 重试 — 换不同 URL 也会用同一条死掉的拨号路径,
	// 没意义。只对 URL 级别失败(HTTP 4xx/5xx、cloudflare 风控特定 SNI)才轮换。
	var hitLatencyURL string
	var lastLatErr error
	for i, url := range LatencyProbeURLs {
		c, cancel := context.WithTimeout(ctx, probeTimeout)
		_, err := urltest.URLTest(c, url, ob)
		cancel()
		if err == nil {
			hitLatencyURL = url
			break
		}
		lastLatErr = err
		// dial 层错误 — 后面 URL 还是会撞同样的拨号失败,直接 fast-fail
		if isDialError(err) && i == 0 {
			res.Error = "urltest: " + err.Error()
			return res
		}
	}
	if hitLatencyURL == "" {
		res.Error = "urltest 全部失败: " + lastLatErr.Error()
		return res
	}
	// 暖连接已建立,再跑一次测稳定 RTT(跟出站管理「测试」的数字 1:1 对齐)
	c, cancel := context.WithTimeout(ctx, probeTimeout)
	delay, err := urltest.URLTest(c, hitLatencyURL, ob)
	cancel()
	if err != nil {
		res.Error = "urltest measurement: " + err.Error()
		return res
	}
	res.LatencyMs = int(delay)

	// 第二步:轮换 IP echo 端点拿 exit IP(不同 SNI / 不同 AS / 不同响应格式)
	dialer := N.Dialer(ob)
	client := &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			ResponseHeaderTimeout: probeTimeout,
		},
	}
	for _, t := range ExitIPProbeTargets {
		res.Target = t.URL
		pctx, cancel := context.WithTimeout(ctx, probeTimeout)
		req, _ := http.NewRequestWithContext(pctx, "GET", t.URL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (probe)")
		resp, err := client.Do(req)
		if err != nil {
			res.Error = err.Error()
			cancel()
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			res.Error = fmt.Sprintf("http %d", resp.StatusCode)
			cancel()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		if err != nil {
			res.Error = err.Error()
			continue
		}
		exitIP := parseExitIP(string(body), t.Kind)
		if exitIP == "" {
			res.Error = "无法从 " + t.Kind + " 响应解析 IP"
			continue
		}
		res.ExitIP = exitIP
		res.OK = true
		res.Error = ""
		return res
	}
	// 延迟通了但 IP echo 全失败 — 罕见,节点判 dead(winner 选举依赖 exit IP 去重)
	return res
}

// parseExitIP 按响应格式从 body 提取 IP。
func parseExitIP(body, kind string) string {
	body = strings.TrimSpace(body)
	switch kind {
	case "trace":
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "ip=") {
				return strings.TrimSpace(line[3:])
			}
		}
	case "plain":
		// 单行 IP 文本,直接返(可能带换行已 trim)
		if isPlainIP(body) {
			return body
		}
	case "json":
		// 简单字符串扫 "ip":"X.X.X.X" — 不引依赖,只覆盖标准格式
		idx := strings.Index(body, `"ip"`)
		if idx < 0 {
			return ""
		}
		rest := body[idx+4:]
		colon := strings.Index(rest, `"`)
		if colon < 0 {
			return ""
		}
		quote := strings.Index(rest[colon+1:], `"`)
		if quote < 0 {
			return ""
		}
		val := rest[colon+1 : colon+1+quote]
		if isPlainIP(val) {
			return val
		}
	}
	return ""
}

// isDialError 判断错误是否属于"拨号层"(TCP/TLS/anytls 握手失败),不是 HTTP/URL 层。
// 这类错误换 URL 也救不回(同一拨号通道是死的),直接 fast-fail 避免重试浪费时间。
func isDialError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, k := range []string{
		"failed to create session",       // anytls / vless 会话建立失败
		"dial tcp",                       // Go net 拨号失败前缀
		"i/o timeout",                    // TCP 层超时
		"connection refused",             // 对端拒接
		"no route to host",               // 路由层不可达
		"network is unreachable",         // 网络不可达
		"EOF",                            // 握手中对端关连
		"connection reset by peer",       // RST
		"use of closed network connection", // dialer/transport 关闭
		"tls: handshake failure",         // TLS 握手失败
		"unexpected EOF",
	} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// isPlainIP 弱校验:形如 IPv4 / IPv6 文本(不深度解析,够区分"IP" vs "HTML 错误页")
func isPlainIP(s string) bool {
	if len(s) < 3 || len(s) > 45 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || c == '.' || c == ':' ||
			(c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return strings.Contains(s, ".") || strings.Contains(s, ":")
}
