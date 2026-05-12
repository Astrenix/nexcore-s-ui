package service

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
)

type InboundService struct {
	ClientService
}

func (s *InboundService) Get(ids string) (*[]map[string]interface{}, error) {
	if ids == "" {
		return s.GetAll()
	}
	return s.getById(ids)
}

func (s *InboundService) getById(ids string) (*[]map[string]interface{}, error) {
	var inbound []model.Inbound
	var result []map[string]interface{}
	db := database.GetDB()
	err := db.Model(model.Inbound{}).Where("id in ?", strings.Split(ids, ",")).Scan(&inbound).Error
	if err != nil {
		return nil, err
	}
	for _, inb := range inbound {
		inbData, err := inb.MarshalFull()
		if err != nil {
			return nil, err
		}
		result = append(result, *inbData)
	}
	return &result, nil
}

func (s *InboundService) GetAll() (*[]map[string]interface{}, error) {
	db := database.GetDB()
	inbounds := []model.Inbound{}
	err := db.Model(model.Inbound{}).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}

	// 一次性预拉聚合数据,循环里 O(1) lookup,避免 N+1 查询:
	//   1. 流量累计  StatsService.GetTotals  → tag → {up, down}
	//   2. 在线 IP    onlineResources.InboundIPs(SaveStats cron 每 10s 刷)
	//   3. 中转目标  解析 setting.config.route.rules 里 _nb_binding 标记的规则
	//   4. 出站显示名 outbounds.display_name(给中转目标回填中转名称)
	// 这些之前都是前端从 /load 的 onlines/stats/config/outbounds 各自再 join 一遍,
	// 数据在网络上传多次。现在直接合并到 inbound 行,前端表格逐字段读即可。
	totals, _ := (&StatsService{}).GetTotals("inbound")
	inboundRelay := buildInboundRelayMap()
	outboundDisplay := buildOutboundDisplayMap()
	// 全局 link_addr_source — 用来给每个 inbound 算 effective_addr_source
	globalLinkSrc := (&SettingService{}).GetLinkAddrSource()

	var data []map[string]interface{}
	for _, inbound := range inbounds {
		var shadowtls_version uint
		ss_managed := false
		inbData := map[string]interface{}{}
		// 把 sing-box options 整体展开到顶层 — 主控 + 前端编辑表单都要完整字段
		// (transport / tls / multiplex / network / users / address 等)。
		// 之前只摘 listen + listen_port,导致 tun 的 address / vmess 的 transport
		// 全在 GET 里看不到,前端编辑要回查才能拿全。
		if inbound.Options != nil {
			var restFields map[string]json.RawMessage
			if err := json.Unmarshal(inbound.Options, &restFields); err != nil {
				return nil, err
			}
			for k, v := range restFields {
				inbData[k] = v
			}
			if inbound.Type == "shadowtls" {
				json.Unmarshal(restFields["version"], &shadowtls_version)
			}
			if inbound.Type == "shadowsocks" {
				json.Unmarshal(restFields["managed"], &ss_managed)
			}
		}
		// 顶层 DB 字段在 options 字段之后覆盖 — 它们是 model.Inbound 真值
		// (id / tls_id / enable / type / tag),options 里若有 type/tag 也以 DB 为准
		inbData["id"] = inbound.Id
		inbData["type"] = inbound.Type
		inbData["tag"] = inbound.Tag
		inbData["tls_id"] = inbound.TlsId
		inbData["enable"] = inbound.Enable
		// 这三个字段不在 options 里,但前端编辑表单 + 主控对接需要拿全。
		// 之前 GetAll 只展 options + id/type/tag/tls_id/enable,导致 addrs(多
		// server)/ out_json(出站 JSON)/ ext(per-cred 限额)在 GET 里全丢。
		// 都返 RawMessage(addrs/out_json)或 JSON object(ext 解析后)— 前端就可
		// 直接读字段、不用回查单条 inbound 详情。
		if len(inbound.Addrs) > 0 {
			inbData["addrs"] = inbound.Addrs
		}
		if len(inbound.OutJson) > 0 {
			inbData["out_json"] = inbound.OutJson
		}
		if inbound.Ext != "" {
			var extObj interface{}
			if err := json.Unmarshal([]byte(inbound.Ext), &extObj); err == nil {
				inbData["ext"] = extObj
			}
		}
		// link_addr_source — 入站级覆盖全局 settings.linkAddrSource。空表示
		// 跟随全局。effective_addr_source 是 resolve 后的最终值(panel|ip|tls),
		// 入站列表 UI 直接展示这个,免前端再 join settings。
		inbData["link_addr_source"] = inbound.LinkAddrSource
		inbData["effective_addr_source"] = resolveAddrSource(inbound.LinkAddrSource, globalLinkSrc)
		// 入站列表 UI 直接读这三个字段,免去前端跨 onlines/stats/config 三处 join。
		if t, ok := totals[inbound.Tag]; ok {
			inbData["total_up"] = t["up"]
			inbData["total_down"] = t["down"]
		} else {
			inbData["total_up"] = int64(0)
			inbData["total_down"] = int64(0)
		}
		if onlineResources != nil && onlineResources.InboundIPs != nil {
			inbData["online_ips"] = onlineResources.InboundIPs[inbound.Tag]
		} else {
			inbData["online_ips"] = 0
		}
		if relay, ok := inboundRelay[inbound.Tag]; ok && relay != "" {
			// relay_to 输出 {tag, display_name} 对象,前端一次拿全。
			// display_name 即出站编辑器里的"中转名称",空时只输出 tag。
			relayObj := map[string]string{"tag": relay}
			if dn := outboundDisplay[relay]; dn != "" {
				relayObj["display_name"] = dn
			}
			inbData["relay_to"] = relayObj
		}
		// users 一律走 clients 表多对多查,返回 client.name 字符串列表 ——
		// 包括 Basic Auth 协议(mixed/socks/http/naive)。前端用 length 显示
		// 客户数,统一 InboundClients modal 管理。
		if s.hasUser(inbound.Type) &&
			!(inbound.Type == "shadowtls" && shadowtls_version < 3) &&
			!(inbound.Type == "shadowsocks" && ss_managed) {
			users := []string{}
			err = db.Raw("SELECT clients.name FROM clients, json_each(clients.inbounds) as je WHERE je.value = ?", inbound.Id).Scan(&users).Error
			if err != nil {
				return nil, err
			}
			inbData["users"] = users
		}

		data = append(data, inbData)
	}
	return &data, nil
}

