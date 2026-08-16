package service

// 节点交付编排(Provision Orchestration)
//
// 这一层是「主控只填 IP/用户名/密码」得以成立的关键:主控下发一份**声明式**的
// 交付意图,节点自己把它变成现实 —— 建 DNS A 记录、签 TLS 证书、创建入站、
// reload sing-box。主控不需要懂 sing-box 配置长什么样。
//
// 与 provision_presets.go 的分工:
//   - presets:纯函数,"一条 vless-reality 入站的 JSON 长什么样"
//   - 本文件:副作用,"把它真的建出来",并保证幂等
//
// ## 幂等模型
//
// 主控会重试(网络抖动、装机回调超时重放)。重试不能产生第二套入站。
// 幂等锚点是 **inbound tag**:
//   - 主控显式给 tag → 按它判重
//   - 未给 → 用稳定 tag `nx-<preset>`(同一 preset 每个节点一条)
// 已存在同 tag 的入站直接复用并回报,不重建、不改配置。
//
// TLS 记录同理,锚点是 model.Tls.Name(= fqdn 或 "reality-<tag>")。
//
// ## 为什么 Cloudflare Token 会落到节点上
//
// 证书走 sing-box 内置 ACME 的 DNS-01 challenge,**续签时同样需要 token**
// (每 60 天一次,主控未必在线)。所以 token 必须留在节点。
// 缓解措施是权限最小化:该 token 只需 `Zone:DNS:Edit`,且限定到单个 zone ——
// 主控端 UI 必须把这条写在输入框旁边。节点失陷时攻击者只能改这个 zone 的 DNS,
// 碰不到 Cloudflare 账号本身。
//
// 不需要证书的交付(纯 vless-reality + mixed)可以令 PersistCFToken=false,
// 此时节点上不留任何 Cloudflare 凭据。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// provisionActor 写进 model.Changes 的操作者名 —— 面板的变更历史里能看出
// "这条入站是主控自动交付的,不是人手工建的"。
const provisionActor = "nexcore-controller"

// ProvisionRequest 主控下发的交付意图。
type ProvisionRequest struct {
	// === Cloudflare(需要域名或证书的 preset 才必填)===

	// CFToken Cloudflare API Token。权限只需 Zone:DNS:Edit。
	CFToken string `json:"cf_token"`
	// CFZoneID 目标 zone。留空但给了 CFToken 时,若只有一个 zone 则自动选中。
	CFZoneID string `json:"cf_zone_id"`
	// ACMEEmail Let's Encrypt 注册邮箱。签证书必填。
	ACMEEmail string `json:"acme_email"`
	// PersistCFToken 是否把 token 存进节点 settings 供 ACME 自动续签。
	// 交付里有 needs_cert 的 preset 时应当为 true,否则 60 天后证书过期。
	PersistCFToken bool `json:"persist_cf_token"`

	// === 域名 ===

	// Fqdn 主控已建好 A 记录时直接指定,节点跳过建记录这步。
	Fqdn string `json:"fqdn"`
	// SubdomainPrefix Fqdn 为空且需要域名时,用它作前缀自动生成
	// `<prefix>-<8hex>.<zone>`。留空 → "n"。
	SubdomainPrefix string `json:"subdomain_prefix"`
	// Proxied 自动建的 A 记录是否开 Cloudflare 橙云代理。
	// 注意:整个交付共用一条 A 记录,所以这是节点级开关 ——
	// 同一节点上既要 CDN 又要直连时,主控应发两次 provision 用不同 fqdn。
	Proxied bool `json:"proxied"`

	// === 节点身份 ===

	// NodePublicIP 主控视角的节点公网 IP。
	//
	// **以主控给的为准**,不用节点自探:节点在 NAT / 多网卡 / IPv6-only 场景下
	// 自探结果经常不是主控实际能连上的那个地址,而主控手里的 IP 正是它 SSH
	// 进来用的那个 —— 那是唯一被证明可达的地址。留空才回退到自探。
	NodePublicIP string `json:"node_public_ip"`

	// === 交付内容 ===

	Inbounds []InboundSpec `json:"inbounds"`
}

