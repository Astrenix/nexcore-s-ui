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

// ProbeTargets 多个等价 trace 端点(都返 ip= 文本格式),按顺序轮换 + 失败重试。
//
// 为什么不只一个:机场反盗用经常把"探测器常用目标"(单一 cloudflare IP)加风控,
// 也会按 SNI 识别(captive portal probe 的 SNI 集中在 cp.cloudflare.com)。
// 轮换 3 个目标 + 失败重试,有概率绕过特定 SNI / IP 黑名单。
//
// 都用 http://(走 outbound 内的 TLS 而不是再套一层)避免双 TLS 延迟 + 错误源混淆。
var ProbeTargets = []string{
	"http://cp.cloudflare.com/cdn-cgi/trace",   // cloudflare 官方 captive portal anycast
	"http://www.cloudflare.com/cdn-cgi/trace",  // cloudflare 主域 trace,不同 IP 段
	"http://1.1.1.1/cdn-cgi/trace",             // cloudflare 1.1.1.1 直 IP
}

// ProbeResult 探测一条候选节点的结果。
type ProbeResult struct {
	ExitIP    string // trace 返回的 ip= 字段(真落地 IP)
	LatencyMs int    // 端到端 HTTP GET 时延(ms)
	OK        bool
	Error     string // 最后一次失败的原因(用于诊断)
	Target    string // 命中的目标 URL(成功时填,失败时填最后一次试的)
}

// ProbeOutboundByTag 通过已经存在于 outbound_manager 的出站,先用 sing-box urltest
// 测延迟(与「出站管理 → 测试」完全相同的测量方式),再发一次 HTTP GET 拿 exit IP。
//
// 为什么分两次而不是把 HTTP 端到端时间当延迟:
//   - 出站管理「测试」延迟用 urltest.URLTest(裸 TCP+TLS,GET generate_204,无 body)
//   - 如果订阅池用「HTTP GET trace + ReadAll body」的端到端 RTT 当延迟,数字会
//     系统性比出站管理大 30–200ms(读 body + 解析时间),用户看到两边不一致会困惑
//   - 改成 urltest 测延迟 + 单独 HTTP GET 拿 IP,延迟数字跟出站管理 1:1 对齐
//
// urltest 探测目标也跟 outbound_check.go 一致(cp.cloudflare.com/generate_204),
// 确保完全可比。HTTP GET 那一步仍然轮换 ProbeTargets 以应对 SNI 黑名单。
//
// 失败模式:
//   - urltest 失败 → 节点 dead(连不上)
//   - urltest OK 但所有 HTTP GET 都失败 → 节点判 dead(拿不到 exit IP,信息不完整)
//     这种情况罕见但确实可能(单 IP 黑洞 cloudflare 路由,机场拒探测器目标),
//     标 dead 比"alive 但无 IP"更安全
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

	// 第一步:urltest 测延迟(与出站管理「测试」同款)
	//
	// 跑两次:第一次预热(吃掉 TCP+TLS+anytls 握手等冷启动开销 ≈ 5-6 RTT),
	// 第二次复用 keepalive 测稳定 RTT,数字才跟出站管理「测试」对得上。
	// 不预热的话,临时 __probe outbound 第一次拨号要 ≈70ms,而出站管理对常驻
	// pool-hk(已暖)只要 ≈15ms,用户看见 5× 差异会困惑。
	//
	// 预热失败 → 节点 dead(连不上);测量失败但预热成功 → 当作不可探测但保留
	// 预热延迟(可能是 cloudflare 路由抖动),不极端否定。
	utCtx, utCancel := context.WithTimeout(ctx, probeTimeout)
	if _, err := urltest.URLTest(utCtx, defaultProbeURL, ob); err != nil {
		utCancel()
		res.Error = "urltest warmup: " + err.Error()
		return res
	}
	delay, err := urltest.URLTest(utCtx, defaultProbeURL, ob)
	utCancel()
	if err != nil {
		res.Error = "urltest: " + err.Error()
		return res
	}
	res.LatencyMs = int(delay)

	// 第二步:HTTP GET 拿 exit IP(轮换目标对抗 SNI 黑名单 + IP 风控)
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
	for _, target := range ProbeTargets {
		res.Target = target
		pctx, cancel := context.WithTimeout(ctx, probeTimeout)
		req, _ := http.NewRequestWithContext(pctx, "GET", target, nil)
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
		var exitIP string
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "ip=") {
				exitIP = strings.TrimSpace(line[3:])
				break
			}
		}
		if exitIP == "" {
			res.Error = "trace 缺少 ip= 字段"
			continue
		}
		res.ExitIP = exitIP
		res.OK = true
		res.Error = ""
		return res
	}
	// urltest 通了但 HTTP 全失败 — 拿不到 exit IP,判 dead(信息不完整,winner 选举依赖 exit IP 去重)
	return res
}
