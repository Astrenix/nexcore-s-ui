package service

// 节点交付预设(Provision Presets)
//
// 本文件把「一条可售入站长什么样」的领域知识从前端表单(frontend/src/layouts/modals/Inbound.vue,
// 1100+ 行)下沉到节点后端,让主控只需要声明意图 —— "给我开一条 vless-reality" ——
// 而不必自己拼 sing-box 配置。
//
// 设计约束:
//   1. **纯函数**:preset 只负责"根据 spec 造出 TLS/inbound 的 JSON",不碰数据库、
//      不调 Cloudflare。落库与编排由 provision.go 负责。这样 preset 可以单测。
//   2. **两段式产物**:reality / acme 类入站需要先建一条 model.Tls 记录拿到 id,
//      再把 id 填进 inbound.tls_id。所以 Build 返回 (tlsSpec, inboundJSON-builder)。
//   3. **凭据不在这里生成**:vless/vmess/trojan 的 UUID、mixed 的用户名密码,
//      在 s-ui 里统一走 clients 表(见 database/model/inbounds.go 对 users 字段的注释),
//      由主控按订阅 fan-out 创建。preset 只负责入站本身。
//      空 users 时 InboundService.fetchUsers 会塞哨兵账号,端口监听但无人能登 —— 安全。

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// PresetKind 交付预设标识。主控蓝图里存的就是这个字符串。
type PresetKind string

const (
	// PresetVlessReality VLESS + XTLS-Vision + Reality。
	// 不需要域名、不需要证书 —— 抛弃式节点 / 无域名场景的主力。
	PresetVlessReality PresetKind = "vless-reality"

	// PresetVlessWsTLS VLESS + WebSocket + TLS(ACME)。可过 Cloudflare CDN。
	PresetVlessWsTLS PresetKind = "vless-ws-tls"

	// PresetHysteria2 Hysteria2(QUIC/UDP)。需要证书,弱网表现优于 TCP 系。
	PresetHysteria2 PresetKind = "hysteria2"

	// PresetTrojanWsTLS Trojan + WebSocket + TLS。老客户端兼容档。
	PresetTrojanWsTLS PresetKind = "trojan-ws-tls"

	// PresetVmessWsTLS VMess + WebSocket + TLS。老客户端兼容档。
	PresetVmessWsTLS PresetKind = "vmess-ws-tls"

	// PresetMixedBasicAuth mixed 入站(同端口同时提供 HTTP 代理与 SOCKS5),
	// Basic Auth 鉴权。这是「HTTP/S5 代理订阅」附加业务的落点 ——
	// Shadowrocket 等客户端可直接导入 http:// / socks:// 订阅。
	//
	// 注意:mixed 默认**不加 TLS**。裸 HTTP/SOCKS5 代理正是这类业务的形态
	// (客户端按 http/socks 协议直连),加 TLS 反而大多数客户端不认。
	PresetMixedBasicAuth PresetKind = "mixed-basicauth"
)

// AllPresets 供 API 暴露给主控做能力发现(主控蓝图编辑器据此渲染选项)。
var AllPresets = []PresetKind{
	PresetVlessReality,
	PresetVlessWsTLS,
	PresetHysteria2,
	PresetTrojanWsTLS,
	PresetVmessWsTLS,
	PresetMixedBasicAuth,
}

// PresetMeta 预设的描述元信息 —— 主控 UI 直接渲染,不必在两个仓库各维护一份文案。
type PresetMeta struct {
	Kind PresetKind `json:"kind"`
	Name string     `json:"name"`
	Desc string     `json:"desc"`
	// NeedsDomain 是否需要一个解析到本机的域名(→ 主控必须先建 A 记录)
	NeedsDomain bool `json:"needs_domain"`
	// NeedsCert 是否需要 ACME 证书(→ 需要 Cloudflare Token 做 DNS-01)
	NeedsCert bool `json:"needs_cert"`
	// CDNCapable 是否可以挂在 Cloudflare 代理(橙云)后面
	CDNCapable bool `json:"cdn_capable"`
	// Protocol sing-box 入站 type
	Protocol string `json:"protocol"`
}