// ProvisionedInbound 单条入站的交付结果。
//
// Error 非空表示这一条失败了,其余条目仍可能成功 —— 交付是 per-inbound
// 独立成败的,不做全或无。主控据此决定重试哪几条。
type ProvisionedInbound struct {
	Preset   string `json:"preset"`
	Tag      string `json:"tag"`
	ID       uint   `json:"id"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	TLSID    uint   `json:"tls_id"`
	// Address 客户端应当连接的地址:有域名用域名,否则用节点公网 IP。
	// 主控生成订阅链接时直接用它,不必自己判断。
	Address string `json:"address"`
	// Reused true = 幂等命中,本次没有创建新入站
	Reused bool   `json:"reused"`
	Error  string `json:"error,omitempty"`
}

// ProvisionResult 整次交付的结果。
type ProvisionResult struct {
	Fqdn     string               `json:"fqdn"`
	PublicIP string               `json:"public_ip"`
	Inbounds []ProvisionedInbound `json:"inbounds"`
	// Warnings 不致命但主控应当知道的事(如 token 未持久化导致证书无法续签)
	Warnings []string `json:"warnings"`
	// TookMs 整次交付耗时,主控日志里能看出是哪一步慢
	TookMs int64 `json:"took_ms"`
}

// ProvisionService 交付编排。零值可用。
type ProvisionService struct {
	cf      CloudflareService
	config  ConfigService
	setting SettingService
	inbound InboundService
}

// Apply 执行一次交付。
//
// 步骤:
//  1. 解析节点公网 IP
//  2. 需要域名时:建/复用 A 记录 → fqdn
//  3. 需要证书时:建/复用一条 ACME TLS 记录(整次交付共用一张证书)
//  4. 逐条入站:幂等检查 → BuildInbound → 落 Reality TLS(若需) → 落入站
//  5. 持久化 CF token(若要求)
//
// 任何一条入站失败都只影响它自己;函数只在"前置条件根本不成立"时返回 error。
func (s *ProvisionService) Apply(req ProvisionRequest) (*ProvisionResult, error) {
	started := time.Now()
	res := &ProvisionResult{Inbounds: make([]ProvisionedInbound, 0, len(req.Inbounds))}

	if len(req.Inbounds) == 0 {
		return nil, fmt.Errorf("交付意图为空:inbounds 至少要有一条")
	}

	// --- 1. 节点公网 IP ---
	publicIP := strings.TrimSpace(req.NodePublicIP)
	if publicIP == "" {
		publicIP = s.cf.DetectPublicIP()
		if publicIP == "" {
			return nil, fmt.Errorf("拿不到节点公网 IP:主控未下发 node_public_ip,节点自探也失败")
		}
		res.Warnings = append(res.Warnings,
			"主控未下发 node_public_ip,已回退到节点自探("+publicIP+");NAT/多网卡环境下这可能不是可达地址")
	}
	res.PublicIP = publicIP

	// --- 2. 这批交付需要什么 ---
	needDomain, needCert := false, false
	for _, spec := range req.Inbounds {
		meta, ok := LookupPreset(spec.Preset)
		if !ok {
			continue // 未知 preset 留到逐条处理时报错,不阻断整批
		}
		if meta.NeedsDomain {
			needDomain = true
		}
		if meta.NeedsCert && spec.TLSID == 0 {
			needCert = true
		}
	}

	// --- 3. 域名 ---
	fqdn := strings.TrimSpace(req.Fqdn)
	if needDomain && fqdn == "" {
		var err error
		fqdn, err = s.ensureDNSRecord(req, publicIP)
		if err != nil {
			return nil, fmt.Errorf("建 DNS 记录失败: %w", err)
		}
	}
	res.Fqdn = fqdn

	// --- 4. 共享 ACME 证书 ---
	// 整批交付共用一张证书:同一个 fqdn 签多次既浪费 Let's Encrypt 配额
	// (每域名每周 50 张),也让节点上堆出一堆等价的 TLS 记录。
	sharedTLSID := uint(0)
	if needCert {
		if req.ACMEEmail == "" {
			return nil, fmt.Errorf("本次交付含需要证书的入站,但 acme_email 为空")
		}
		if req.CFToken == "" {
			return nil, fmt.Errorf("本次交付含需要证书的入站,但 cf_token 为空(DNS-01 challenge 必需)")
		}
		var err error
		sharedTLSID, err = s.ensureACMETLS(fqdn, req.ACMEEmail, req.CFToken)
		if err != nil {
			return nil, fmt.Errorf("创建 ACME TLS 记录失败: %w", err)
		}
	}

	// --- 5. 逐条入站 ---
	// usedPorts 跨条累积:同批次内先分配出去的端口还没真正 listen,
	// 系统探测不到,必须靠这张表避免同批撞车。
	usedPorts := map[int]bool{}
	for _, spec := range req.Inbounds {
		out := s.provisionOne(spec, fqdn, publicIP, sharedTLSID, usedPorts)
		res.Inbounds = append(res.Inbounds, out)
	}

	// --- 6. 让分享链接指向正确的地址 ---
	if err := s.applyLinkAddressSettings(publicIP); err != nil {
		res.Warnings = append(res.Warnings, "写入节点地址设置失败,分享链接可能指向错误地址: "+err.Error())
	}

	// --- 7. 持久化 Cloudflare token 供证书续签 ---
	if req.PersistCFToken && req.CFToken != "" {
		if err := s.setting.SetCfToken(req.CFToken, req.ACMEEmail); err != nil {
			res.Warnings = append(res.Warnings, "Cloudflare token 持久化失败,证书到期后无法自动续签: "+err.Error())
		}
	} else if needCert && !req.PersistCFToken {
		res.Warnings = append(res.Warnings,
			"本次签发了 ACME 证书但未持久化 Cloudflare token —— 证书到期(约 60 天)后无法自动续签")
	}

	res.TookMs = time.Since(started).Milliseconds()
	return res, nil
}

// provisionOne 交付单条入站。任何失败都收进返回值的 Error 字段,不往上抛。
func (s *ProvisionService) provisionOne(spec InboundSpec, fqdn, publicIP string, sharedTLSID uint, usedPorts map[int]bool) ProvisionedInbound {
	out := ProvisionedInbound{Preset: string(spec.Preset)}

	meta, ok := LookupPreset(spec.Preset)
	if !ok {
		out.Error = fmt.Sprintf("未知的交付预设 %q", spec.Preset)
		return out
	}
	out.Protocol = meta.Protocol

	// 稳定 tag:主控没给就用 nx-<preset>,让重试能靠 tag 判重。
	// 同一 preset 要在一个节点上开多条时,主控必须显式给不同 tag。
	tag := strings.TrimSpace(spec.Tag)
	if tag == "" {
		tag = "nx-" + string(spec.Preset)
		spec.Tag = tag
	}
	out.Tag = tag

	// 幂等:已有同 tag 入站 → 原样复用,不动它的配置。
	// 重跑交付的常见原因是上一次网络中断,而不是"配置需要更新" ——
	// 贸然覆盖会把运营手工调过的参数抹掉。要改配置走面板或专门的 update 接口。
	if existing, err := s.findInboundByTag(tag); err != nil {
		out.Error = "查询既有入站失败: " + err.Error()
		return out
	} else if existing != nil {
		out.ID = existing.id
		out.Port = existing.port
		out.TLSID = existing.tlsID
		out.Reused = true
		out.Address = pickAddress(meta, fqdn, publicIP)
		return out
	}

	// 需要证书的 preset:优先用本批共享证书,主控显式指定 tls_id 时以它为准
	if meta.NeedsCert && spec.TLSID == 0 {
		spec.TLSID = sharedTLSID
	}
	if meta.NeedsDomain && spec.Fqdn == "" {
		spec.Fqdn = fqdn
	}

	built, err := BuildInbound(spec, usedPorts)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Port = built.Port

	// Reality 类 preset 自带一份 TLS(含 private_key),先落库拿 id
	if built.TLS != nil {
		tlsID, err := s.ensureRealityTLS(built.TLS)
		if err != nil {
			out.Error = "创建 Reality TLS 记录失败: " + err.Error()
			return out
		}
		built.InboundJSON["tls_id"] = tlsID
	}
	if v, ok := built.InboundJSON["tls_id"].(uint); ok {
		out.TLSID = v
	}

	payload, err := json.Marshal(built.InboundJSON)
	if err != nil {
		out.Error = "序列化入站配置失败: " + err.Error()
		return out
	}

	// 走 ConfigService.Save 而不是直接写表 —— 这条路径会同时做:
	// sing-box AddInbound(热加载,不重启)、FillOutJson(分享链接元数据)、
	// model.Changes(变更历史)、LastUpdate(前端增量刷新)。绕过去就全丢了。
	//
	// initUsers 传空:客户端由主控按订阅 fan-out 创建。InboundService.initUsers
	// 会为多账号协议注入哨兵账号,入站在拿到真实客户端之前是"监听但无人能登"。
	if _, err := s.config.Save("inbounds", "new", payload, "", provisionActor, ""); err != nil {
		out.Error = "保存入站失败: " + err.Error()
		return out
	}

	// 回查 id — Save 不返回它,但主控需要 id 才能调 /inbounds/:id/clients
	if created, err := s.findInboundByTag(tag); err == nil && created != nil {
		out.ID = created.id
		out.TLSID = created.tlsID
	}
	out.Address = pickAddress(meta, fqdn, publicIP)
	return out
}

// pickAddress 客户端该连哪个地址。
// 有域名的 preset 用域名(证书 SNI 要对得上);Reality / mixed 用裸 IP。
func pickAddress(meta PresetMeta, fqdn, publicIP string) string {
	if meta.NeedsDomain && fqdn != "" {
		return fqdn
	}
	return publicIP
}

// applyLinkAddressSettings 让分享链接指向节点的真实地址。
//
// 不做这一步会出真问题:`linkAddrSource` 默认是 "panel",分享链接的 server
// 字段跟着**HTTP 请求的 Host 头**走。主控是通过面板 URL 调 API 的,于是
// 用户拿到的订阅可能指向面板域名、内网地址、甚至 127.0.0.1 —— 配置全对
// 但就是连不上,而且极难排查(面板里看什么都正常)。
//
// 交付出来的入站里,Reality 与 mixed 根本没有域名,只能用 IP;有域名的入站
// 则由 preset 在入站级设 link_addr_source="tls"(用证书 SNI)覆盖这个全局值。
// 所以全局设成 "ip" 是对两类入站都正确的基线。
//
// ## 已配置过的节点一律不碰
//
// panelIp 非空 = 这台节点的地址策略已经有人定过了(老节点人工配的,或上一次
// 交付写的)。这时**什么都不做**。
//
// 覆盖它的后果不是"配置被改了"这么轻:老节点若在用 linkAddrSource=panel
// (跟着面板域名出订阅),被改成 ip 之后,已经发出去的订阅里 server 字段
// 会从域名变成裸 IP —— 绑域名证书的入站当场 TLS 握手失败,而运营那边
// 看到的是"什么都没动,用户却集体连不上"。
//
// 新装的节点 panelIp 必然为空,所以自动化那条路不受影响。
func (s *ProvisionService) applyLinkAddressSettings(publicIP string) error {
	if strings.TrimSpace(publicIP) == "" {
		return nil
	}
	if existing := s.setting.GetPanelIp(); existing != "" {
		// 已有地址策略 —— 尊重它,交付只管建入站
		return nil
	}
	payload, err := json.Marshal(map[string]string{
		"panelIp":        publicIP,
		"linkAddrSource": "ip",
	})
	if err != nil {
		return err
	}
	// 走 ConfigService.Save 而非直接改表:它会一并写 model.Changes 与
	// LastUpdate,面板前端的增量刷新才能看到这次改动。
	_, err = s.config.Save("settings", "", json.RawMessage(payload), "", provisionActor, "")
	return err
}

// === DNS ===

// ensureDNSRecord 保证存在一条指向本机的 A 记录,返回 fqdn。
//
// 子域名是随机的(`<prefix>-<8hex>`):固定名字会让"某个 zone 下的节点清单"
// 可枚举 —— 扫一遍 jp1/jp2/jp3 就能把全部落地摸出来。随机化让被动收集失效。
func (s *ProvisionService) ensureDNSRecord(req ProvisionRequest, publicIP string) (string, error) {
	token := strings.TrimSpace(req.CFToken)
	if token == "" {
		return "", fmt.Errorf("需要域名的入站要求 cf_token,但它是空的")
	}
	prefix, err := sanitizeSubdomainPrefix(req.SubdomainPrefix)
	if err != nil {
		return "", err
	}
	zoneID, err := s.resolveZoneID(token, req.CFZoneID)
	if err != nil {
		return "", err
	}

	sub := s.cf.RandomSubdomain(prefix)
	fqdn, _, err := s.cf.UpsertARecord(token, zoneID, sub, publicIP, req.Proxied)
	if err != nil {
		return "", err
	}
	return fqdn, nil
}

// sanitizeSubdomainPrefix 校验并归一化子域名前缀。
//
// 这个值会走两条路,两条都不容脏输入:
//   - 拼进 fqdn(`<prefix>-<8hex>.<zone>`)后**建成真实 DNS 记录**
//   - 拼进 Cloudflare API 的 query(`?type=A&name=<fqdn>`)
//
// 于是 `.` 会把记录建到另一级子域下、`&` 或 `#` 会截断 query 让"查同名记录"
// 查空进而重复创建、空格会得到一个 Cloudflare 拒绝但错误信息完全指不到原因的请求。
// 这些都不是攻击,是运营手滑就会踩到的 —— 而后果发生在别人的域名上。
//
// 规则按 DNS 标签来:小写字母、数字、连字符,不以连字符开头结尾。
// 长度留 24(后面还要接 `-` + 8 位 hex,总长要留在 63 字节标签上限内)。
func sanitizeSubdomainPrefix(raw string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(raw))
	if p == "" {
		return "", nil // 空 → 由 RandomSubdomain 回退到 "n"
	}
	if len(p) > 24 {
		return "", fmt.Errorf("子域名前缀过长(%d 字符),上限 24", len(p))
	}
	if strings.HasPrefix(p, "-") || strings.HasSuffix(p, "-") {
		return "", fmt.Errorf("子域名前缀 %q 不能以连字符开头或结尾", raw)
	}
	for _, r := range p {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return "", fmt.Errorf("子域名前缀 %q 含非法字符 %q —— 只允许小写字母、数字、连字符", raw, string(r))
	}
	return p, nil
}

