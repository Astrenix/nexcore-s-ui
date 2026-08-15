package common

import (
	"net"
	"strings"
)

// ClientIP 从直连地址 + X-Forwarded-For 推导"应当被信任的"客户端 IP。
//
// 关键安全约束:X-Forwarded-For 是客户端可以任意伪造的普通请求头。历史实现
// 无条件取 XFF 首段,导致登录爆破节流(唯一的未鉴权防线)可被"每次请求换一个
// 伪造 IP"完全绕过,同时污染审计日志里的来源 IP。
//
// 这里只在**直连对端本身是回环 / 私网 / link-local 地址**时才采信 XFF —— 也就是
// 面板前面确实挂着同机或内网反向代理(nginx / 宝塔 / Caddy)的场景,此时 XFF 由
// 那台反代写入,可信。若请求直接来自公网,XFF 一律忽略,以 TCP 层的真实对端为准。
//
// remoteAddr 传 http.Request.RemoteAddr("ip:port" 形式),xff 传原始头值。
func ClientIP(remoteAddr string, xff string) string {
	peer := hostFromAddr(remoteAddr)
	if xff == "" || !isTrustedProxyIP(peer) {
		return peer
	}
	// 反代链自右向左追加,最左是原始客户端;取最左段并做基本合法性校验,
	// 校验失败则退回直连对端,不让垃圾值进节流表/审计日志。
	first := strings.TrimSpace(strings.Split(xff, ",")[0])
	if first == "" || net.ParseIP(first) == nil {
		return peer
	}
	return first
}

func hostFromAddr(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// isTrustedProxyIP 回环 / 私网 / link-local 视为"反代位于可信网络内"。
func isTrustedProxyIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