func (s *InboundService) FromIds(ids []uint) ([]*model.Inbound, error) {
	db := database.GetDB()
	inbounds := []*model.Inbound{}
	err := db.Model(model.Inbound{}).Where("id in ?", ids).Scan(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *InboundService) Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
	var err error

	// 兼容老格式 / xray-style JSON 粘贴:port 类字段 string→number、删 acme.key_type 等
	data = SanitizeRawConfig(data)

	switch act {
	case "new", "edit":
		var inbound model.Inbound
		err = inbound.UnmarshalJSON(data)
		if err != nil {
			return err
		}
		if inbound.TlsId > 0 {
			err = tx.Model(model.Tls{}).Where("id = ?", inbound.TlsId).Find(&inbound.Tls).Error
			if err != nil {
				return err
			}
			// 跟 GetAllConfig 同款兜底:DB 里历史 TLS 记录可能含 sing-box 1.13.5+
			// 已删的字段(acme.key_type 等),不在这里 sanitize 的话 MarshalJSON
			// 把脏 server 直接拼进 inboundConfig,corePtr.AddInbound 会被 strict-unmarshal
			// 拒("unknown field key_type")让 Save 整个失败。
			if inbound.Tls != nil {
				inbound.Tls.Server = SanitizeRawConfig(inbound.Tls.Server)
			}
		}
		var oldTag string
		if act == "edit" {
			err = tx.Model(model.Inbound{}).Select("tag").Where("id = ?", inbound.Id).Find(&oldTag).Error
			if err != nil {
				return err
			}
		}

		if corePtr.IsRunning() {
			if act == "edit" {
				// 编辑必须先把旧的 tag 摘掉(不论新状态是 enable 还是 disable),
				// 否则 sing-box 会保留两份监听冲突。Disable 的情况摘完就停。
				err = corePtr.RemoveInbound(oldTag)
				if err != nil && err != os.ErrInvalid {
					return err
				}
				// 同步断现有连接 — RemoveInbound 只是从 manager 注销 tag,
				// 已建立的 TCP/SOCKS 连接还在跑。不断的话客户端 keep-alive
				// reuse 旧连接,disable 后还能用,体感"开关无效"。
				corePtr.GetInstance().ConnTracker().CloseConnByInbound(oldTag)
			}

			if inbound.Enable {
				inboundConfig, err := inbound.MarshalJSON()
				if err != nil {
					return err
				}

				if act == "edit" {
					inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
				} else {
					inboundConfig, err = s.initUsers(tx, inboundConfig, initUserIds, inbound.Type)
				}
				if err != nil {
					return err
				}

				err = corePtr.AddInbound(inboundConfig)
				if err != nil {
					return err
				}
			}
		}

		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return err
		}

		err = tx.Save(&inbound).Error
		if err != nil {
			return err
		}
		switch act {
		case "new":
			err = s.ClientService.UpdateClientsOnInboundAdd(tx, initUserIds, inbound.Id, hostname)
		case "edit":
			err = s.ClientService.UpdateLinksByInboundChange(tx, &[]model.Inbound{inbound}, hostname, oldTag)
		}
		if err != nil {
			return err
		}
	case "del":
		var tag string
		err = json.Unmarshal(data, &tag)
		if err != nil {
			return err
		}
		if corePtr.IsRunning() {
			err = corePtr.RemoveInbound(tag)
			if err != nil && err != os.ErrInvalid {
				return err
			}
		}
		var id uint
		err = tx.Model(model.Inbound{}).Select("id").Where("tag = ?", tag).Scan(&id).Error
		if err != nil {
			return err
		}
		err = s.ClientService.UpdateClientsOnInboundDelete(tx, id, tag)
		if err != nil {
			return err
		}
		err = tx.Where("tag = ?", tag).Delete(model.Inbound{}).Error
		if err != nil {
			return err
		}
		// 清掉 stats 表里这个 tag 的累加流量样本 —— 否则用户重建同名 inbound
		// 时新流量会跟旧的混在一起累加(GetTotals 只按 tag 聚合),让"总流量"
		// 一开始就不是 0。
		if err = tx.Where("resource = ? AND tag = ?", "inbound", tag).Delete(&model.Stats{}).Error; err != nil {
			return err
		}
	default:
		return common.NewErrorf("unknown action: %s", act)
	}
	return nil
}

