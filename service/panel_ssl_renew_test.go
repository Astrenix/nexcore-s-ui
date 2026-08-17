package service

// 面板证书自动续签的判定护栏。
//
// 2026-08-17 事故:tw1 / ca1 的面板证书 8/8 静默过期,主控调节点 API 全部 x509 失败,
// 用户买了流量包一条线路都开不出来。根因是续签只有手动入口。加了自动续签之后,
// 判定错一次的代价与那次事故等价 —— 所以把"该不该续"抽成纯函数钉在这里。

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldRenewPanelCert(t *testing.T) {
	cases := []struct {
		name string
		info PanelCertInfo
		want bool
	}{
		{"远未到期", PanelCertInfo{Configured: true, DaysLeft: 89}, false},
		{"刚好卡在阈值上 —— 续", PanelCertInfo{Configured: true, DaysLeft: PanelCertRenewThresholdDays}, true},
		{"阈值内一天 —— 续", PanelCertInfo{Configured: true, DaysLeft: PanelCertRenewThresholdDays - 1}, true},
		{"阈值外一天 —— 不续", PanelCertInfo{Configured: true, DaysLeft: PanelCertRenewThresholdDays + 1}, false},
		// 事故当时正是这个状态:已过期 9 天。若判定写成 `DaysLeft>0 && <=30`,过期越久越不修
		{"已过期(DaysLeft 为负)—— 必须续", PanelCertInfo{Configured: true, DaysLeft: -9, Expired: true}, true},
		{"面板跑 HTTP,没配证书 —— 不该乱签", PanelCertInfo{Configured: false}, false},
		{"证书文件读不出来 —— 不在这里决策", PanelCertInfo{Configured: true, Error: "读取证书文件失败"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRenewPanelCert(c.info); got != c.want {
				t.Fatalf("shouldRenewPanelCert(%+v) = %v, want %v", c.info, got, c.want)
			}
		})
	}
}

// writeSelfSignedCert 生成一张自签证书写到临时文件,notAfter 由调用方指定。
func writeSelfSignedCert(t *testing.T, notAfter time.Time) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "panel.example.test"},
		NotBefore:    notAfter.Add(-90 * 24 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	path := filepath.Join(t.TempDir(), "cert.crt")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	return path
}

// 端到端(只到判定为止):真实证书文件 → GetPanelCertInfo → shouldRenewPanelCert。
// 这条链路是自动续签的全部输入,解析出错或天数算反都会在这里暴露。
func TestGetPanelCertInfoFeedsRenewDecision(t *testing.T) {
	svc := PanelSSLService{}

	t.Run("还剩 60 天不该续", func(t *testing.T) {
		info := svc.GetPanelCertInfo(writeSelfSignedCert(t, time.Now().Add(60*24*time.Hour)))
		if info.Error != "" {
			t.Fatalf("解析失败: %s", info.Error)
		}
		if shouldRenewPanelCert(info) {
			t.Fatalf("剩 %d 天却判定要续", info.DaysLeft)
		}
	})

	t.Run("只剩 6 天必须续", func(t *testing.T) {
		info := svc.GetPanelCertInfo(writeSelfSignedCert(t, time.Now().Add(6*24*time.Hour)))
		if !shouldRenewPanelCert(info) {
			t.Fatalf("剩 %d 天却判定不用续", info.DaysLeft)
		}
	})

	t.Run("已过期 9 天必须续(事故当时的状态)", func(t *testing.T) {
		info := svc.GetPanelCertInfo(writeSelfSignedCert(t, time.Now().Add(-9*24*time.Hour)))
		if !info.Expired {
			t.Fatalf("过期证书没被标记 Expired: %+v", info)
		}
		if !shouldRenewPanelCert(info) {
			t.Fatalf("已过期 %d 天却判定不用续", -info.DaysLeft)
		}
	})

	t.Run("文件不存在时不做续签决策", func(t *testing.T) {
		info := svc.GetPanelCertInfo(filepath.Join(t.TempDir(), "nope.crt"))
		if shouldRenewPanelCert(info) {
			t.Fatal("读不到证书就去签,会在配置异常时反复烧 LE 配额")
		}
	})
}