// resolveZoneID 主控没指定 zone 时,只有恰好一个 zone 才自动选中。
// 多 zone 下猜错会把节点域名建到别人的站点上,宁可报错让主控明确指定。
func (s *ProvisionService) resolveZoneID(token, given string) (string, error) {
	if z := strings.TrimSpace(given); z != "" {
		return z, nil
	}
	zones, err := s.cf.ListZones(token)
	if err != nil {
		return "", fmt.Errorf("列举 Cloudflare zone 失败: %w", err)
	}
	switch len(zones) {
	case 0:
		return "", fmt.Errorf("该 Cloudflare token 下没有任何 zone")
	case 1:
		return zones[0].Id, nil
	default:
		names := make([]string, 0, len(zones))
		for _, z := range zones {
			names = append(names, z.Name)
		}
		return "", fmt.Errorf("该 token 下有 %d 个 zone(%s),请在主控明确指定 cf_zone_id",
			len(zones), strings.Join(names, ", "))
	}
}

// === TLS ===

// ensureACMETLS 建/复用一条 ACME TLS 记录,以 fqdn 作幂等锚点。
func (s *ProvisionService) ensureACMETLS(fqdn, email, token string) (uint, error) {
	if fqdn == "" {
		return 0, fmt.Errorf("签证书需要 fqdn,但它是空的")
	}
	if id, err := s.findTLSByName(fqdn); err != nil {
		return 0, err
	} else if id > 0 {
		return id, nil
	}
	return s.cf.IssueTLS(fqdn, fqdn, email, token, "")
}

