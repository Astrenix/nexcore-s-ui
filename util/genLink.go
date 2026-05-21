package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

type LinkParam struct {
	Key   string
	Value string
}

// LinkGenerator 生成客户端分享链接。addrSource 决定 add 字段来源(三态):
//
//	"panel"(默认 / 空)  用 hostname(panel webDomain / Host)
//	"ip"                 用 panelIp(管理员手填的源 IP);空时 fallback hostname
//	"tls"                用 inbound.tls.server_name(通配符 / 空时 fallback hostname)
//
// 入站可在 inbound.LinkAddrSource 单独覆盖此全局策略 — 调用方负责把
// `inbound.LinkAddrSource OR settings.linkAddrSource OR "panel"` 计算好后
// 传入 addrSource 参数。本函数不知道全局 settings,只看入参。
//
// remark 命名规则(vmess.ps / 其它协议 #fragment):
//   - 主前缀 remarkPrefix 由调用方算出 — 直连模式 = settings.nodeName,
//     中转模式 = outbound.display_name(回退 outbound.tag)。空则 fallback inbound.Tag。
//   - clientName 不空时附加 "-<clientName>",方便用户在客户端区分账号。
//   - 同一 inbound 配多 Addrs 时,附加 addrRemark suffix 区分各 server。
func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string, addrSource string, panelIp string, clientName string, remarkPrefix string) []string {
	linkAddrSource := addrSource
	inbound, err := i.MarshalFull()
	if err != nil {
		return []string{}
	}

	baseRemark := strings.TrimSpace(remarkPrefix)
	if baseRemark == "" {
		baseRemark = i.Tag
	}
	if cn := strings.TrimSpace(clientName); cn != "" {
		baseRemark = baseRemark + "-" + cn
	}

	var tls map[string]interface{}
	if i.TlsId > 0 {
		tls = prepareTls(i.Tls)
		// 通配符 TLS UX 修复:用户报"导入软件黄色警告"的根因是 server_name
		// 字段空或本身是通配符 *.x → vmess.sni / vless?sni= 都会传通配符给客户端,
		// 客户端 TLS 校验证书 SAN 时要求精确 host(*.x.example 不是合法 SNI),
		// 报"hostname mismatch"。
		// 这里从 acme.domain 取通配符,用 inbound.tag 替换 *,生成合法子域名。
		// 例:acme.domain=["*.lp.nicevpn.org"] + tag="vless-15414" → "vless-15414.lp.nicevpn.org"。
		// 链接 server 字段(实际 dial host)不动,客户端按 IP / panel host 直连;
		// TLS 握手时 sni 用替换后的子域名,通配符证书匹配 ✅。
		// 用户无需改 DNS(链接 server 不依赖子域名解析)。
		if tls != nil {
			if sn, _ := tls["server_name"].(string); sn == "" || strings.HasPrefix(sn, "*.") {
				if dyn := wildcardSniFromAcme(i.Tls.Server, i.Tag); dyn != "" {
					tls["server_name"] = dyn
				}
			}
		}
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		return []string{}
	}

	var Addrs []map[string]interface{}
	if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
		return []string{}
	}
	if len(Addrs) == 0 {
		// server 字段是客户端实际 dial 的目标 — 必须 DNS 解析或直连到本机公网。
		// 来源由 (inbound.LinkAddrSource OR settings.linkAddrSource) 控制:
		//   "panel"(默认) → hostname(panel webDomain / Host),DNS 必通
		//   "ip"           → panelIp(管理员手填的源 IP),raw TCP 协议绕开 CDN 时用
		//   "tls"          → inbound.tls.server_name(通配符 / 空时 fallback hostname)
		// 缺省 / 兜底一律 fallback hostname,保证 link 总有合法 server 字段。
		serverField := hostname
		switch linkAddrSource {
		case "ip":
			if strings.TrimSpace(panelIp) != "" {
				serverField = strings.TrimSpace(panelIp)
			}
		case "tls":
			if i.TlsId > 0 {
				if sn, ok := tls["server_name"].(string); ok && sn != "" && !strings.HasPrefix(sn, "*.") {
					serverField = sn
				}
			}
		}
		Addrs = append(Addrs, map[string]interface{}{
			"server":      serverField,
			"server_port": (*inbound)["listen_port"],
			"remark":      baseRemark,
		})
		if i.TlsId > 0 {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string)
			Addrs[index]["remark"] = baseRemark + addrRemark
			if i.TlsId > 0 {
				newTls := map[string]interface{}{}
				for k, v := range tls {
					newTls[k] = v
				}

				// Override tls
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					for k, v := range addrTls {
						newTls[k] = v
					}
				}
				Addrs[index]["tls"] = newTls
			}
		}
	}

	switch i.Type {
	case "socks":
		return socksLink(userConfig["socks"], Addrs)
	case "http":
		return httpLink(userConfig["http"], Addrs)
	case "mixed":
		return append(
			socksLink(userConfig["socks"], Addrs),
			httpLink(userConfig["http"], Addrs)...,
		)
	case "shadowsocks":
		return shadowsocksLink(userConfig, *inbound, Addrs)
	case "naive":
		return naiveLink(userConfig["naive"], *inbound, Addrs)
	case "hysteria":
		return hysteriaLink(userConfig["hysteria"], *inbound, Addrs)
	case "hysteria2":
		return hysteria2Link(userConfig["hysteria2"], *inbound, Addrs)
	case "tuic":
		return tuicLink(userConfig["tuic"], *inbound, Addrs)
	case "vless":
		return vlessLink(userConfig["vless"], *inbound, Addrs)
	case "anytls":
		return anytlsLink(userConfig["anytls"], Addrs)
	case "trojan":
		return trojanLink(userConfig["trojan"], *inbound, Addrs)
	case "vmess":
		return vmessLink(userConfig["vmess"], *inbound, Addrs)
	}

	return []string{}
}

