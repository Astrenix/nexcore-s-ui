package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParsedNode 一条 URI 链接解析后的结果。
//
// 设计要点:
//   - Type / Server / ServerPort 是顶层冗余字段,便于 DB 查询去重 + 跨协议统一
//   - Options 是完整的 sing-box outbound JSON(去掉 type 和 tag),包含 server / server_port、
//     password / uuid / tls / transport 等所有字段;winner 选出后直接拷贝到 outbound.options
//   - Remark 是 URL-decoded 的 # 后字段(节点名称),给前端显示 + tag 命名
type ParsedNode struct {
	Remark     string
	Type       string
	Server     string
	ServerPort uint16
	Options    json.RawMessage
}

var errEmptyHostPort = errors.New("empty host:port")

// ParseLink 解析单条 URI 链接为 ParsedNode。
// 支持:anytls / vless / vmess / trojan / shadowsocks (ss) / hysteria2 / tuic。
// 返回 nil, error 表示协议不支持或格式非法。
func ParseLink(raw string) (*ParsedNode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty link")
	}

	switch {
	case strings.HasPrefix(raw, "anytls://"):
		return parseAnytls(raw)
	case strings.HasPrefix(raw, "vless://"):
		return parseVless(raw)
	case strings.HasPrefix(raw, "vmess://"):
		return parseVmess(raw)
	case strings.HasPrefix(raw, "trojan://"):
		return parseTrojan(raw)
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(raw)
	case strings.HasPrefix(raw, "hysteria2://"), strings.HasPrefix(raw, "hy2://"):
		return parseHysteria2(raw)
	case strings.HasPrefix(raw, "tuic://"):
		return parseTuic(raw)
	default:
		idx := strings.Index(raw, "://")
		if idx > 0 {
			return nil, fmt.Errorf("unsupported scheme: %s", raw[:idx])
		}
		return nil, fmt.Errorf("not a uri: %q", raw[:min(40, len(raw))])
	}
}

// ---------------- helpers ----------------

// splitHostPort 解析 host:port,容忍 IPv6 [::1]:443 格式。
func splitHostPort(s string) (string, uint16, error) {
	if s == "" {
		return "", 0, errEmptyHostPort
	}
	// IPv6 形式 [::1]:443
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 || end+1 >= len(s) || s[end+1] != ':' {
			return "", 0, fmt.Errorf("bad ipv6 host:port: %q", s)
		}
		port, err := strconv.ParseUint(s[end+2:], 10, 16)
		if err != nil {
			return "", 0, fmt.Errorf("bad port: %v", err)
		}
		return s[1:end], uint16(port), nil
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, fmt.Errorf("missing port: %q", s)
	}
	port, err := strconv.ParseUint(s[i+1:], 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("bad port: %v", err)
	}
	return s[:i], uint16(port), nil
}

// decodeFragment 解 URL 编码的 # 部分(节点名)。
func decodeFragment(frag string) string {
	if frag == "" {
		return ""
	}
	if s, err := url.QueryUnescape(frag); err == nil {
		return s
	}
	return frag
}

