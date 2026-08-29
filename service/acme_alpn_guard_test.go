package service

// 共享 ACME TLS 记录的 ALPN 必须同时覆盖 QUIC 系与 TCP 系入站。
//
// 🩸 2026-08-29 实测事故:acmeSharedALPN 原来只有 ["h2","http/1.1"]。
// 这份 TLS 记录是【一证多入站共用】的(ensureACMETLS 建一次,provision 把它挂给
// 所有 needs_cert 的入站),而 ALPN 是【按传输层】分的 —— hysteria2 走 QUIC,
// 客户端只提议 h3,于是握手阶段就被服务端拒掉:
//
//	CRYPTO_ERROR 0x178 (remote): tls: no application protocol
//
// 表现极具迷惑性:入站正常监听、面板一切正常、分享链接也生成得出来,
// **只有真的连一次才知道它从来没通过**。首台带 hysteria2 的节点开出来就是死的。
//
// 本守卫不硬抄清单,而是【跟着 PresetCatalog 走】:只要目录里存在需要证书的
// QUIC 系 preset,共享 ALPN 就必须含 h3;将来新增 tuic 之类也自动被覆盖。

import (
	"testing"
)

// quicTransportProtocols 是走 QUIC 的协议 —— 它们的 ALPN 必须是 h3。
var quicTransportProtocols = map[string]bool{
	"hysteria":  true,
	"hysteria2": true,
	"tuic":      true,
}

func alpnHas(want string) bool {
	for _, a := range acmeSharedALPN {
		if a == want {
			return true
		}
	}
	return false
}

func TestSharedACMEALPNCoversEveryTransportInCatalog(t *testing.T) {
	var quicPresets, tcpPresets []string
	for _, m := range PresetCatalog {
		if !m.NeedsCert {
			continue // 不签证书的(Reality / mixed)不共用这条 TLS 记录
		}
		if quicTransportProtocols[m.Protocol] {
			quicPresets = append(quicPresets, string(m.Kind))
		} else {
			tcpPresets = append(tcpPresets, string(m.Kind))
		}
	}

	// 自证 ① —— 一个 QUIC preset 都没识别到,本守卫就是空转的。
	// 目录里删光 QUIC preset 时应当【主动出声】,而不是静静地变成一条永远通过的用例。
	if len(quicPresets) == 0 {
		t.Fatal("PresetCatalog 里没有任何『需要证书的 QUIC 系 preset』—— " +
			"要么协议名不在 quicTransportProtocols 里(判据瞎了),要么目录真的删空了。" +
			"两种情况都要人来确认,不能默默放过")
	}
	// 自证 ② —— TCP 系那半边同理。
	if len(tcpPresets) == 0 {
		t.Fatal("PresetCatalog 里没有任何『需要证书的 TCP 系 preset』—— 判据的另一半失效了")
	}
	t.Logf("识别面:QUIC 系 %v / TCP 系 %v", quicPresets, tcpPresets)

	if !alpnHas("h3") {
		t.Fatalf("acmeSharedALPN=%v 缺 h3,而目录里有走 QUIC 的 preset %v —— "+
			"这些入站会在 TLS 握手就被服务端拒掉(alert 120 no_application_protocol),"+
			"而监听端口、面板状态、分享链接全都正常,只有真连一次才发现",
			acmeSharedALPN, quicPresets)
	}
	for _, need := range []string{"h2", "http/1.1"} {
		if !alpnHas(need) {
			t.Fatalf("acmeSharedALPN=%v 缺 %q,而目录里有走 TCP+TLS 的 preset %v —— "+
				"只顾着修 QUIC 会把这些反向打断",
				acmeSharedALPN, need, tcpPresets)
		}
	}
}
