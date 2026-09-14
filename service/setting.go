package service

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var defaultConfig = `{
  "log": {
    "level": "info"
  },
  "dns": {
    "servers": [],
    "rules": []
  },
  "route": {
    "rules": [
		  {
        "action": "sniff"
      },
      {
        "protocol": [
          "dns"
        ],
        "action": "hijack-dns"
      }
    ]
  },
  "experimental": {}
}`

var defaultValueMap = map[string]string{
	"webListen":     "",
	"webDomain":     "",
	"webPort":       "3095",
	"secret":        common.Random(32),
	"webCertFile":   "",
	"webKeyFile":    "",
	"webPath":       "/app/",
	"webURI":        "",
	"sessionMaxAge": "0",
	// trafficAge:stats 流量明细的保留天数(DelStatsJob @daily 按它删)。
	// 30 → 7:节点本地的 stats 明细只喂面板图表 —— 计费走 clients.up/down
	// 累计列(主控读 /inbounds/:id/client-traffics,那条 SQL 只 SELECT clients),
	// 历史归档在主控的 proxy_host_traffic_dailies 按天存着。本地留 30 天
	// 纯属重复存储:实测一台 14 用户的节点 stats 就有 225 万行 / 162MB。
	// 0 表示关闭流量统计(既有语义,不要改)。
	"trafficAge":    "7",
	// apiLogAge:api_logs 保留天数。此前【没有任何自动清理】——
	// ApiLogService.PruneOlderThan 早就写好且注释写着"后台 cron 用",
	// 但全仓零调用方,于是只涨不减(实测 50171 行)。
	"apiLogAge":     "7",
	// changesAge:changes 变更审计的保留天数。它同时是前端增量刷新的判据
	// (CheckChanges 比 date_time > lu),删旧行只会让"没变化"更早成立,安全。
	// 审计价值比流量明细高,给的窗口也长一些。
	"changesAge":    "30",
	"timeLocation":  "UTC",
	"config":        defaultConfig,
	"version":       config.GetVersion(),
	// 节点名称:多机管理时给每台机一个 nickname,显示在前端侧边栏 logo
	// 下方,方便区分。空值时不显示。
	"nodeName": "",
	// 客户端分享链接 server 字段(add)的来源 — v1.7.23 三态:
	//   panel  (默认) — 用 panel webDomain / Host header(管理员能保证 DNS 通,且能套 CDN)
	//   ip            — 用 settings.panelIp(管理员手填的服务器公网 IP),raw TCP 协议绕开 CDN 时用
	//   tls           — 用 inbound.tls.server_name(签证书的域名;通配符 fallback hostname)
	// 入站可用 inbound.link_addr_source 单独覆盖此全局值(空则跟随全局)。
	"linkAddrSource": "panel",
	// panelIp:管理员手填的服务器公网 IP。给 linkAddrSource=ip 模式用 — 当
	// 出站 / 客户端 link 必须直连源 IP(绕 CDN)时,server 字段填这个。
	// 空时 fallback 到 hostname。
	"panelIp": "",
}

type SettingService struct {
}

func (s *SettingService) GetAllSetting() (*map[string]string, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := map[string]string{}

	for _, setting := range settings {
		allSetting[setting.Key] = setting.Value
	}

	for key, defaultValue := range defaultValueMap {
		if _, exists := allSetting[key]; !exists {
			err = s.saveSetting(key, defaultValue)
			if err != nil {
				return nil, err
			}
			allSetting[key] = defaultValue
		}
	}

	// Due to security principles
	delete(allSetting, "secret")
	delete(allSetting, "config")
	delete(allSetting, "version")
	// CF API Token 持久化但不在通用 settings 接口里下发到前端。
	// 前端用专门的 GetCfToken / SetCfToken 端点存取,降低误传到日志/导出的风险。
	delete(allSetting, "cf_api_token")
	delete(allSetting, "cf_acme_email")

	return &allSetting, nil
}