// b64decode 容忍 url-safe + 无 padding。
func b64decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, " ", "")
	// 优先 URL-safe(机场常用),失败回退 std
	for _, enc := range []*base64.Encoding{
		base64.URLEncoding,
		base64.RawURLEncoding,
		base64.StdEncoding,
		base64.RawStdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("base64 decode failed for %q", s[:min(40, len(s))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildTLS 公用 TLS 字段构造:
// 大多数协议(anytls / vless / trojan / hysteria2 / tuic)的 query 参数命名一致:
//   - security=tls 或 sni 出现 → 启 TLS
//   - sni:server_name(空 fallback host)
//   - insecure=1 / allowInsecure=1:跳证书校验
//   - fp:utls fingerprint(chrome / firefox / safari …)
//   - alpn:逗号分隔(h2,http/1.1)
func buildTLS(q url.Values, defaultHost string) map[string]any {
	sec := q.Get("security")
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer") // 一些老 trojan 用 peer
	}
	hasTLS := sec == "tls" || sec == "reality" || sni != "" || q.Get("tls") == "1"
	if !hasTLS {
		return nil
	}
	tls := map[string]any{"enabled": true}
	if sni != "" {
		tls["server_name"] = sni
	} else if defaultHost != "" {
		tls["server_name"] = defaultHost
	}
	if q.Get("insecure") == "1" || strings.EqualFold(q.Get("allowInsecure"), "true") || q.Get("allowInsecure") == "1" {
		tls["insecure"] = true
	}
	if fp := q.Get("fp"); fp != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	if alpn := q.Get("alpn"); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}
	// Reality
	if sec == "reality" {
		reality := map[string]any{"enabled": true}
		if pbk := q.Get("pbk"); pbk != "" {
			reality["public_key"] = pbk
		}
		if sid := q.Get("sid"); sid != "" {
			reality["short_id"] = sid
		}
		tls["reality"] = reality
	}
	return tls
}

// buildTransport vless/vmess/trojan 等的 transport 字段(type=ws/grpc/httpupgrade/http)。
func buildTransport(q url.Values) map[string]any {
	t := q.Get("type")
	if t == "" || t == "tcp" || t == "raw" {
		return nil
	}
	switch t {
	case "ws":
		tr := map[string]any{"type": "ws"}
		if p := q.Get("path"); p != "" {
			tr["path"] = p
		}
		if h := q.Get("host"); h != "" {
			tr["headers"] = map[string]any{"Host": h}
		}
		return tr
	case "grpc":
		tr := map[string]any{"type": "grpc"}
		if svc := q.Get("serviceName"); svc != "" {
			tr["service_name"] = svc
		}
		return tr
	case "httpupgrade":
		tr := map[string]any{"type": "httpupgrade"}
		if p := q.Get("path"); p != "" {
			tr["path"] = p
		}
		if h := q.Get("host"); h != "" {
			tr["host"] = h
		}
		return tr
	case "http", "h2":
		tr := map[string]any{"type": "http"}
		if p := q.Get("path"); p != "" {
			tr["path"] = []string{p}
		}
		if h := q.Get("host"); h != "" {
			tr["host"] = []string{h}
		}
		return tr
	}
	return nil
}

// finalize 把通用字段塞进 options map 并 marshal。
func finalize(typ, server string, port uint16, password, uuid string,
	tls map[string]any, transport map[string]any, extra map[string]any) (json.RawMessage, error) {
	opts := map[string]any{
		"server":      server,
		"server_port": port,
	}
	if password != "" {
		opts["password"] = password
	}
	if uuid != "" {
		opts["uuid"] = uuid
	}
	if tls != nil {
		opts["tls"] = tls
	}
	if transport != nil {
		opts["transport"] = transport
	}
	for k, v := range extra {
		opts[k] = v
	}
	return json.Marshal(opts)
}

// ---------------- protocols ----------------

// anytls://uuid@host:port/?type=tcp&insecure=1&fp=chrome&sni=...#remark
// anytls 在 sing-box outbound 里:type=anytls,password 字段(不是 uuid)
func parseAnytls(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	password := u.User.Username()
	q := u.Query()
	tls := buildTLS(q, host)
	if tls == nil {
		tls = map[string]any{"enabled": true} // anytls 默认 TLS
		if host != "" {
			tls["server_name"] = host
		}
	}
	opts, err := finalize("anytls", host, port, password, "", tls, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: decodeFragment(u.Fragment), Type: "anytls",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// vless://uuid@host:port?encryption=none&security=tls&sni=...&type=ws&path=/ws&flow=...&fp=chrome#remark
func parseVless(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	q := u.Query()
	tls := buildTLS(q, host)
	transport := buildTransport(q)
	extra := map[string]any{}
	if flow := q.Get("flow"); flow != "" {
		extra["flow"] = flow
	}
	opts, err := finalize("vless", host, port, "", uuid, tls, transport, extra)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: decodeFragment(u.Fragment), Type: "vless",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// vmess://BASE64({"v":"2","ps":"...","add":"host","port":"443","id":"uuid","aid":"0","net":"ws","type":"none","host":"","path":"/","tls":"tls","sni":"...","alpn":"...","fp":"chrome"})
func parseVmess(raw string) (*ParsedNode, error) {
	body := strings.TrimPrefix(raw, "vmess://")
	// vmess 还可能是 SIP002 风格(很少),先优先尝试 base64
	jsonBytes, err := b64decode(body)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return nil, fmt.Errorf("vmess json: %v", err)
	}
	host, _ := m["add"].(string)
	var port uint16
	switch p := m["port"].(type) {
	case float64:
		port = uint16(p)
	case string:
		v, _ := strconv.ParseUint(p, 10, 16)
		port = uint16(v)
	}
	uuid, _ := m["id"].(string)
	if host == "" || port == 0 || uuid == "" {
		return nil, errors.New("vmess: missing add/port/id")
	}
	// security:默认 auto;原 link 字段叫 scy(部分客户端)
	security, _ := m["scy"].(string)
	if security == "" {
		security = "auto"
	}
	q := url.Values{}
	if sni, _ := m["sni"].(string); sni != "" {
		q.Set("sni", sni)
	}
	if tlsField, _ := m["tls"].(string); tlsField == "tls" {
		q.Set("security", "tls")
	}
	if fp, _ := m["fp"].(string); fp != "" {
		q.Set("fp", fp)
	}
	if alpn, _ := m["alpn"].(string); alpn != "" {
		q.Set("alpn", alpn)
	}
	if h, _ := m["host"].(string); h != "" {
		q.Set("host", h)
	}
	if p, _ := m["path"].(string); p != "" {
		q.Set("path", p)
	}
	net, _ := m["net"].(string)
	if net != "" {
		// vmess "net" → 统一映射到 transport type
		switch net {
		case "ws", "grpc", "httpupgrade", "http":
			q.Set("type", net)
		}
	}
	tls := buildTLS(q, host)
	transport := buildTransport(q)

	// alterId(老版兼容)
	var alterId int
	switch v := m["aid"].(type) {
	case float64:
		alterId = int(v)
	case string:
		alterId, _ = strconv.Atoi(v)
	}
	extra := map[string]any{
		"security":  security,
		"alter_id":  alterId,
	}
	opts, err := finalize("vmess", host, port, "", uuid, tls, transport, extra)
	if err != nil {
		return nil, err
	}
	remark, _ := m["ps"].(string)
	return &ParsedNode{
		Remark: remark, Type: "vmess",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// trojan://password@host:port?security=tls&sni=...&type=tcp#remark
func parseTrojan(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	password := u.User.Username()
	q := u.Query()
	tls := buildTLS(q, host)
	if tls == nil {
		tls = map[string]any{"enabled": true, "server_name": host} // trojan 默认 TLS
	}
	transport := buildTransport(q)
	opts, err := finalize("trojan", host, port, password, "", tls, transport, nil)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: decodeFragment(u.Fragment), Type: "trojan",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// shadowsocks 三种格式:
//  1. ss://BASE64(method:password)@host:port#remark      (SIP002)
//  2. ss://BASE64(method:password@host:port)#remark      (legacy)
//  3. ss://method:password@host:port#remark              (明文,少见)
func parseShadowsocks(raw string) (*ParsedNode, error) {
	body := strings.TrimPrefix(raw, "ss://")
	// 拆 fragment(remark)
	var remark string
	if i := strings.Index(body, "#"); i >= 0 {
		remark = decodeFragment(body[i+1:])
		body = body[:i]
	}
	// 优先 SIP002:形式 userinfo@host:port,userinfo 是 base64(method:password)
	var method, password, host string
	var port uint16
	if i := strings.Index(body, "@"); i >= 0 {
		userinfo := body[:i]
		hostpart := body[i+1:]
		// userinfo 可能是 base64(method:password) 或明文 method:password
		decoded, err := b64decode(userinfo)
		var mp string
		if err == nil {
			mp = string(decoded)
		} else {
			mp = userinfo
		}
		if j := strings.Index(mp, ":"); j > 0 {
			method = mp[:j]
			password = mp[j+1:]
		}
		// host:port 部分可能带 query(?plugin=...),先剥
		if k := strings.Index(hostpart, "?"); k >= 0 {
			hostpart = hostpart[:k]
		}
		host, port, err = splitHostPort(hostpart)
		if err != nil {
			return nil, err
		}
	} else {
		// legacy:整体 base64(method:password@host:port)
		decoded, err := b64decode(body)
		if err != nil {
			return nil, err
		}
		s := string(decoded)
		i := strings.Index(s, "@")
		if i < 0 {
			return nil, errors.New("ss legacy: missing @")
		}
		mp := s[:i]
		hostpart := s[i+1:]
		if j := strings.Index(mp, ":"); j > 0 {
			method = mp[:j]
			password = mp[j+1:]
		}
		host, port, err = splitHostPort(hostpart)
		if err != nil {
			return nil, err
		}
	}
	if method == "" || host == "" {
		return nil, errors.New("ss: missing method or host")
	}
	extra := map[string]any{"method": method}
	opts, err := finalize("shadowsocks", host, port, password, "", nil, nil, extra)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: remark, Type: "shadowsocks",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// hysteria2://password@host:port?insecure=1&sni=...&obfs=salamander&obfs-password=...#remark
func parseHysteria2(raw string) (*ParsedNode, error) {
	raw = strings.Replace(raw, "hy2://", "hysteria2://", 1)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	password := u.User.Username()
	q := u.Query()
	tls := buildTLS(q, host)
	if tls == nil {
		tls = map[string]any{"enabled": true, "server_name": host}
	}
	extra := map[string]any{}
	if obfs := q.Get("obfs"); obfs != "" {
		obfsMap := map[string]any{"type": obfs}
		if pw := q.Get("obfs-password"); pw != "" {
			obfsMap["password"] = pw
		}
		extra["obfs"] = obfsMap
	}
	opts, err := finalize("hysteria2", host, port, password, "", tls, nil, extra)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: decodeFragment(u.Fragment), Type: "hysteria2",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}

// tuic://uuid:password@host:port?sni=...&alpn=h3&congestion_control=bbr#remark
func parseTuic(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host, port, err := splitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	uuid := u.User.Username()
	password, _ := u.User.Password()
	q := u.Query()
	tls := buildTLS(q, host)
	if tls == nil {
		tls = map[string]any{"enabled": true, "server_name": host}
	}
	extra := map[string]any{}
	if cc := q.Get("congestion_control"); cc != "" {
		extra["congestion_control"] = cc
	}
	if udp := q.Get("udp_relay_mode"); udp != "" {
		extra["udp_relay_mode"] = udp
	}
	opts, err := finalize("tuic", host, port, password, uuid, tls, nil, extra)
	if err != nil {
		return nil, err
	}
	return &ParsedNode{
		Remark: decodeFragment(u.Fragment), Type: "tuic",
		Server: host, ServerPort: port, Options: opts,
	}, nil
}