func (s *InboundService) UpdateOutJsons(tx *gorm.DB, inboundIds []uint, hostname string) error {
	var inbounds []model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ?", inboundIds).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		err = util.FillOutJson(&inbound, hostname)
		if err != nil {
			return err
		}
		err = tx.Model(model.Inbound{}).Where("tag = ?", inbound.Tag).Update("out_json", inbound.OutJson).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *InboundService) GetAllConfig(db *gorm.DB) ([]json.RawMessage, error) {
	var inboundsJson []json.RawMessage
	var inbounds []*model.Inbound
	// 仅下发启用的入站。enable 字段在 model.Inbound 上,由 UI 的 Switch 写入。
	err := db.Model(model.Inbound{}).Preload("Tls").Where("enable = ?", true).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		// 兼容老 DB:sing-box 1.13.5+ 多个字段被删/类型收紧 (acme.key_type 等),
		// reload 时见到这些字段会让 sing-box 起不来。Save 时已 sanitize,
		// 但 v1.7.4 时代写入的老数据可能从没经过 sanitize — 加载时再跑一次兜底,
		// 升级用户即使从未编辑过现有配置也能直接 reload 成功。
		if inbound.Tls != nil && len(inbound.Tls.Server) > 0 {
			inbound.Tls.Server = SanitizeRawConfig(inbound.Tls.Server)
		}
		inbound.Options = SanitizeRawConfig(inbound.Options)
		inboundJson, err := inbound.MarshalJSON()
		if err != nil {
			return nil, err
		}
		inboundJson, err = s.addUsers(db, inboundJson, inbound.Id, inbound.Type)
		if err != nil {
			return nil, err
		}
		inboundsJson = append(inboundsJson, inboundJson)
	}
	return inboundsJson, nil
}

func (s *InboundService) hasUser(inboundType string) bool {
	switch inboundType {
	case "mixed", "socks", "http", "shadowsocks", "vmess", "trojan", "naive", "hysteria", "shadowtls", "tuic", "hysteria2", "vless", "anytls":
		return true
	}
	return false
}