// ensureRealityTLS 建/复用 Reality 的 TLS 记录。
//
// 不能复用 IssueTLS —— 那是 ACME 专用的。这里直接落库,并补上
// model.Changes,否则前端 store 的 tlsConfigs 拿不到增量,入站编辑页的
// TLS 下拉会看不到这条(跟 cloudflare.go 里 IssueTLS 的处理同理)。
func (s *ProvisionService) ensureRealityTLS(spec *BuiltTLS) (uint, error) {
	if id, err := s.findTLSByName(spec.Name); err != nil {
		return 0, err
	} else if id > 0 {
		return id, nil
	}

	tls := model.Tls{
		Name:   spec.Name,
		Server: spec.Server,
		Client: spec.Client,
	}

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Create(&tls).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	chg, _ := json.Marshal(map[string]any{
		"id":   tls.Id,
		"name": tls.Name,
		"src":  "provision-reality",
	})
	if err := tx.Create(&model.Changes{
		DateTime: time.Now().Unix(),
		Actor:    provisionActor,
		Key:      "tls",
		Action:   "new",
		Obj:      chg,
	}).Error; err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	LastUpdate.Store(time.Now().Unix())
	return tls.Id, nil
}

func (s *ProvisionService) findTLSByName(name string) (uint, error) {
	var t model.Tls
	err := database.GetDB().Model(&model.Tls{}).Where("name = ?", name).Limit(1).Find(&t).Error
	if err != nil {
		return 0, err
	}
	return t.Id, nil
}