// PresetCatalog 全部预设的元信息。顺序即主控 UI 的展示顺序。
var PresetCatalog = []PresetMeta{
	{
		Kind: PresetVlessReality, Name: "VLESS-Reality", Protocol: "vless",
		Desc:        "XTLS-Vision + Reality,无需域名与证书,抗封锁最强",
		NeedsDomain: false, NeedsCert: false, CDNCapable: false,
	},
	{
		Kind: PresetVlessWsTLS, Name: "VLESS-WS-TLS", Protocol: "vless",
		Desc:        "WebSocket + TLS,可挂 Cloudflare CDN 隐藏落地 IP",
		NeedsDomain: true, NeedsCert: true, CDNCapable: true,
	},
	{
		Kind: PresetHysteria2, Name: "Hysteria2", Protocol: "hysteria2",
		Desc:        "QUIC/UDP,弱网与高丢包环境体验显著优于 TCP",
		NeedsDomain: true, NeedsCert: true, CDNCapable: false,
	},
	{
		Kind: PresetTrojanWsTLS, Name: "Trojan-WS-TLS", Protocol: "trojan",
		Desc:        "老客户端兼容档,WebSocket + TLS",
		NeedsDomain: true, NeedsCert: true, CDNCapable: true,
	},
	{
		Kind: PresetVmessWsTLS, Name: "VMess-WS-TLS", Protocol: "vmess",
		Desc:        "老客户端兼容档,WebSocket + TLS",
		NeedsDomain: true, NeedsCert: true, CDNCapable: true,
	},
	{
		Kind: PresetMixedBasicAuth, Name: "HTTP/SOCKS5", Protocol: "mixed",
		Desc:        "同端口提供 HTTP 与 SOCKS5 代理,Basic Auth 鉴权(附加业务)",
		NeedsDomain: false, NeedsCert: false, CDNCapable: false,
	},
}

// LookupPreset 按 kind 找元信息。未知 preset 返回 (zero, false)。
func LookupPreset(kind PresetKind) (PresetMeta, bool) {
	for _, m := range PresetCatalog {
		if m.Kind == kind {
			return m, true
		}
	}
	return PresetMeta{}, false
}

// === 预设输入 ===

// InboundSpec 主控声明的单条入站意图。
//
// 除 Preset 外全部可留空 —— 留空的字段由 preset 用安全默认值补齐,
// 这正是「主控只填 IP/用户名/密码」得以成立的原因。
type InboundSpec struct {
	Preset PresetKind `json:"preset"`
	// Tag 入站标签(sing-box 全局唯一)。留空 → 自动生成 "<preset>-<4hex>"。
	Tag string `json:"tag"`
	// Port 监听端口。0 → 自动选一个空闲端口(见 pickFreePort)。
	Port int `json:"port"`
	// Listen 监听地址,留空 → "::" (双栈)。
	Listen string `json:"listen"`

	// Fqdn 该入站使用的域名。needs_domain 的 preset 必填(由主控建好 A 记录后传入)。
	Fqdn string `json:"fqdn"`

	// SNI Reality 的伪装目标域名。留空 → 用 defaultRealityDest。
	// 仅 PresetVlessReality 使用。
	SNI string `json:"sni"`

	// Path WebSocket 路径。留空 → 随机 "/<8hex>"。仅 ws 系 preset 使用。
	Path string `json:"path"`

	// Proxied 该入站是否位于 Cloudflare 橙云之后。
	// 影响:分享链接里 host 头的处理与端口选择(CDN 只回源 443/2053/2083/2087/2096/8443)。
	Proxied bool `json:"proxied"`

	// TLSID 复用已存在的 TLS 记录(通配符证书共享场景)。
	// >0 时 preset 不再自建 TLS,直接引用。
	TLSID uint `json:"tls_id"`

	// Remark 备注,写进 inbound 的 tag 后缀不影响功能,仅作运营识别。
	Remark string `json:"remark"`
}

// BuiltTLS preset 要求先落库的 TLS 记录。TLSID>0(复用)或 preset 不需要 TLS 时为 nil。
type BuiltTLS struct {
	Name string `json:"name"`
	// Server 下发给 sing-box 的服务端 TLS 配置(含 reality.private_key / acme 块)
	Server json.RawMessage `json:"server"`
	// Client 分享链接生成用的客户端侧配置(含 reality.public_key / utls 指纹)
	Client json.RawMessage `json:"client"`
}