// wildcardSniFromAcme 从 TLS 的 server.acme.domain 找通配符域名,把 *
// 替换为 inbound.tag(经 DNS label 安全清洗)生成合法 SNI 子域名。
//
//	acme.domain = ["*.lp.nicevpn.org"] + tag = "vless-15414"
//	→ 返回 "vless-15414.lp.nicevpn.org"
//
// 多个域名时取第一个含 * 的;没找到通配符或解析失败返回 ""。
// 调用方:LinkGenerator 仅在 server_name 缺失/通配符时调用,正常 SNI 路径不动。
func wildcardSniFromAcme(serverRaw json.RawMessage, inboundTag string) string {
	if len(serverRaw) == 0 {
		return ""
	}
	var srv map[string]interface{}
	if err := json.Unmarshal(serverRaw, &srv); err != nil {
		return ""
	}
	acme, ok := srv["acme"].(map[string]interface{})
	if !ok {
		return ""
	}
	domains, _ := acme["domain"].([]interface{})
	for _, d := range domains {
		s, ok := d.(string)
		if !ok || !strings.HasPrefix(s, "*.") {
			continue
		}
		sub := sanitizeDNSLabel(inboundTag)
		if sub == "" {
			sub = "node"
		}
		return sub + s[1:] // *.lp.nicevpn.org → <sub>.lp.nicevpn.org
	}
	return ""
}

// sanitizeDNSLabel 把任意字符串转为合法 DNS label:仅保留 a-z/0-9/-,
// 首尾的 - 修剪,长度截断 ≤ 63(RFC 1035)。inbound.tag 本身基本就是
// 这种格式,这里只是防御性处理(用户可能自定义 tag 含中文/特殊字符)。
func sanitizeDNSLabel(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteByte('-')
		}
	}
	res := strings.Trim(b.String(), "-")
	if len(res) > 63 {
		res = res[:63]
	}
	return res
}

