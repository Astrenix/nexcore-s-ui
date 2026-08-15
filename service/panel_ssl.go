package service

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/cloudflare"
)

// PanelCertInfo 面板 HTTPS 证书的到期信息,给设置页展示 + 决定是否该续签。
type PanelCertInfo struct {
	Configured bool     `json:"configured"`           // 是否配置了 webCertFile
	Domains    []string `json:"domains,omitempty"`    // 证书覆盖的域名(CN + SAN)
	Issuer     string   `json:"issuer,omitempty"`     // 签发机构 CN
	NotBefore  int64    `json:"notBefore,omitempty"`  // 生效时间(unix)
	NotAfter   int64    `json:"notAfter,omitempty"`   // 到期时间(unix)
	DaysLeft   int      `json:"daysLeft"`             // 距到期天数(已过期为负)
	Expired    bool     `json:"expired"`              // 是否已过期
	Error      string   `json:"error,omitempty"`      // 读取/解析失败原因
}

// GetPanelCertInfo 读取 webCertFile 指向的证书链,解析首个叶子证书的到期信息。
// 不做任何写操作,纯只读展示。文件缺失/非法只在结果里带 Error,不返回硬错误
// (设置页仍要能正常渲染)。
func (s *PanelSSLService) GetPanelCertInfo(certFile string) PanelCertInfo {
	if strings.TrimSpace(certFile) == "" {
		return PanelCertInfo{Configured: false}
	}
	info := PanelCertInfo{Configured: true}
	pemData, err := os.ReadFile(certFile)
	if err != nil {
		info.Error = "读取证书文件失败: " + err.Error()
		return info
	}
	// 证书链里可能有多段 PEM,第一段 CERTIFICATE 是叶子证书(面板域名的那张)
	var leaf *x509.Certificate
	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, perr := x509.ParseCertificate(block.Bytes)
		if perr != nil {
			continue
		}
		leaf = cert
		break
	}
	if leaf == nil {
		info.Error = "证书文件里没有可解析的 CERTIFICATE 块"
		return info
	}

	domains := leaf.DNSNames
	if len(domains) == 0 && leaf.Subject.CommonName != "" {
		domains = []string{leaf.Subject.CommonName}
	}
	info.Domains = domains
	info.Issuer = leaf.Issuer.CommonName
	info.NotBefore = leaf.NotBefore.Unix()
	info.NotAfter = leaf.NotAfter.Unix()
	info.DaysLeft = int(time.Until(leaf.NotAfter).Hours() / 24)
	info.Expired = time.Now().After(leaf.NotAfter)
	return info
}

// PanelSSLDir 是面板自己用的 ACME 证书存储根。certmagic FileStorage 会在
// 这下面生成 certificates/<issuer>/<domain>/<domain>.{crt,key,json}。
//
// 跟 sing-box 入站 ACME(./acme/<fqdn>/)分开 —— 那个是 sing-box 自己管,
// 路径默认相对工作目录。面板 SSL 是面板自己用,放绝对路径,装载更稳。
const PanelSSLDir = "/usr/local/nexcore-s-ui/cert"

type PanelSSLService struct{}

