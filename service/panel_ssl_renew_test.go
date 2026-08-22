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
	"strings"
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

// ─────────────────────────────────────────────────────────────────────────────
// 续签节流:必须跨【重启】生效。
//
// 2026-08-22 E566:节流原本只有一个进程内变量 panelCertRenewLastTry。
// 而 cronjob/cronJob.go 的首检 goroutine 在面板启动 2 分钟后就跑一次 CertRenewJob,
// 于是"6 小时最小间隔"对重启完全无效 —— 每重启一次就是一次新的 ACME 尝试。
//
// 这恰恰是 panelCertRenewMinInterval 的注释里写明要防的场景:
// 8/6 事故的形态就是 crash-loop 反复撞 LE 429、retry-after 被无限顺延;
// 而运维在证书出问题时最自然的动作也是重启面板。LE 同域名每周 5 张重复证书,
// 打满之后 retry-after 可达数天 —— 恢复窗口被自己毒化。
//
// 下面 "重启后" 那一格是本次修复唯一改变行为的一格:把持久化那半去掉,只有它会红。
// ─────────────────────────────────────────────────────────────────────────────
func TestShouldThrottlePanelCertRenew(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	var never time.Time

	cases := []struct {
		name           string
		mem, persisted time.Time
		want           bool
	}{
		{"从未尝试过 —— 必须放行(否则证书到期了也不续)", never, never, false},

		// 同进程内:内存那半负责
		{"同进程内 10 分钟前刚试过 —— 挡", ago(10 * time.Minute), never, true},
		{"同进程内 7 小时前试过 —— 放行", ago(7 * time.Hour), never, false},

		// 🩸 重启后:内存归零,只剩持久化那半。这一格就是本次修复。
		{"重启后(内存归零)10 分钟前刚试过 —— 仍要挡", never, ago(10 * time.Minute), true},
		{"重启后 5h59m 前试过 —— 仍要挡", never, ago(5*time.Hour + 59*time.Minute), true},
		{"重启后 7 小时前试过 —— 放行", never, ago(7 * time.Hour), false},

		// 两半都有值:取更晚的那个
		{"持久化比内存新 —— 按持久化算,挡", ago(9 * time.Hour), ago(30 * time.Minute), true},
		{"内存比持久化新 —— 按内存算,挡", ago(30 * time.Minute), ago(9 * time.Hour), true},
		{"两半都很旧 —— 放行", ago(8 * time.Hour), ago(9 * time.Hour), false},

		// 边界:间隔【满】6 小时就该放行,写成 <= 会让重试永远晚一轮
		{"恰好满 6 小时 —— 放行", ago(panelCertRenewMinInterval), never, false},
		{"差 1 秒不满 6 小时 —— 挡", ago(panelCertRenewMinInterval - time.Second), never, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldThrottlePanelCertRenew(c.mem, c.persisted, now); got != c.want {
				t.Fatalf("shouldThrottlePanelCertRenew(mem=%v, persisted=%v) = %v, want %v",
					c.mem, c.persisted, got, c.want)
			}
		})
	}
}

// 持久化的写入侧与读取侧必须用【同一个】时间 layout。
//
// 两侧是分开写的:写在 RenewPanelCertIfExpiring 里(Format),读在
// loadPanelCertRenewLastTry 里(Parse)。改一边编译照过、测试照绿,
// 只是 Parse 从此永远失败 → 返回零值 → 节流【静默】退回到"只有内存那半",
// 也就是悄悄退回本次修复之前的状态。没有任何信号会告诉你这件事。
func TestPanelCertRenewTimestampLayoutIsPaired(t *testing.T) {
	src, err := os.ReadFile("panel_ssl_renew.go")
	if err != nil {
		t.Fatalf("读源文件: %v", err)
	}
	s := string(src)

	formatLayout := captureAfter(t, s, "panelCertRenewLastTry.Format(", ")")
	parseLayout := captureAfter(t, s, "time.Parse(", ",")

	if formatLayout == "" || parseLayout == "" {
		t.Fatalf("没提取到 layout(写=%q 读=%q)—— 判据失效了,不是代码干净", formatLayout, parseLayout)
	}
	if formatLayout != parseLayout {
		t.Fatalf("续签时间戳的写入 layout(%s)与读取 layout(%s)不一致 —— "+
			"Parse 会永远失败,节流静默退回「只有内存那半」,重启一次就能重跑 ACME",
			formatLayout, parseLayout)
	}
}

// captureAfter 取 start 之后、end 之前的那段,去掉首尾空白。
func captureAfter(t *testing.T, s, start, end string) string {
	t.Helper()
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}
