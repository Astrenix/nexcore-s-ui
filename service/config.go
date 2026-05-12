package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

// AUDIT.md H3:lastStartFailTime / startCoreInProgress 之前是裸变量 + 部分位置漏锁,
// race detector 跑就报 data race。改成 atomic 一族,CompareAndSwap 替代手写互斥,
// 语义不变但消除并发竞争(panel API 多并发触发 RestartCore 时直接体现)。
var (
	LastUpdate          int64
	corePtr             *core.Core
	startCoreInProgress atomic.Bool  // 串行化 Start / restartCoreWithConfig
	lastStartFailNano   atomic.Int64 // time.UnixNano(),0 = 从未失败过
	startCooldown       = 15 * time.Second
)

type ConfigService struct {
	ClientService
	TlsService
	SettingService
	InboundService
	OutboundService
	EndpointService
	BlockRuleService
	SubService
}

// ensureBuiltinOutbounds 保证最终 config 至少包含 tag=direct 出站。
// rule-set 默认 `download_detour: "direct"`、DNS 把 geoip-cn / 私网回 direct,
// 一旦用户把 direct 删了或空 DB 状态,sing-box 启动直接报
// "download detour not found: direct"。缺则在末尾补一个最简 direct。
//
// 不自动注 block —— 新版 sing-box 已废弃 block outbound 类型,改用路由 action=reject。
func ensureBuiltinOutbounds(outbounds []json.RawMessage) []json.RawMessage {
	for _, o := range outbounds {
		var meta struct{ Tag string }
		if err := json.Unmarshal(o, &meta); err == nil && meta.Tag == "direct" {
			return outbounds
		}
	}
	return append(outbounds, json.RawMessage(`{"type":"direct","tag":"direct"}`))
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Endpoints    []json.RawMessage `json:"endpoints"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService(core *core.Core) *ConfigService {
	corePtr = core
	return &ConfigService{}
}

func (s *ConfigService) GetConfig(data string) (*[]byte, error) {
	var err error
	if len(data) == 0 {
		data, err = s.SettingService.GetConfig()
		if err != nil {
			return nil, err
		}
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal([]byte(data), &singboxConfig)
	if err != nil {
		return nil, err
	}

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	// 合并订阅池出站(pool_outbounds 表 — 跟用户手配的 outbounds 表分离,
	// sing-box 看到的是 union 后的完整列表,内部表分布对 core 透明)。
	// pool_outbounds 由 SubService.ElectWinners 自动维护,tag 形如 pool-{cc},
	// 不重名(uniqueIndex)。
	poolOutConfigs, err := s.SubService.GetPoolOutboundConfigs()
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds = append(singboxConfig.Outbounds, poolOutConfigs...)
	// 兜底:必须存在 `direct` 出站。
	// rule-set 默认 `download_detour: "direct"`,DNS 默认规则也会把私网 / geoip-cn
	// 路由到 direct;一旦用户把 direct 出站删了或从未建过(空 DB 状态),
	// sing-box 启动会立刻挂在 "download detour not found: direct"。
	// 这里在生成最终 config 时强制保证 direct/block 两个出站存在,缺则补。
	singboxConfig.Outbounds = ensureBuiltinOutbounds(singboxConfig.Outbounds)
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	// 把 BlockRule 表里 enable=true 的行翻译成 reject route.rule,prepend 到
	// route.rules 最前面 — 优先级高于用户在「路由列表」里手编的规则,
	// 命中即 reject。详见 database/model/block_rule.go 的模块边界说明。
	singboxConfig.Route = injectBlockRules(singboxConfig.Route)
	rawConfig, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	// 下发给 sing-box 前剥离前端 metadata(_nb_binding 等)。setting.config 里
	// 保留(前端业务依赖)— 仅在生成 sing-box 入参的瞬间清洗。
	rawConfig = StripDownstreamFields(rawConfig)
	return &rawConfig, nil
}

func (s *ConfigService) StartCore() error {
	if corePtr.IsRunning() {
		return nil
	}
	// CAS 序列化:两个并发 StartCore 只允许一个真正进 — 另一个直接返回。
	if !startCoreInProgress.CompareAndSwap(false, true) {
		return nil
	}
	defer startCoreInProgress.Store(false)

	if last := lastStartFailNano.Load(); last > 0 && time.Since(time.Unix(0, last)) < startCooldown {
		logger.Info("start core cooldown ", int64(startCooldown/time.Second), " seconds")
		return nil
	}

	logger.Info("starting core")
	rawConfig, err := s.GetConfig("")
	if err != nil {
		return err
	}
	if err := corePtr.Start(*rawConfig); err != nil {
		lastStartFailNano.Store(time.Now().UnixNano())
		logger.Error("start sing-box err:", err.Error())
		return err
	}
	logger.Info("sing-box started")
	return nil
}

func (s *ConfigService) RestartCore() error {
	err := s.StopCore()
	if err != nil {
		return err
	}
	return s.StartCore()
}

func (s *ConfigService) restartCoreWithConfig(config json.RawMessage) error {
	if !startCoreInProgress.CompareAndSwap(false, true) {
		return nil
	}
	defer startCoreInProgress.Store(false)

	if corePtr.IsRunning() {
		if err := corePtr.Stop(); err != nil {
			logger.Error("restart sing-box err (stop):", err.Error())
			return err
		}
	}
	rawConfig, err := s.GetConfig(string(config))
	if err != nil {
		logger.Error("restart sing-box err (get config):", err.Error())
		return err
	}
	if err := corePtr.Start(*rawConfig); err != nil {
		lastStartFailNano.Store(time.Now().UnixNano())
		logger.Error("restart sing-box err (start):", err.Error())
		return err
	}
	logger.Info("sing-box restarted with new config")
	return nil
}

func (s *ConfigService) StopCore() error {
	err := corePtr.Stop()
	if err != nil {
		return err
	}
	logger.Info("sing-box stopped")
	return nil
}

func (s *ConfigService) CheckOutbound(tag string, link string) core.CheckOutboundResult {
	if tag == "" {
		return core.CheckOutboundResult{Error: "missing query parameter: tag"}
	}
	if corePtr == nil || !corePtr.IsRunning() {
		return core.CheckOutboundResult{Error: "core not running"}
	}
	return core.CheckOutbound(corePtr.GetCtx(), tag, link)
}

func (s *ConfigService) Save(obj string, act string, data json.RawMessage, initUsers string, loginUser string, hostname string) ([]string, error) {
	var err error
	var objs []string = []string{obj}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
			// Try to start core if it is not running
			if !corePtr.IsRunning() {
				s.StartCore()
			}
		} else {
			tx.Rollback()
		}
	}()

	switch obj {
	case "clients":
		var inboundIds []uint
		inboundIds, err = s.ClientService.Save(tx, act, data, hostname)
		if err == nil && len(inboundIds) > 0 {
			objs = append(objs, "inbounds")
			err = s.InboundService.RestartInbounds(tx, inboundIds)
			if err != nil {
				return nil, common.NewErrorf("failed to update users for inbounds: %v", err)
			}
		}
	case "tls":
		err = s.TlsService.Save(tx, act, data, hostname)
		objs = append(objs, "clients", "inbounds")
	case "inbounds":
		err = s.InboundService.Save(tx, act, data, initUsers, hostname)
		objs = append(objs, "clients")
	case "outbounds":
		err = s.OutboundService.Save(tx, act, data)
	case "endpoints":
		err = s.EndpointService.Save(tx, act, data)
	case "block-rules":
		// 屏蔽规则改完需要 reload sing-box 让 route.rules 注入立即生效。
		// 跟 case "config" 同样异步 restart,Save 不阻塞返回。
		err = s.BlockRuleService.Save(tx, act, data)
		if err == nil {
			go func() { _ = s.RestartCore() }()
		}
	case "config":
		// 整段 config JSON 也跑 sanitize:用户从 xray/v2ray 等粘贴整份配置时
		// 常见 server_port=string、acme.key_type 等不兼容字段。
		data = SanitizeRawConfig(data)
		err = s.SettingService.SaveConfig(tx, data)
		if err != nil {
			return nil, err
		}
		configData := make(json.RawMessage, len(data))
		copy(configData, data)
		go func() { _ = s.restartCoreWithConfig(configData) }()
	case "settings":
		err = s.SettingService.Save(tx, data)
	default:
		return nil, common.NewError("unknown object: ", obj)
	}
	if err != nil {
		return nil, err
	}

	dt := time.Now().Unix()
	err = tx.Create(&model.Changes{
		DateTime: dt,
		Actor:    loginUser,
		Key:      obj,
		Action:   act,
		Obj:      data,
	}).Error
	if err != nil {
		return nil, err
	}

	LastUpdate = time.Now().Unix()

	return objs, nil
}

// CoreInstance 暴露 core.Box 实例给 api 层 — 例如查询当前活跃连接数。
// 核心未运行时返回 nil。
func (s *ConfigService) CoreInstance() *core.Box {
	if corePtr == nil || !corePtr.IsRunning() {
		return nil
	}
	return corePtr.GetInstance()
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	if LastUpdate == 0 {
		db := database.GetDB()
		var count int64
		err := db.Model(model.Changes{}).Where("date_time > " + lu).Count(&count).Error
		if err == nil {
			LastUpdate = time.Now().Unix()
		}
		return count > 0, err
	} else {
		intLu, err := strconv.ParseInt(lu, 10, 64)
		return LastUpdate > intLu, err
	}
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	whereString := "`id`>0"
	if len(actor) > 0 {
		whereString += " and `actor`='" + actor + "'"
	}
	if len(chngKey) > 0 {
		whereString += " and `key`='" + chngKey + "'"
	}
	db := database.GetDB()
	var chngs []model.Changes
	err := db.Model(model.Changes{}).Where(whereString).Order("`id` desc").Limit(c).Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}

// injectBlockRules 把 model.BlockRule 表里 enable=true 的行翻译成 sing-box
// route.rule(action=reject),prepend 到 route.rules 数组最前面。
//
// 失败兜底:任何一步出错都返回原 routeRaw 不动,只 Warning。reload sing-box
// 时即使 BlockRule 注入失败,基础 route.rules 仍能让 core 正常起来。
func injectBlockRules(routeRaw json.RawMessage) json.RawMessage {
	var blockRules []model.BlockRule
	if err := database.GetDB().Where("enable = ?", true).Order("id ASC").Find(&blockRules).Error; err != nil {
		logger.Warning("[block-rules] load failed:", err)
		return routeRaw
	}
	if len(blockRules) == 0 {
		return routeRaw
	}
	if len(routeRaw) == 0 || string(routeRaw) == "null" {
		routeRaw = json.RawMessage(`{"rules":[]}`)
	}
	var routeMap map[string]any
	if err := json.Unmarshal(routeRaw, &routeMap); err != nil {
		logger.Warning("[block-rules] parse route failed:", err)
		return routeRaw
	}
	managed := make([]any, 0, len(blockRules))
	for _, br := range blockRules {
		if rule := blockRuleToRouteRule(br); rule != nil {
			managed = append(managed, rule)
		}
	}
	if len(managed) == 0 {
		return routeRaw
	}
	var existing []any
	if r, ok := routeMap["rules"].([]any); ok {
		existing = r
	}
	routeMap["rules"] = append(managed, existing...)
	out, err := json.Marshal(routeMap)
	if err != nil {
		logger.Warning("[block-rules] marshal route failed:", err)
		return routeRaw
	}
	return out
}

// blockRuleToRouteRule 把单条 BlockRule 翻成 sing-box route.rule(map 形式)。
// type 不在白名单 / value 解析失败 → 返回 nil 跳过该条(不让一条坏规则导致
// reload 整体失败)。
//
// 翻译表:
//   domain    → domain_suffix(覆盖更广,匹配 x-ui 的"包含子域"行为)
//   ip        → ip_cidr
//   geosite   → geosite
//   geoip     → geoip
//   port      → port (数字数组)
//   protocol  → protocol(tls/http/quic ... sing-box sniff 后的 protocol 名)
//   source    → source_ip_cidr
func blockRuleToRouteRule(br model.BlockRule) map[string]any {
	values := splitTrim(br.Value, ",")
	if len(values) == 0 {
		return nil
	}
	rule := map[string]any{"action": "reject"}
	switch br.Type {
	case "domain":
		rule["domain_suffix"] = values
	case "ip":
		rule["ip_cidr"] = values
	case "geosite":
		rule["geosite"] = values
	case "geoip":
		rule["geoip"] = values
	case "port":
		ports := make([]int, 0, len(values))
		for _, v := range values {
			if n, err := strconv.Atoi(v); err == nil {
				ports = append(ports, n)
			}
		}
		if len(ports) == 0 {
			return nil
		}
		rule["port"] = ports
	case "protocol":
		rule["protocol"] = values
	case "source":
		rule["source_ip_cidr"] = values
	default:
		return nil
	}
	if br.InboundTag != "" {
		rule["inbound"] = []string{br.InboundTag}
	}
	return rule
}

func splitTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