func prepareTls(t *model.Tls) map[string]interface{} {
	var iTls, oTls map[string]interface{}
	if err := json.Unmarshal(t.Client, &oTls); err != nil {
		return nil
	}
	if err := json.Unmarshal(t.Server, &iTls); err != nil {
		return nil
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			// 双向 ok 检查 — 老 DB / 异常配置可能让 reality 不是 map,client 端
			// 也未必有 reality 键(用户从其它 TLS 模式切过来)。两边任一非 map
			// 直接跳过 reality 注入,保留原有 oTls。
			reality, ok1 := v.(map[string]interface{})
			clientReality, ok2 := oTls["reality"].(map[string]interface{})
			if !ok1 {
				continue
			}
			if !ok2 {
				clientReality = map[string]interface{}{}
			}
			clientReality["enabled"] = reality["enabled"]
			if shortIDs, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(shortIDs) > 0 {
				clientReality["short_id"] = shortIDs[common.RandomInt(len(shortIDs))]
			}
			oTls["reality"] = clientReality
		}
	}
	return oTls
}

// asAddr 一次性把 addr map 里的高频字段(server / server_port / remark)取成
// 字符串/数字,所有 link 子函数共用 — 防止单条入站 Addrs 字段格式异常时(JSON
// 解析后类型对不上)整个 LinkGenerator 崩进 gin recover 报 500。
// 任何字段缺失 / 类型错 → 返回 ok=false,调用方 skip 该条 addr。
func asAddr(addr map[string]interface{}) (server string, port float64, remark string, ok bool) {
	server, ok = addr["server"].(string)
	if !ok {
		return
	}
	port, ok = addr["server_port"].(float64)
	if !ok {
		return
	}
	remark, _ = addr["remark"].(string) // remark 缺失给空字符串,不算硬错
	return
}

func socksLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	user, _ := userConfig["username"].(string)
	pass, _ := userConfig["password"].(string)
	auth := url.QueryEscape(user) + ":" + url.QueryEscape(pass)
	var links []string
	for _, addr := range addrs {
		server, port, _, ok := asAddr(addr)
		if !ok {
			continue
		}
		links = append(links, fmt.Sprintf("socks5://%s@%s:%d", auth, server, uint(port)))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, addrs []map[string]interface{}) []string {
	user, _ := userConfig["username"].(string)
	pass, _ := userConfig["password"].(string)
	auth := url.QueryEscape(user) + ":" + url.QueryEscape(pass)
	var links []string
	for _, addr := range addrs {
		server, port, _, ok := asAddr(addr)
		if !ok {
			continue
		}
		protocol := "http"
		if addr["tls"] != nil {
			protocol = "https"
		}
		links = append(links, fmt.Sprintf("%s://%s@%s:%d", protocol, auth, server, uint(port)))
	}
	return links
}

func shadowsocksLink(
	userConfig map[string]map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var userPass []string
	method, _ := inbound["method"].(string)
	if strings.HasPrefix(method, "2022") {
		inbPass, _ := inbound["password"].(string)
		userPass = append(userPass, inbPass)
	}
	var pass string
	if method == "2022-blake3-aes-128-gcm" {
		pass, _ = userConfig["shadowsocks16"]["password"].(string)
	} else {
		pass, _ = userConfig["shadowsocks"]["password"].(string)
	}
	userPass = append(userPass, pass)

	uriBase := fmt.Sprintf("ss://%s", toBase64([]byte(fmt.Sprintf("%s:%s", method, strings.Join(userPass, ":")))))

	var links []string
	for _, addr := range addrs {
		server, port, remark, ok := asAddr(addr)
		if !ok {
			continue
		}
		links = append(links, fmt.Sprintf("%s@%s:%.0f#%s", uriBase, server, port, remark))
	}
	return links
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	username, _ := userConfig["username"].(string)

	baseUri := "http2://"
	var links []string

	for _, addr := range addrs {
		server, port, remark, ok := asAddr(addr)
		if !ok {
			continue
		}
		var params []LinkParam
		params = append(params, LinkParam{"padding", "1"})
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if sni, ok := tls["server_name"].(string); ok {
				params = append(params, LinkParam{"peer", sni})
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				params = append(params, LinkParam{"alpn", strings.Join(stringSlice(alpn), ",")})
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				params = append(params, LinkParam{"insecure", "1"})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"tfo", "1"})
		} else {
			params = append(params, LinkParam{"tfo", "0"})
		}

		uri := baseUri + toBase64([]byte(fmt.Sprintf("%s:%s@%s:%.0f", username, password, server, port)))
		links = append(links, addParams(uri, params, remark))
	}
	return links
}