func (s *InboundService) fetchUsers(db *gorm.DB, inboundType string, condition string, inbound map[string]interface{}) ([]json.RawMessage, error) {
	if inboundType == "shadowtls" {
		version, _ := inbound["version"].(float64)
		if int(version) < 3 {
			return nil, nil
		}
	}
	if inboundType == "shadowsocks" {
		method, _ := inbound["method"].(string)
		if method == "2022-blake3-aes-128-gcm" {
			inboundType = "shadowsocks16"
		}
	}

	var users []string

	err := db.Raw(
		fmt.Sprintf(`SELECT json_extract(clients.config, "$.%s")
		FROM clients WHERE enable = true AND %s`,
			inboundType, condition)).Scan(&users).Error
	if err != nil {
		return nil, err
	}
	var usersJson []json.RawMessage
	for _, user := range users {
		if inboundType == "vless" && inbound["tls"] == nil {
			user = strings.Replace(user, "xtls-rprx-vision", "", -1)
		}
		usersJson = append(usersJson, json.RawMessage(user))
	}

	// 安全兜底:任何多账号协议在 users 数组为空时,sing-box 行为可能危险:
	//   - mixed/socks/http/naive:users=null → 无 Basic Auth 模式 → 任意账号都通 = 开放代理
	//   - vless/vmess/trojan 等 UUID 协议:users=null 通常 sing-box 会启动失败或拒绝所有连接
	// 这里统一塞一个哨兵账号(谁都猜不到的 64 字节随机密码 + 不可能的 UUID),
	// 让 sing-box 进入"有鉴权但没人能登"的状态 = 等价"全部 client disabled
	// 后端口实际不可用",安全且可观测(端口监听但连接全 reject)。
	if len(usersJson) == 0 {
		usersJson = []json.RawMessage{sentinelUserFor(inboundType)}
	}
	return usersJson, nil
}

// hasAnyEnabledUser 查这个 inbound 是否还有任何 enabled client 关联。
// 没有 = sing-box 不该监听这个端口(否则:mixed 变开放代理 / 其他协议任何
// 客户端都能 TCP 握手成功只是协议层失败 → 探测器看起来"还能用")。
func (s *InboundService) hasAnyEnabledUser(tx *gorm.DB, inboundType string, inboundId uint) bool {
	if !s.hasUser(inboundType) {
		return true // 无凭证概念的协议(direct/tun 等)不受此约束
	}
	var n int64
	cond := fmt.Sprintf("%d IN (SELECT json_each.value FROM json_each(clients.inbounds))", inboundId)
	if err := tx.Raw(fmt.Sprintf("SELECT COUNT(*) FROM clients WHERE enable=true AND %s", cond)).Scan(&n).Error; err != nil {
		return true // 查失败时放行,免误关
	}
	return n > 0
}

// sentinelUserFor 给指定协议生成一个不可能登录的"哨兵账号"。每次调用密码
// 都是新随机的,免被人通过共谋猜中。
func sentinelUserFor(inboundType string) json.RawMessage {
	rnd := func(n int) string {
		const alpha = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, n)
		_, _ = cryptorand.Read(b)
		for i := range b {
			b[i] = alpha[int(b[i])%len(alpha)]
		}
		return string(b)
	}
	pwd := rnd(64)
	uuid := fmt.Sprintf("%s-%s-%s-%s-%s", rnd(8), rnd(4), rnd(4), rnd(4), rnd(12))
	switch inboundType {
	case "mixed", "socks", "http", "naive":
		return json.RawMessage(fmt.Sprintf(`{"username":"__nb_sentinel__","password":%q}`, pwd))
	case "vless":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","uuid":%q}`, uuid))
	case "vmess":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","uuid":%q,"alterId":0}`, uuid))
	case "trojan", "anytls":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","password":%q}`, pwd))
	case "shadowsocks", "shadowsocks16", "shadowtls":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","password":%q}`, pwd))
	case "hysteria":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","auth_str":%q}`, pwd))
	case "hysteria2":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","password":%q}`, pwd))
	case "tuic":
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","uuid":%q,"password":%q}`, uuid, pwd))
	default:
		return json.RawMessage(fmt.Sprintf(`{"name":"__nb_sentinel__","password":%q}`, pwd))
	}
}

func (s *InboundService) addUsers(db *gorm.DB, inboundJson []byte, inboundId uint, inboundType string) ([]byte, error) {
	if !s.hasUser(inboundType) {
		return inboundJson, nil
	}
	// 所有多账号协议统一从 clients 表注入(包括 mixed/socks/http/naive 的
	// Basic Auth),走 fetchUsers — randomConfigs 已经为这些协议在
	// clients.config 里生成 {username, password},sing-box 拿到的就是
	// [{username, password}, ...] 数组,跟原生格式兼容。
	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	condition := fmt.Sprintf("%d IN (SELECT json_each.value FROM json_each(clients.inbounds))", inboundId)
	inbound["users"], err = s.fetchUsers(db, inboundType, condition, inbound)
	if err != nil {
		return nil, err
	}

	return json.Marshal(inbound)
}