// === 入站查询 ===

type existingInbound struct {
	id    uint
	port  int
	tlsID uint
	tag   string
	typ   string
}

// findInboundByTag 找不到返回 (nil, nil)。
func (s *ProvisionService) findInboundByTag(tag string) (*existingInbound, error) {
	var row model.Inbound
	err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", tag).Limit(1).Find(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Id == 0 {
		return nil, nil
	}
	out := &existingInbound{
		id:    row.Id,
		tlsID: row.TlsIdValue(),
		tag:   row.Tag,
		typ:   row.Type,
	}
	// listen_port 在 Options 里(model.Inbound 把非固定字段都塞那)
	if len(row.Options) > 0 {
		var opts map[string]any
		if err := json.Unmarshal(row.Options, &opts); err == nil {
			if p, ok := opts["listen_port"].(float64); ok {
				out.port = int(p)
			}
		}
	}
	return out, nil
}

// === 状态回报 ===

// ProvisionState 节点当前的交付状态。主控用它做对账:
// 定期拉一次,跟自己记的 listing 比对,发现漂移(运营手工删了入站等)。
type ProvisionState struct {
	// Managed 本节点是否已被主控交付过(存在任何 nx- 前缀的入站)
	Managed  bool                 `json:"managed"`
	Inbounds []ProvisionStateItem `json:"inbounds"`
	// Presets 本节点支持的交付预设 —— 主控据此渲染蓝图编辑器,
	// 老版本节点不支持新 preset 时主控能提前发现而不是交付到一半失败。
	Presets []PresetMeta `json:"presets"`
	// HasCFToken 节点是否持有 Cloudflare token(能否自动续签证书)
	HasCFToken bool `json:"has_cf_token"`
}

