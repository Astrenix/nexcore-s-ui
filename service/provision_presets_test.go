package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestRealityKeypairFormat Reality 密钥必须是 base64.RawURLEncoding 的 32 字节 X25519。
//
// 这条断言看着琐碎,但格式错了的后果极不成比例:sing-box 会正常启动、入站会正常
// 监听、面板一切正常 —— 只有客户端连不上,而且报的是含糊的握手失败。用
// StdEncoding(带 '+' '/' 和 '=' padding)是最容易犯的错,因为它也能解码成功。
func TestRealityKeypairFormat(t *testing.T) {
	priv, pub, err := generateRealityKeypair()
	if err != nil {
		t.Fatalf("生成密钥对失败: %v", err)
	}

	for name, key := range map[string]string{"private": priv, "public": pub} {
		if strings.ContainsAny(key, "+/=") {
			t.Errorf("%s key %q 含 StdEncoding 字符,必须是 RawURLEncoding", name, key)
		}
		raw, err := base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			t.Errorf("%s key %q 不是合法 RawURLEncoding: %v", name, key, err)
			continue
		}
		if len(raw) != 32 {
			t.Errorf("%s key 解码后 %d 字节,X25519 要求 32", name, len(raw))
		}
	}

	// 两次生成必须不同 —— 若某次重构把密钥变成常量,这里会立刻炸
	priv2, _, _ := generateRealityKeypair()
	if priv == priv2 {
		t.Error("连续两次生成的私钥相同,密钥没有真正随机化")
	}
}

// TestBuildInboundAllPresets 每个 preset 都要能产出可序列化、字段完整的入站。
func TestBuildInboundAllPresets(t *testing.T) {
	used := map[int]bool{}
	for _, meta := range PresetCatalog {
		spec := InboundSpec{Preset: meta.Kind}
		if meta.NeedsDomain {
			spec.Fqdn = "node-test.example.com"
		}
		if meta.NeedsCert {
			spec.TLSID = 42 // 假装编排层已经建好共享证书
		}

		built, err := BuildInbound(spec, used)
		if err != nil {
			t.Errorf("preset %s 构建失败: %v", meta.Kind, err)
			continue
		}
		if built.Protocol != meta.Protocol {
			t.Errorf("preset %s protocol=%q,期望 %q", meta.Kind, built.Protocol, meta.Protocol)
		}
		if built.Port <= 0 || built.Port > 65535 {
			t.Errorf("preset %s 分配了非法端口 %d", meta.Kind, built.Port)
		}
		if built.Tag == "" {
			t.Errorf("preset %s 没有生成 tag", meta.Kind)
		}
		if _, err := json.Marshal(built.InboundJSON); err != nil {
			t.Errorf("preset %s 的入站配置无法序列化: %v", meta.Kind, err)
		}
		// 只有 Reality 自带 TLS;其余靠编排层给 tls_id
		if meta.Kind == PresetVlessReality && built.TLS == nil {
			t.Error("vless-reality 必须自带 TLS 记录(含 private_key)")
		}
		if meta.Kind != PresetVlessReality && built.TLS != nil {
			t.Errorf("preset %s 不该自建 TLS", meta.Kind)
		}
	}
}