// GetCfToken / SetCfToken — Cloudflare API Token 持久化存储,供"自动签发"流程
// 复用,免得用户每次都重新粘贴一次 token。token 存 base64 仅做一层混淆 —
// 真正的安全是 DB 文件 owner-only 权限 + 面板登录鉴权。
func (s *SettingService) GetCfToken() (token, email string) {
	t, _ := s.getString("cf_api_token")
	if decoded, err := base64.StdEncoding.DecodeString(t); err == nil {
		t = string(decoded)
	}
	email, _ = s.getString("cf_acme_email")
	return strings.TrimSpace(t), strings.TrimSpace(email)
}

func (s *SettingService) SetCfToken(token, email string) error {
	enc := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(token)))
	if err := s.saveSetting("cf_api_token", enc); err != nil {
		return err
	}
	if email != "" {
		return s.saveSetting("cf_acme_email", strings.TrimSpace(email))
	}
	return nil
}

func (s *SettingService) ClearCfToken() error {
	if err := s.saveSetting("cf_api_token", ""); err != nil {
		return err
	}
	return s.saveSetting("cf_acme_email", "")
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.Setting{}).Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// saveSetting 用 SQLite 原生 UPSERT(ON CONFLICT(key) DO UPDATE),原子写入。
//
// AUDIT.md H5:旧实现是 select-then-write,两个并发 Save 同 key 时:
//
//	G1 select(no row)→ G2 select(no row)→ G1 insert → G2 insert
//	→ Setting.Key 现在有 UNIQUE 索引,G2 会被 DB 拦下;但若没索引就会插重。
//
// 改 UPSERT 后无论是否首次都是一条 SQL,GORM 用 clause.OnConflict 翻成
// `INSERT ... ON CONFLICT(key) DO UPDATE SET value=excluded.value`。
// 跟 model.Setting.Key 上的 uniqueIndex 配合,行级原子。
func (s *SettingService) saveSetting(key string, value string) error {
	db := database.GetDB()
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&model.Setting{Key: key, Value: value}).Error
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getBool(key string) (bool, error) {
	str, err := s.getString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

// func (s *SettingService) setBool(key string, value bool) error {
// 	return s.setString(key, strconv.FormatBool(value))
// }

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}
func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

// GetLinkAddrSource 返回客户端分享链接 server 字段的来源策略,三态:
// "panel"(默认) — 用 webDomain / Host
// "ip"           — 用 settings.panelIp(管理员手填的源服务器 IP)
// "tls"          — 用 inbound TLS server_name
// 入站可在 inbound.LinkAddrSource 单独覆盖此值。取错或空 fallback "panel"。
func (s *SettingService) GetLinkAddrSource() string {
	v, _ := s.getString("linkAddrSource")
	switch v {
	case "ip", "tls":
		return v
	}
	return "panel"
}

// GetPanelIp 返回管理员手填的服务器公网 IP(linkAddrSource=ip 模式专用)。
// 空则 LinkGenerator 会 fallback 到 hostname。不主动探测公网 IP — 多数云
// VM 内网网卡返内网 IP,自动探测会拿错值;由管理员显式填入最准。
func (s *SettingService) GetPanelIp() string {
	v, _ := s.getString("panelIp")
	return strings.TrimSpace(v)
}

// effectivePanelIp 缓存 + 自动探测,链接生成时用。
//
// 优先级:管理员手填 > 出网公网 IP 探测(向多个 echo-IP 服务并发查询)。
// 探测结果缓存 1 小时,避免每次生成链接都打公网请求。
//
// 设计原因:用户报"入站设了服务器 IP 模式但生成的链接还是 panel 域名" —
// 实际是用户没在 settings 里填 panelIp,GetPanelIp 返空字符串,LinkGenerator
// "ip" 分支 fallback 到 hostname。改成空时自动探一次,大多数情况能拿到正确公网 IP。
var (
	panelIpDetectedCache string
	panelIpDetectedAt    time.Time
	panelIpDetectMu      sync.Mutex
)