type ProvisionStateItem struct {
	ID       uint   `json:"id"`
	Tag      string `json:"tag"`
	Type     string `json:"type"`
	Port     int    `json:"port"`
	TLSID    uint   `json:"tls_id"`
	Enable   bool   `json:"enable"`
	Provided bool   `json:"provisioned"` // tag 带 nx- 前缀 = 主控交付的
}

// State 汇报当前节点的交付状态。
func (s *ProvisionService) State() (*ProvisionState, error) {
	var rows []model.Inbound
	if err := database.GetDB().Model(&model.Inbound{}).Find(&rows).Error; err != nil {
		return nil, err
	}

	out := &ProvisionState{Presets: PresetCatalog, Inbounds: make([]ProvisionStateItem, 0, len(rows))}
	for i := range rows {
		r := rows[i]
		item := ProvisionStateItem{
			ID:       r.Id,
			Tag:      r.Tag,
			Type:     r.Type,
			TLSID:    r.TlsIdValue(),
			Enable:   r.Enable,
			Provided: strings.HasPrefix(r.Tag, "nx-"),
		}
		if len(r.Options) > 0 {
			var opts map[string]any
			if err := json.Unmarshal(r.Options, &opts); err == nil {
				if p, ok := opts["listen_port"].(float64); ok {
					item.Port = int(p)
				}
			}
		}
		if item.Provided {
			out.Managed = true
		}
		out.Inbounds = append(out.Inbounds, item)
	}

	tok, _ := s.setting.GetCfToken()
	out.HasCFToken = tok != ""
	return out, nil
}