func (s *InboundService) initUsers(db *gorm.DB, inboundJson []byte, clientIds string, inboundType string) ([]byte, error) {
	ClientIds := strings.Split(clientIds, ",")
	if len(ClientIds) == 0 {
		return inboundJson, nil
	}

	if !s.hasUser(inboundType) {
		return inboundJson, nil
	}

	var inbound map[string]interface{}
	err := json.Unmarshal(inboundJson, &inbound)
	if err != nil {
		return nil, err
	}

	condition := fmt.Sprintf("id IN (%s)", strings.Join(ClientIds, ","))
	inbound["users"], err = s.fetchUsers(db, inboundType, condition, inbound)
	if err != nil {
		return nil, err
	}

	return json.Marshal(inbound)
}

func (s *InboundService) RestartInbounds(tx *gorm.DB, ids []uint) error {
	if !corePtr.IsRunning() {
		return nil
	}
	var inbounds []*model.Inbound
	err := tx.Model(model.Inbound{}).Preload("Tls").Where("id in ?", ids).Find(&inbounds).Error
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		// 跟 Save / GetAllConfig 同款兜底:DB 历史 TLS 记录可能含 sing-box 1.13.5+
		// 已删字段(acme.key_type 等),不 sanitize 直接 MarshalJSON 喂给 sing-box
		// 会被 strict-unmarshal 拒("unknown field key_type"),client save 触发的
		// RestartInbounds 路径都会失败。
		if inbound.Tls != nil {
			inbound.Tls.Server = SanitizeRawConfig(inbound.Tls.Server)
		}
		err = corePtr.RemoveInbound(inbound.Tag)
		if err != nil && err != os.ErrInvalid {
			return err
		}
		// Close all existing connections
		corePtr.GetInstance().ConnTracker().CloseConnByInbound(inbound.Tag)

		// 入站本身被 disable 时不再 AddInbound — 否则 client 改动触发的
		// RestartInbounds 会"复活"已 disable 的入站(用户报:disable inbound
		// 后改动相关 client,端口又活了)。
		if !inbound.Enable {
			continue
		}

		inboundConfig, err := inbound.MarshalJSON()
		if err != nil {
			return err
		}
		inboundConfig, err = s.addUsers(tx, inboundConfig, inbound.Id, inbound.Type)
		if err != nil {
			return err
		}
		err = corePtr.AddInbound(inboundConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// buildInboundRelayMap 解析 setting.config.route.rules,把 _nb_binding 标记的
// 中转关系打包成 inboundTag → outboundTag map。GetAll 用它给入站列表加
// "中转目标" 列;前端无需再单独读一遍 setting.config 解析 rules。
//
// 跟 client.go::buildLinkRemarkCtx 的 InboundRelay 字段同款解析,这里独立一份
// 是为了 InboundService 不依赖 ClientService 的内部类型。
func buildInboundRelayMap() map[string]string {
	out := map[string]string{}
	cfgStr, err := (&SettingService{}).GetConfig()
	if err != nil || cfgStr == "" {
		return out
	}
	var cfg struct {
		Route struct {
			Rules []map[string]interface{} `json:"rules"`
		} `json:"route"`
	}
	if json.Unmarshal([]byte(cfgStr), &cfg) != nil {
		return out
	}
	for _, r := range cfg.Route.Rules {
		binding, _ := r["_nb_binding"].(bool)
		if !binding {
			continue
		}
		// action=direct 视为"本机出站",不计入中转
		if action, _ := r["action"].(string); action == "direct" {
			continue
		}
		ot, _ := r["outbound"].(string)
		if ot == "" {
			continue
		}
		inb, _ := r["inbound"].([]interface{})
		for _, it := range inb {
			if tag, ok := it.(string); ok && tag != "" {
				out[tag] = ot
			}
		}
	}
	return out
}

// buildOutboundDisplayMap 拉所有出站的 tag → display_name(中转名称)。
// 给 GetAll 给每个含中转关系的 inbound 行回填 outbound 的中转名称用。
// 跟 client.go::buildLinkRemarkCtx 复制一份是为了 InboundService 不依赖
// ClientService 的私有 ctx 类型。
//
// v1.7.26:订阅池出站独立到 pool_outbounds 表,这里两表 union(用户手配出站 +
// 订阅池 winner),tag 命名空间不重(pool- 前缀强制 + uniqueIndex)。
func buildOutboundDisplayMap() map[string]string {
	out := map[string]string{}
	var obs []model.Outbound
	if err := database.GetDB().Model(model.Outbound{}).Find(&obs).Error; err == nil {
		for _, ob := range obs {
			out[ob.Tag] = strings.TrimSpace(ob.DisplayName)
		}
	}
	var pools []model.PoolOutbound
	if err := database.GetDB().Model(model.PoolOutbound{}).Find(&pools).Error; err == nil {
		for _, po := range pools {
			out[po.Tag] = strings.TrimSpace(po.DisplayName)
		}
	}
	return out
}