// BuiltInbound preset 的产物。
type BuiltInbound struct {
	// TLS 非 nil 时,调用方必须先建这条 TLS 记录,把返回的 id 填进 InboundJSON["tls_id"]
	TLS *BuiltTLS `json:"tls,omitempty"`
	// InboundJSON 可直接交给 ConfigService.Save("inbounds","new",...) 的 JSON 对象
	InboundJSON map[string]any `json:"inbound"`
	// Tag / Port / Protocol 冗余出来供编排层记账与回报主控
	Tag      string `json:"tag"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	// Fqdn 该入站对外使用的地址(reality/mixed 无域名时为空,由主控回填节点 IP)
	Fqdn string `json:"fqdn"`
}

// defaultRealityDest Reality 握手伪装目标。
//
// 选择标准:TLS1.3 + H2、境内外都可达、流量大到不会因"很多人连它"而显眼、
// 且不属于会主动封禁的服务。cloudflare.com 自身是最稳的选择之一 ——
// 任何 CDN 客户的回源都会打到它,指纹淹没在噪声里。
const defaultRealityDest = "www.cloudflare.com"

// cdnOriginPorts Cloudflare 橙云回源支持的 HTTPS 端口。
// 挂 CDN 的入站端口必须落在这个集合里,否则 CDN 根本不会回源。
var cdnOriginPorts = []int{443, 2053, 2083, 2087, 2096, 8443}

// BuildInbound 按 spec 生成一条入站的完整配置。
//
// 端口选择顺序:spec.Port > CDN 场景从 cdnOriginPorts 挑空闲 > 随机高位空闲端口。
// usedPorts 是本批次内已经分配出去的端口(还没真正 listen,系统探测不到),
// 调用方逐条 Build 时把已分配端口传进来避免同批冲突。
func BuildInbound(spec InboundSpec, usedPorts map[int]bool) (*BuiltInbound, error) {
	meta, ok := LookupPreset(spec.Preset)
	if !ok {
		return nil, fmt.Errorf("未知的交付预设: %q", spec.Preset)
	}
	if meta.NeedsDomain && strings.TrimSpace(spec.Fqdn) == "" {
		return nil, fmt.Errorf("预设 %s 需要域名,但 fqdn 为空", spec.Preset)
	}
	if usedPorts == nil {
		usedPorts = map[int]bool{}
	}

	port, err := resolvePort(spec, meta, usedPorts)
	if err != nil {
		return nil, err
	}
	usedPorts[port] = true

	tag := strings.TrimSpace(spec.Tag)
	if tag == "" {
		suffix, err := randHexString(2)
		if err != nil {
			return nil, err
		}
		tag = string(spec.Preset) + "-" + suffix
	}

	listen := strings.TrimSpace(spec.Listen)
	if listen == "" {
		listen = "::"
	}

	out := &BuiltInbound{
		Tag:      tag,
		Port:     port,
		Protocol: meta.Protocol,
		Fqdn:     spec.Fqdn,
	}

	// 所有入站共有的监听骨架
	inbound := map[string]any{
		"type":        meta.Protocol,
		"tag":         tag,
		"listen":      listen,
		"listen_port": port,
		"enable":      true,
		"tls_id":      spec.TLSID,
	}

	// 分享链接里的 server 字段该填什么(入站级覆盖全局 linkAddrSource)。
	//
	// 有域名的入站必须用证书的 SNI —— 用 IP 的话客户端做 TLS 握手时
	// 证书域名对不上,直接失败。无域名的(Reality / mixed)留空跟随全局
	// "ip",由 provision.go 的 applyLinkAddressSettings 写好节点公网 IP。
	if meta.NeedsDomain {
		inbound["link_addr_source"] = "tls"
	}

	switch spec.Preset {
	case PresetVlessReality:
		if err := buildVlessReality(spec, inbound, out); err != nil {
			return nil, err
		}
	case PresetVlessWsTLS:
		buildWsTLS(spec, inbound, out, "vless")
	case PresetTrojanWsTLS:
		buildWsTLS(spec, inbound, out, "trojan")
	case PresetVmessWsTLS:
		buildWsTLS(spec, inbound, out, "vmess")
	case PresetHysteria2:
		if err := buildHysteria2(spec, inbound, out); err != nil {
			return nil, err
		}
	case PresetMixedBasicAuth:
		// mixed 不需要额外字段:type/listen/listen_port 就够了。
		// 用户(Basic Auth 凭据)走 clients 表,由主控按订阅创建。
	default:
		return nil, fmt.Errorf("预设 %s 尚未实现", spec.Preset)
	}

	out.InboundJSON = inbound
	return out, nil
}

// buildVlessReality VLESS + XTLS-Vision + Reality。
//
// Reality 的密钥对分两处:
//   - 服务端 tls.reality.private_key(落 model.Tls.Server)
//   - 客户端 tls.reality.public_key(落 model.Tls.Client,genLink 取它拼 pbk 参数)
//
// short_id 服务端存数组、客户端存单值 —— util/genLink.go 从数组里随机挑一个
// 发给客户端,所以两边格式不同是有意为之,不要"统一"。
func buildVlessReality(spec InboundSpec, inbound map[string]any, out *BuiltInbound) error {
	dest := strings.TrimSpace(spec.SNI)
	if dest == "" {
		dest = defaultRealityDest
	}

	// Vision 流控:Reality 场景的标准搭配,规避 TLS-in-TLS 特征。
	// 注意 flow 是 **client 侧** 字段(在 clients.config.vless 里),
	// 入站本身不带 flow —— 这是 sing-box 与 xray 的一个差异点。

	if spec.TLSID > 0 {
		// 复用既有 Reality TLS 记录
		return nil
	}

	priv, pub, err := generateRealityKeypair()
	if err != nil {
		return fmt.Errorf("生成 Reality 密钥对失败: %w", err)
	}
	shortID, err := randHexString(4) // 8 hex chars
	if err != nil {
		return err
	}

	server := map[string]any{
		"enabled":     true,
		"server_name": dest,
		"reality": map[string]any{
			"enabled": true,
			"handshake": map[string]any{
				"server":      dest,
				"server_port": 443,
			},
			"private_key": priv,
			"short_id":    []string{shortID},
		},
	}
	client := map[string]any{
		"enabled":     true,
		"server_name": dest,
		"utls": map[string]any{
			"enabled": true,
			// chrome 指纹在真实流量里占比最大,单独识别成本最高
			"fingerprint": "chrome",
		},
		"reality": map[string]any{
			"enabled":    true,
			"public_key": pub,
			"short_id":   shortID,
		},
	}

	srvJSON, err := json.Marshal(server)
	if err != nil {
		return err
	}
	cliJSON, err := json.Marshal(client)
	if err != nil {
		return err
	}
	out.TLS = &BuiltTLS{
		Name:   "reality-" + out.Tag,
		Server: srvJSON,
		Client: cliJSON,
	}
	// Reality 不用域名,对外地址由主控回填成节点公网 IP
	out.Fqdn = ""
	return nil
}

// buildWsTLS vless/trojan/vmess 三者的 WebSocket + TLS 形态只差 type,
// 传输层与 TLS 配置完全一致 —— 合并成一个构造器避免三份拷贝各自漂移。
//
// TLS 记录不在这里造:ACME 证书需要 Cloudflare Token,而 token 是编排层
// (provision.go)才持有的运行期机密,preset 保持纯函数不碰它。
// 编排层负责建好 acme TLS 记录后把 id 填进 inbound["tls_id"]。
func buildWsTLS(spec InboundSpec, inbound map[string]any, out *BuiltInbound, protocol string) {
	path := strings.TrimSpace(spec.Path)
	if path == "" {
		if h, err := randHexString(4); err == nil {
			path = "/" + h
		} else {
			path = "/ws"
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	transport := map[string]any{
		"type": "ws",
		"path": path,
		// early_data 降低首包延迟;Cloudflare 与主流客户端都支持。
		"max_early_data":         2048,
		"early_data_header_name": "Sec-WebSocket-Protocol",
	}
	if spec.Fqdn != "" {
		transport["headers"] = map[string]any{"Host": spec.Fqdn}
	}
	inbound["transport"] = transport
	_ = protocol // type 已在骨架里按 meta.Protocol 设好
}

// buildHysteria2 Hysteria2 入站。
//
// 与 TCP 系不同,hysteria2 的 TLS 是协议内建的(QUIC),必须绑证书 —— 没有
// Reality 之类的免证书路径。证书同样由编排层建好后填 tls_id。
func buildHysteria2(spec InboundSpec, inbound map[string]any, out *BuiltInbound) error {
	// 带宽留空 = 让 Hysteria2 走 BBR 拥塞控制自适应。
	// 写死 up/down_mbps 会关掉 BBR 改用 Brutal,在带宽估计不准时反而更差,
	// 所以这里刻意不设 —— 运营需要时可在面板里手动加。
	inbound["ignore_client_bandwidth"] = false
	// masquerade:被主动探测时伪装成一个普通网站而不是回 QUIC 错误
	inbound["masquerade"] = "https://" + defaultRealityDest
	return nil
}

// === 端口分配 ===

// resolvePort 决定入站监听端口。
//
//	spec.Port > 0            → 用它(但仍校验空闲,占用则报错让调用方知道冲突)
//	CDN 场景(Proxied)       → 从 cdnOriginPorts 挑第一个空闲的
//	其余                     → 随机高位端口(20000-60000)
func resolvePort(spec InboundSpec, meta PresetMeta, used map[int]bool) (int, error) {
	if spec.Port > 0 {
		if used[spec.Port] {
			return 0, fmt.Errorf("端口 %d 在本批次内已被占用", spec.Port)
		}
		if !portFree(spec.Port) {
			return 0, fmt.Errorf("端口 %d 已被本机其它进程占用", spec.Port)
		}
		return spec.Port, nil
	}

	if spec.Proxied && meta.CDNCapable {
		for _, p := range cdnOriginPorts {
			if !used[p] && portFree(p) {
				return p, nil
			}
		}
		return 0, fmt.Errorf("Cloudflare 回源端口 %v 全部被占用,无法为 CDN 入站分配端口", cdnOriginPorts)
	}

	return pickFreePort(used)
}

// pickFreePort 在 20000-60000 随机试探,最多 64 次。
//
// 用真实 net.Listen 试探而不是读 /proc/net/tcp:sing-box 监听的端口在
// s-ui 进程之外,只有内核知道谁真正持有它 —— Listen 失败就是被占,零误判。
// 代价是每次探测有一次 syscall,64 次上限完全可接受。
func pickFreePort(used map[int]bool) (int, error) {
	for i := 0; i < 64; i++ {
		n, err := randIntBelow(40000)
		if err != nil {
			return 0, err
		}
		p := 20000 + n
		if used[p] {
			continue
		}
		if portFree(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("连续 64 次都没抽到空闲端口,请检查本机端口占用情况")
}

// portFree TCP+UDP 双栈探测。任一协议占用即视为不可用 ——
// hysteria2 走 UDP、其余走 TCP,统一按"两个都空闲"发放可以避免
// 后续换协议时撞车,代价只是稍微保守一点。
func portFree(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = l.Close()

	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	_ = pc.Close()
	return true
}

// === 密码学 helpers ===

// generateRealityKeypair 生成 sing-box Reality 用的 X25519 密钥对。
//
// 编码为 base64.RawURLEncoding —— 这是 sing-box / xray 的 reality 密钥约定格式
// (`sing-box generate reality-keypair` 输出的就是它)。用 base64.StdEncoding
// 会因为 '+' '/' 字符在 URL 与配置里被转义而炸,不要改。
func generateRealityKeypair() (privateKey, publicKey string, err error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv.Bytes()), enc.EncodeToString(priv.PublicKey().Bytes()), nil
}

// randHexString 返回 n 字节随机数的十六进制(长度 2n)。
func randHexString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// randIntBelow 返回 [0, max) 的密码学随机整数。
//
// 用取模会引入模偏置,对端口分配无实际危害,但这里样本空间小、
// 拒绝采样的代价可忽略,索性做对。
func randIntBelow(max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("randIntBelow: max 必须为正,收到 %d", max)
	}
	// 取 4 字节,拒绝掉会造成偏置的尾段
	limit := (1 << 32) - ((1 << 32) % uint64(max))
	buf := make([]byte, 4)
	for i := 0; i < 32; i++ {
		if _, err := rand.Read(buf); err != nil {
			return 0, err
		}
		v := uint64(buf[0])<<24 | uint64(buf[1])<<16 | uint64(buf[2])<<8 | uint64(buf[3])
		if v < limit {
			return int(v % uint64(max)), nil
		}
	}
	return 0, fmt.Errorf("randIntBelow: 拒绝采样连续 32 次未命中(不应发生)")
}
