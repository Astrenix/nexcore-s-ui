package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SSRF 防护:订阅 URL 由用户提交后由**服务端**发起请求,若不限制目标,攻击者
// (或被社工诱导的管理员)可让面板去打内网服务 / 云 metadata(169.254.169.254)
// / 回环端口,把面板变成内网探测与数据回读的跳板。
//
// 这里做三件事:
//  1. scheme 只允许 http/https;
//  2. 建立连接时(DialContext)对**实际解析出的每一个 IP**做内网/回环/link-local
//     校验 —— 在 Dial 层拦截可同时覆盖"域名解析到内网"和 DNS rebinding;
//  3. 跟随重定向时对每一跳的 URL 重新校验 scheme(目标 IP 仍由 Dial 层兜底)。

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 169.254.0.0/16 link-local(含云 metadata 169.254.169.254)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// validateFetchURL 校验 scheme 并返回规范化的 URL。
func validateFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q (only http/https allowed)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url has no host")
	}
	return u, nil
}

// ssrfSafeDialContext 包装一个 Dialer,在真正拨号前对解析出的目标 IP 做黑名单校验。
func ssrfSafeDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// 解析全部 A/AAAA,任一落在内网即拒绝(挡 DNS rebinding:这里解析到的
		// IP 就是随后 dialer 连接用的地址,二者一致,无 TOCTOU 窗口)。
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no address for host %q", host)
		}
		for _, ipAddr := range ips {
			if isBlockedIP(ipAddr.IP) {
				return nil, fmt.Errorf("blocked SSRF target %s (internal address)", ipAddr.IP)
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

// probeTargetBlocked 校验订阅节点的拨号目标(server 可能是 IP 或域名)是否指向
// 内网/回环/metadata。订阅探测会把每个节点挂成 outbound 并拨号,若节点 server
// 来自恶意机场订阅指向受害面板内网(如 10.0.0.5:6379),就能借 alive/last_error
// 差异对内网做端口探测。域名先解析,任一解析结果落内网即拒绝。
// 解析失败视为不可达(阻断),让这类节点直接判 dead。
func probeTargetBlocked(server string) bool {
	if server == "" {
		return true
	}
	if ip := net.ParseIP(server); ip != nil {
		return isBlockedIP(ip)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, server)
	if err != nil || len(ips) == 0 {
		return true
	}
	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr.IP) {
			return true
		}
	}
	return false
}

// newSSRFSafeClient 构造一个仅允许访问公网 http/https 的 client。
func newSSRFSafeClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext:           ssrfSafeDialContext(timeout),
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			// 每一跳都重新校验 scheme;目标 IP 由 DialContext 兜底。
			if _, err := validateFetchURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}