// IssuePanelSSL 用 Cloudflare DNS-01 给面板域名签 Let's Encrypt 证书。
// 复用面板里持久化的 cfCredentials(token + email),caller 不用重粘。
// 返回 (certPath, keyPath) 供 caller 写到 webCertFile/webKeyFile settings。
//
// 同步阻塞,签发流程通常 30s ~ 2min(看 DNS 传播)。失败带具体原因。
func (s *PanelSSLService) IssuePanelSSL(domain, email, cfToken string) (string, string, error) {
	if domain == "" {
		return "", "", common.NewError("域名不能为空")
	}
	if email == "" {
		return "", "", common.NewError("ACME 注册邮箱不能为空(去 TLS 一键签发流程里保存过 CF Token + email 后会自动复用)")
	}
	if cfToken == "" {
		return "", "", common.NewError("Cloudflare API Token 不能为空")
	}
	if err := os.MkdirAll(PanelSSLDir, 0700); err != nil {
		return "", "", common.NewError("无法创建证书目录 ", PanelSSLDir, ": ", err.Error())
	}

	storage := &certmagic.FileStorage{Path: PanelSSLDir}
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) {
			return certmagic.New(nil, certmagic.Config{Storage: storage}), nil
		},
	})
	cfg := certmagic.New(cache, certmagic.Config{
		Storage: storage,
		Logger:  nil,
	})
	acmeIssuer := certmagic.NewACMEIssuer(cfg, certmagic.ACMEIssuer{
		CA:     certmagic.LetsEncryptProductionCA,
		Email:  email,
		Agreed: true,
		DNS01Solver: &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: &cloudflare.Provider{APIToken: cfToken},
			},
		},
	})
	cfg.Issuers = []certmagic.Issuer{acmeIssuer}

	// 单证书签发 + 落库,3 分钟超时(包含 DNS-01 challenge 传播等待)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	logger.Info("PanelSSL: 开始为 ", domain, " 走 Cloudflare DNS-01 签发证书…")
	if err := cfg.ManageSync(ctx, []string{domain}); err != nil {
		return "", "", common.NewError("ACME 签发失败: ", err.Error())
	}

	// 读出 storage 里的实际路径,塞给 settings
	issuerKey := acmeIssuer.IssuerKey()
	certKey := certmagic.StorageKeys.SiteCert(issuerKey, domain)
	keyKey := certmagic.StorageKeys.SitePrivateKey(issuerKey, domain)
	certPath := filepath.Join(PanelSSLDir, filepath.FromSlash(certKey))
	keyPath := filepath.Join(PanelSSLDir, filepath.FromSlash(keyKey))
	if _, err := os.Stat(certPath); err != nil {
		return "", "", common.NewError("证书文件未找到: ", certPath, " (", err.Error(), ")")
	}
	if _, err := os.Stat(keyPath); err != nil {
		return "", "", common.NewError("私钥文件未找到: ", keyPath, " (", err.Error(), ")")
	}
	logger.Info("PanelSSL: 签发成功 cert=", certPath, " key=", keyPath)
	return certPath, keyPath, nil
}

// IssueAndApply 一站式:
//  1. 在 Cloudflare 写 A 记录 → 域名能解析到本机公网 IP(否则即使签了证书,
//     用户浏览器也解析不到面板)。这是关键步骤,旧版漏掉了 A 记录导致
//     "证书签好了但域名打不开"。
//  2. ACME DNS-01 签发 Let's Encrypt 证书。
//  3. 写 settings(webCertFile/webKeyFile/webDomain)。
// 不重启面板 —— 由 caller 在响应返回后异步触发,避免响应未发就把自己关了。
func (s *PanelSSLService) IssueAndApply(settingSvc *SettingService, cfSvc *CloudflareService, domain, email, cfToken string) error {
	// Step 1:在 Cloudflare 加 A 记录,指向本机公网 IP。
	publicIP := cfSvc.DetectPublicIP()
	if publicIP == "" {
		return common.NewError("无法获取本机公网 IP — 请手动在 Cloudflare 给 " + domain + " 加 A 记录,或确认服务器能访问 ipify/icanhazip 等 echo IP 服务")
	}
	zones, err := cfSvc.ListZones(cfToken)
	if err != nil {
		return common.NewError("列出 Cloudflare zone 失败: ", err.Error())
	}
	// 找跟 domain 匹配的 zone(后缀匹配,取最长匹配 — 多 zone 嵌套时取更具体的)
	var zoneId, zoneName string
	for _, z := range zones {
		if z.Name == domain || strings.HasSuffix(domain, "."+z.Name) {
			if len(z.Name) > len(zoneName) {
				zoneId = z.Id
				zoneName = z.Name
			}
		}
	}
	if zoneId == "" {
		return common.NewError("Cloudflare 上找不到 " + domain + " 对应的 zone — 确认该域名已托管在 CF + Token 有此 zone 的 Zone:Read 权限")
	}
	// 计算子域名(去掉 zone 后缀)。domain == zone 时用 "@" 表示根域。
	subname := "@"
	if domain != zoneName {
		subname = strings.TrimSuffix(domain, "."+zoneName)
	}
	// proxied=false:面板需要直连(443/3095 端口走橙色云朵反代不通,且 ACME
	// DNS-01 也不要求反代)
	fqdn, _, err := cfSvc.UpsertARecord(cfToken, zoneId, subname, publicIP, false)
	if err != nil {
		return common.NewError("Cloudflare A 记录写入失败: ", err.Error())
	}
	logger.Info("PanelSSL: ", fqdn, " → ", publicIP, " A 记录已 upsert(zone=", zoneName, ")")

	// Step 2:跑 ACME 签证书
	certPath, keyPath, err := s.IssuePanelSSL(domain, email, cfToken)
	if err != nil {
		return err
	}

	// Step 3:写 settings
	for _, kv := range [][2]string{
		{"webCertFile", certPath},
		{"webKeyFile", keyPath},
		{"webDomain", domain},
	} {
		if err := settingSvc.saveSetting(kv[0], kv[1]); err != nil {
			return common.NewError("写 setting ", kv[0], " 失败: ", err.Error())
		}
	}
	return nil
}