// stringSlice 把 []interface{} 转 []string,非 string 元素直接跳过 — 老配置
// 把 alpn 写成 mixed type 数组([]interface{ "h2", null })时不会 panic。
func stringSlice(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	baseUri := "hysteria://"
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params = append(params, LinkParam{"auth", auth})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params = append(params, LinkParam{"obfs", obfs})
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		// out_json 缺失/类型错就跳过 mport 注入,不让坏数据让整个 link 生成失败
		if outRaw, ok := inbound["out_json"].(json.RawMessage); ok && len(outRaw) > 0 {
			var outJson map[string]interface{}
			if json.Unmarshal(outRaw, &outJson) == nil {
				if mport, ok := outJson["server_ports"].([]interface{}); ok {
					params = append(params, LinkParam{"mport", strings.Join(stringSlice(mport), ",")})
				}
			}
		}

		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, port)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "hysteria2://", password)
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		var params []LinkParam
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params = append(params, LinkParam{"downmbps", fmt.Sprintf("%.0f", upmbps)})
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params = append(params, LinkParam{"upmbps", fmt.Sprintf("%.0f", downmbps)})
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok {
			if obfsType, ok := obfs["type"].(string); ok {
				params = append(params, LinkParam{"obfs", obfsType})
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params = append(params, LinkParam{"obfs-password", obfsPassword})
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params = append(params, LinkParam{"fastopen", "1"})
		} else {
			params = append(params, LinkParam{"fastopen", "0"})
		}
		if outRaw, ok := inbound["out_json"].(json.RawMessage); ok && len(outRaw) > 0 {
			var outJson map[string]interface{}
			if json.Unmarshal(outRaw, &outJson) == nil {
				if mport, ok := outJson["server_ports"].([]interface{}); ok {
					params = append(params, LinkParam{"mport", strings.Join(stringSlice(mport), ",")})
				}
			}
		}

		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, port)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	baseUri := fmt.Sprintf("%s%s@", "anytls://", password)
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}

		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, port)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, _ := userConfig["password"].(string)
	uuid, _ := userConfig["uuid"].(string)
	baseUri := fmt.Sprintf("%s%s:%s@", "tuic://", uuid, password)
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		var params []LinkParam
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			getTlsParams(&params, tls, "insecure")
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params = append(params, LinkParam{"congestion_control", congestionControl})
		}

		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, port)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		// 双 ok:tls map 取 + enabled bool 取,任一失败视为没启 TLS
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if enabled, _ := tls["enabled"].(bool); enabled {
				getTlsParams(&params, tls, "allowInsecure")
				if flow, ok := userConfig["flow"].(string); ok {
					params = append(params, LinkParam{"flow", flow})
				}
			}
		}
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, server, port)
		uri = addParams(uri, params, remark)
		links = append(links, uri)
	}

	return links
}

func trojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {
	password, _ := userConfig["password"].(string)
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		server, port, remark, okA := asAddr(addr)
		if !okA {
			continue
		}
		params := make([]LinkParam, len(baseParams))
		copy(params, baseParams)
		if tls, ok := addr["tls"].(map[string]interface{}); ok {
			if enabled, _ := tls["enabled"].(bool); enabled {
				getTlsParams(&params, tls, "allowInsecure")
			}
		}
		uri := fmt.Sprintf("trojan://%s@%s:%.0f", password, server, port)
		uri = addParams(uri, params, remark)
		links = append(links, uri)
	}

	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, _ := userConfig["uuid"].(string)
	transportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   "2",
		"id":  uuid,
		"aid": 0,
	}

	var net, typ, host, path string
	for _, p := range transportParams {
		switch p.Key {
		case "type":
			net = p.Value
		case "host":
			host = p.Value
		case "path":
			path = p.Value
		}
	}

	if net == "http" || net == "tcp" {
		baseParams["net"] = "tcp"
		if net == "http" {
			typ = "http"
		}
	} else {
		baseParams["net"] = net
	}

	for _, addr := range addrs {
		obj := make(map[string]interface{})
		for k, v := range baseParams {
			obj[k] = v
		}

		obj["add"], _ = addr["server"].(string)
		port, _ := addr["server_port"].(float64)
		obj["port"] = fmt.Sprintf("%.0f", port)
		obj["ps"], _ = addr["remark"].(string)
		if typ != "" {
			obj["type"] = typ
		}
		if host != "" {
			obj["host"] = host
		}
		if path != "" {
			obj["path"] = path
		}
		populateVmessTlsParams(obj, addr["tls"])

		jsonStr, _ := json.Marshal(obj)

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func populateVmessTlsParams(obj map[string]interface{}, tlsConfig interface{}) {
	tlsMap, ok := tlsConfig.(map[string]interface{})
	enabled, _ := tlsMap["enabled"].(bool) // 缺失/类型错 → false,走 else 分支
	if ok && enabled {
		obj["tls"] = "tls"
		var tlsParams []LinkParam
		getTlsParams(&tlsParams, tlsMap, "allowInsecure")
		for _, p := range tlsParams {
			switch p.Key {
			case "security":
				// ignore, as "tls" is already set
			case "allowInsecure":
				obj["allowInsecure"] = 1
			case "sni":
				obj["sni"] = p.Value
			case "fp":
				obj["fp"] = p.Value
			case "alpn":
				obj["alpn"] = p.Value
			}
		}
	} else {
		obj["tls"] = "none"
	}
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d)
}

func addParams(uri string, params []LinkParam, remark string) string {
	URL, _ := url.Parse(uri)
	var q []string
	for _, p := range params {
		switch p.Key {
		case "mport", "alpn":
			q = append(q, fmt.Sprintf("%s=%s", p.Key, p.Value))
		default:
			q = append(q, fmt.Sprintf("%s=%s", p.Key, url.QueryEscape(p.Value)))
		}
	}
	URL.RawQuery = strings.Join(q, "&")
	URL.Fragment = remark
	return URL.String()
}

func getTransportParams(t interface{}) []LinkParam {
	var params []LinkParam
	trasport, _ := t.(map[string]interface{})
	var transportType string
	if tt, ok := trasport["type"].(string); ok {
		transportType = tt
	} else {
		transportType = "tcp"
	}
	params = append(params, LinkParam{"type", transportType})
	if transportType == "tcp" {
		return params
	}

	switch transportType {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			params = append(params, LinkParam{"host", strings.Join(stringSlice(host), ",")})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params = append(params, LinkParam{"host", host})
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params = append(params, LinkParam{"serviceName", serviceName})
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params = append(params, LinkParam{"host", host})
		}
		if path, ok := trasport["path"].(string); ok {
			params = append(params, LinkParam{"path", path})
		}
	}
	return params
}

func getTlsParams(params *[]LinkParam, tls map[string]interface{}, insecureKey string) {
	reality, hasReality := tls["reality"].(map[string]interface{})
	realityEnabled, _ := reality["enabled"].(bool) // 缺失/非 bool → false,走 else
	if hasReality && realityEnabled {
		*params = append(*params, LinkParam{"security", "reality"})
		if pbk, ok := reality["public_key"].(string); ok {
			*params = append(*params, LinkParam{"pbk", pbk})
		}
		if sid, ok := reality["short_id"].(string); ok {
			*params = append(*params, LinkParam{"sid", sid})
		}
	} else {
		*params = append(*params, LinkParam{"security", "tls"})
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			*params = append(*params, LinkParam{insecureKey, "1"})
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			*params = append(*params, LinkParam{"disable_sni", "1"})
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok {
		if fingerprint, ok := utls["fingerprint"].(string); ok {
			*params = append(*params, LinkParam{"fp", fingerprint})
		}
	}
	if sni, ok := tls["server_name"].(string); ok {
		*params = append(*params, LinkParam{"sni", sni})
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		*params = append(*params, LinkParam{"alpn", strings.Join(stringSlice(alpn), ",")})
	}
}