// TestRealityTLSShape Reality 的服务端配置带 private_key、客户端配置带 public_key,
// 且 short_id 服务端是数组、客户端是单值 —— util/genLink.go 依赖这个非对称形状。
func TestRealityTLSShape(t *testing.T) {
	built, err := BuildInbound(InboundSpec{Preset: PresetVlessReality}, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if built.TLS == nil {
		t.Fatal("vless-reality 没产出 TLS 记录")
	}

	var server, client map[string]any
	if err := json.Unmarshal(built.TLS.Server, &server); err != nil {
		t.Fatalf("server 配置不是合法 JSON: %v", err)
	}
	if err := json.Unmarshal(built.TLS.Client, &client); err != nil {
		t.Fatalf("client 配置不是合法 JSON: %v", err)
	}

	srvReality, _ := server["reality"].(map[string]any)
	if srvReality == nil {
		t.Fatal("server 侧缺 reality 块")
	}
	if _, ok := srvReality["private_key"].(string); !ok {
		t.Error("server 侧 reality 缺 private_key")
	}
	if _, ok := srvReality["short_id"].([]any); !ok {
		t.Error("server 侧 reality.short_id 必须是数组 —— genLink 从数组里随机挑一个下发")
	}
	if _, ok := srvReality["public_key"]; ok {
		t.Error("server 侧不该出现 public_key(私钥配置泄给客户端形状就错了)")
	}

	cliReality, _ := client["reality"].(map[string]any)
	if cliReality == nil {
		t.Fatal("client 侧缺 reality 块")
	}
	if _, ok := cliReality["public_key"].(string); !ok {
		t.Error("client 侧 reality 缺 public_key —— genLink 取它拼分享链接的 pbk 参数")
	}
	if _, ok := cliReality["short_id"].(string); !ok {
		t.Error("client 侧 reality.short_id 必须是单个字符串")
	}
	if _, ok := cliReality["private_key"]; ok {
		t.Error("private_key 绝不能出现在 client 配置里 —— 它会随分享链接发给用户")
	}
}

// TestBuildInboundRejectsMissingDomain needs_domain 的 preset 缺 fqdn 必须失败而不是
// 静默建出一条连不上的入站。
func TestBuildInboundRejectsMissingDomain(t *testing.T) {
	for _, meta := range PresetCatalog {
		if !meta.NeedsDomain {
			continue
		}
		_, err := BuildInbound(InboundSpec{Preset: meta.Kind}, nil)
		if err == nil {
			t.Errorf("preset %s 需要域名,缺 fqdn 时却构建成功了", meta.Kind)
		}
	}
}

// TestBuildInboundUnknownPreset 未知 preset 必须报错 —— 主控版本比节点新时
// 会发生,静默忽略会让主控以为交付成功。
func TestBuildInboundUnknownPreset(t *testing.T) {
	if _, err := BuildInbound(InboundSpec{Preset: "vless-quantum-teleport"}, nil); err == nil {
		t.Error("未知 preset 应当报错")
	}
}

// TestPortAllocationNoCollision 同一批交付里多条入站不能分到同一个端口。
//
// 这是真实风险:批内先分配的端口还没被任何进程 listen,系统探测不到,
// 只有 usedPorts 这张表拦得住。
func TestPortAllocationNoCollision(t *testing.T) {
	used := map[int]bool{}
	seen := map[int]string{}

	specs := []InboundSpec{
		{Preset: PresetVlessReality, Tag: "a"},
		{Preset: PresetMixedBasicAuth, Tag: "b"},
		{Preset: PresetVlessReality, Tag: "c"},
		{Preset: PresetMixedBasicAuth, Tag: "d"},
		{Preset: PresetHysteria2, Tag: "e", Fqdn: "x.example.com", TLSID: 1},
	}
	for _, spec := range specs {
		built, err := BuildInbound(spec, used)
		if err != nil {
			t.Fatalf("tag %s 构建失败: %v", spec.Tag, err)
		}
		if prev, dup := seen[built.Port]; dup {
			t.Errorf("端口 %d 被重复分配给 %s 和 %s", built.Port, prev, spec.Tag)
		}
		seen[built.Port] = spec.Tag
	}
}

// TestCDNPortConstraint 挂 CDN 的入站端口必须落在 Cloudflare 的回源端口集合里,
// 否则橙云根本不会回源 —— 表现为"配置全对但就是连不上"。
func TestCDNPortConstraint(t *testing.T) {
	built, err := BuildInbound(InboundSpec{
		Preset:  PresetVlessWsTLS,
		Fqdn:    "cdn.example.com",
		TLSID:   1,
		Proxied: true,
	}, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	for _, p := range cdnOriginPorts {
		if built.Port == p {
			return
		}
	}
	t.Errorf("CDN 入站分到端口 %d,不在 Cloudflare 回源端口集合 %v 内", built.Port, cdnOriginPorts)
}

// TestWsPathAlwaysRooted WebSocket 路径必须以 / 开头 —— 客户端与 CDN 都按
// 绝对路径匹配,少个斜杠就是 404。
func TestWsPathAlwaysRooted(t *testing.T) {
	cases := []string{"", "ws", "/ws", "mypath"}
	for _, in := range cases {
		built, err := BuildInbound(InboundSpec{
			Preset: PresetVlessWsTLS, Fqdn: "x.example.com", TLSID: 1, Path: in,
		}, nil)
		if err != nil {
			t.Fatalf("path=%q 构建失败: %v", in, err)
		}
		transport, _ := built.InboundJSON["transport"].(map[string]any)
		if transport == nil {
			t.Fatalf("path=%q 时缺 transport 块", in)
		}
		got, _ := transport["path"].(string)
		if !strings.HasPrefix(got, "/") {
			t.Errorf("path=%q → 生成 %q,没有以 / 开头", in, got)
		}
	}
}

// TestMixedHasNoTLS mixed(HTTP/SOCKS5 附加业务)不该带 TLS —— 客户端按裸
// http/socks 协议直连,套上 TLS 大多数客户端不认。
func TestMixedHasNoTLS(t *testing.T) {
	built, err := BuildInbound(InboundSpec{Preset: PresetMixedBasicAuth}, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if built.TLS != nil {
		t.Error("mixed 入站不该自建 TLS")
	}
	if id, _ := built.InboundJSON["tls_id"].(uint); id != 0 {
		t.Errorf("mixed 入站 tls_id 应为 0,实际 %d", id)
	}
}