func (s *SettingService) EffectivePanelIp(cf interface{ DetectPublicIP() string }) string {
	if v := s.GetPanelIp(); v != "" {
		return v
	}
	if cf == nil {
		return ""
	}
	panelIpDetectMu.Lock()
	defer panelIpDetectMu.Unlock()
	if panelIpDetectedCache != "" && time.Since(panelIpDetectedAt) < time.Hour {
		return panelIpDetectedCache
	}
	ip := strings.TrimSpace(cf.DetectPublicIP())
	if ip == "" {
		return ""
	}
	panelIpDetectedCache = ip
	panelIpDetectedAt = time.Now()
	return ip
}

// GetNodeName 返回管理员在「设置」里配的节点名称(空则空字符串)。
// 客户端分享链接 ps / fragment 的拼接前缀用它(直连模式)。
func (s *SettingService) GetNodeName() string {
	v, _ := s.getString("nodeName")
	return strings.TrimSpace(v)
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

// SetNodeName 设置节点名称。
//
// 🩸 它不只是「侧边栏显示」—— 它是**分享链接 remark 的前缀**
// (见 client.go remarkPrefixFor:直连模式 = nodeName,空则 LinkGenerator
// 回退成 inbound.Tag)。没配的话用户在客户端里看到的线路名会是
// "nx-vless-reality-xxx" 这种内部 tag,而不是「东京 PCCW」这种可读名字。
//
// 允许置空:空 = 回到 tag 兜底,是一个合法状态,不是错误。
func (s *SettingService) SetNodeName(name string) error {
	return s.setString("nodeName", strings.TrimSpace(name))
}

func (s *SettingService) SetPort(port int) error {
	return s.setInt("webPort", port)
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) GetWebPath() (string, error) {
	webPath, err := s.getString("webPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return webPath, nil
}

func (s *SettingService) SetWebPath(webPath string) error {
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return s.setString("webPath", webPath)
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		err := s.saveSetting("secret", secret)
		if err != nil {
			logger.Warning("save secret failed:", err)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetTrafficAge() (int, error) {
	return s.getInt("trafficAge")
}

// GetApiLogAge 返回 api_logs 的保留天数。<=0 视为"不清理"。
func (s *SettingService) GetApiLogAge() (int, error) {
	return s.getInt("apiLogAge")
}

// GetChangesAge 返回 changes 变更审计的保留天数。<=0 视为"不清理"。
func (s *SettingService) GetChangesAge() (int, error) {
	return s.getInt("changesAge")
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	l, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		l = "Local"
	}
	location, err := time.LoadLocation(l)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", l, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) GetConfig() (string, error) {
	return s.getString("config")
}

func (s *SettingService) SetConfig(config string) error {
	return s.setString("config", config)
}

func (s *SettingService) SaveConfig(tx *gorm.DB, config json.RawMessage) error {
	configs, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return tx.Model(model.Setting{}).Where("key = ?", "config").Update("value", string(configs)).Error
}

func (s *SettingService) Save(tx *gorm.DB, data json.RawMessage) error {
	var err error
	var settings map[string]string
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	for key, obj := range settings {
		// Secure file existence check
		if obj != "" && (key == "webCertFile" ||
			key == "webKeyFile") {
			err = s.fileExists(obj)
			if err != nil {
				return common.NewError(" -> ", obj, " is not exists")
			}
		}

		// Correct Pathes start and ends with `/`
		if key == "webPath" {
			if !strings.HasPrefix(obj, "/") {
				obj = "/" + obj
			}
			if !strings.HasSuffix(obj, "/") {
				obj += "/"
			}
		}

		// Delete all stats if it is set to 0
		if key == "trafficAge" && obj == "0" {
			err = tx.Where("id > 0").Delete(model.Stats{}).Error
			if err != nil {
				return err
			}
		}
		err = tx.Model(model.Setting{}).Where("key = ?", key).Update("value", obj).Error
		if err != nil {
			return err
		}
	}
	return err
}

func (s *SettingService) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}
