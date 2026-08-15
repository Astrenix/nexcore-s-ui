package model

import (
	"encoding/json"
)

type Inbound struct {
	Id   uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Type string `json:"type" form:"type"`
	Tag  string `json:"tag" form:"tag" gorm:"unique"`

	// Enable 控制此入站是否在 sing-box config 里下发。默认 true(GORM
	// AutoMigrate 给已有行回填 default 值);UI 上的 switch 开关写入此字段。
	Enable bool `json:"enable" form:"enable" gorm:"default:true"`

	// Foreign key to tls table.
	// *uint(nullable):无 TLS 的入站存 NULL 而非 0 —— 这样开启 SQLite 外键
	// (_foreign_keys=on)后,FK 对 NULL 不校验、对非空值要求 tls.id 真实存在,
	// 既拦住孤儿引用又不误伤无 TLS 入站。历史上用 0 表示"无 TLS",而 tls 表没有
	// id=0 的行,开 FK 会让所有无 TLS 入站 Save 报错(第一轮因此回退)。
	TlsId *uint `json:"tls_id" form:"tls_id"`
	Tls   *Tls  `json:"tls" form:"tls" gorm:"foreignKey:TlsId;references:Id;constraint:OnDelete:RESTRICT"`

	Addrs   json.RawMessage `json:"addrs" form:"addrs"`
	OutJson json.RawMessage `json:"out_json" form:"out_json"`
	// LinkAddrSource — 入站级覆盖全局 settings.linkAddrSource。
	//   ""    跟随全局(默认)
	//   panel 用 panel webDomain / Host
	//   ip    用 settings.panelIp
	//   tls   用 inbound.tls.server_name
	// 不下发给 sing-box(MarshalJSON 不输出),只走前端 LoadData / LinkGenerator。
	LinkAddrSource string          `json:"link_addr_source,omitempty" form:"link_addr_source" gorm:"size:16"`
	Options        json.RawMessage `json:"-" form:"-"`

	// Ext 存自定义元数据(JSON 字符串),跟 sing-box 配置无关。当前用于 Basic
	// Auth 协议的 per-cred 流量/到期限制:
	//   {"creds":{"<username>":{"volume_limit":N,"expiry":N,"enable":true}}}
	// cronjob/inboundLimitJob 周期检查 stats + 当前时间,超限/过期的 username
	// 改 enable=false,重建 Options.users(过滤掉 disabled 的),sing-box reload。
	// 注:用 string 而不是 json.RawMessage —— GORM 对 SQLite 的 RawMessage
	// + default tag 组合 Scan 失败(driver.Value=string 无法存进 *json.RawMessage)
	Ext string `json:"-" form:"-"`
}

func (i *Inbound) UnmarshalJSON(data []byte) error {
	var err error
	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract fixed fields and store the rest in Options
	if val, exists := raw["id"].(float64); exists {
		i.Id = uint(val)
	}
	delete(raw, "id")
	i.Type, _ = raw["type"].(string)
	delete(raw, "type")
	i.Tag, _ = raw["tag"].(string)
	delete(raw, "tag")

	// TlsId:前端仍以数字 tls_id 传(0 = 无 TLS)。0 存 nil(→ DB NULL),
	// >0 存指针,配合外键约束。
	if val, exists := raw["tls_id"].(float64); exists {
		if id := uint(val); id > 0 {
			i.TlsId = &id
		} else {
			i.TlsId = nil
		}
	}
	delete(raw, "tls_id")
	delete(raw, "tls")
	// users 字段统一不持久化到 inbound.Options —— 所有"多账号"协议(包括
	// mixed/socks/http/naive 的 Basic Auth)都走 clients 表多对多关联。
	// 下发 sing-box 时由 service.addUsers 从 clients.config[type] 注入。
	delete(raw, "users")

	// Enable - 缺省视为 true(老数据无此字段时不影响行为)
	if val, exists := raw["enable"]; exists {
		if b, ok := val.(bool); ok {
			i.Enable = b
		} else {
			i.Enable = true
		}
		delete(raw, "enable")
	} else {
		i.Enable = true
	}

	// Addrs
	i.Addrs, _ = json.MarshalIndent(raw["addrs"], "", "  ")
	delete(raw, "addrs")

	// OutJson
	i.OutJson, _ = json.MarshalIndent(raw["out_json"], "", "  ")
	delete(raw, "out_json")

	// Ext (per-cred 限制元数据)— 不参与 sing-box 配置,只持久化我们的元信息
	if v, ok := raw["ext"]; ok && v != nil {
		if b, err := json.Marshal(v); err == nil {
			i.Ext = string(b)
		}
	}
	delete(raw, "ext")

	// LinkAddrSource — 前端字段,不下发给 sing-box;从 raw 抽出免得被塞进 Options
	if v, ok := raw["link_addr_source"].(string); ok {
		i.LinkAddrSource = v
	}
	delete(raw, "link_addr_source")

	// Remaining fields
	i.Options, err = json.MarshalIndent(raw, "", "  ")
	return err
}

// HasTls 该入站是否绑定了 TLS(tls_id 非空且 >0)。
func (i Inbound) HasTls() bool {
	return i.TlsId != nil && *i.TlsId > 0
}

// TlsIdValue 返回 tls_id 的数值,未绑定 TLS 时返回 0 —— 用于前端契约(前端仍以
// 数字 0 表示无 TLS)和按 id 查询。
func (i Inbound) TlsIdValue() uint {
	if i.TlsId == nil {
		return 0
	}
	return *i.TlsId
}

// MarshalJSON customizes marshalling
func (i Inbound) MarshalJSON() ([]byte, error) {
	// Combine fixed fields and dynamic fields into one map
	combined := make(map[string]interface{})
	combined["type"] = i.Type
	combined["tag"] = i.Tag
	if i.Tls != nil {
		combined["tls"] = i.Tls.Server
	}

	if i.Options != nil {
		var restFields map[string]json.RawMessage
		if err := json.Unmarshal(i.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			combined[k] = v
		}
	}

	return json.Marshal(combined)
}

func (i Inbound) MarshalFull() (*map[string]interface{}, error) {
	combined := make(map[string]interface{})
	combined["id"] = i.Id
	combined["type"] = i.Type
	combined["tag"] = i.Tag
	combined["enable"] = i.Enable
	combined["tls_id"] = i.TlsIdValue()
	combined["addrs"] = i.Addrs
	combined["out_json"] = i.OutJson
	if i.Ext != "" {
		var extObj interface{}
		if err := json.Unmarshal([]byte(i.Ext), &extObj); err == nil {
			combined["ext"] = extObj
		}
	}

	if i.Options != nil {
		var restFields map[string]interface{}
		if err := json.Unmarshal(i.Options, &restFields); err != nil {
			return nil, err
		}

		for k, v := range restFields {
			combined[k] = v
		}
	}
	return &combined, nil
}
