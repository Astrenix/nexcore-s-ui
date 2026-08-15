package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubService 订阅源 + 节点池 + Winner 选举,围绕 model.Sub / model.SubNode 两张表。
//
// 核心动作:
//   - RefreshSub:fetch URL → parse URI → probe(exit IP + 延迟) → upsert sub_nodes
//   - ElectWinners:跨所有订阅按 country 分组挑最低延迟 → 写 outbound `pool-{cc}`
//   - CheckWinners:5min 巡检当前 winner 是否还活;死了立刻 re-elect
//
// 并发安全:RefreshSub / ElectWinners / CheckWinners 三者用单一 mutex 串行化,
// 避免 cron + 手动刷新撞车 + 探测时大量 AddOutbound 冲掉 sing-box 的 outbound_manager。
type SubService struct {
}

var (
	subOpsMu sync.Mutex
)

// CountryPoolTagPrefix outbound.tag 命名前缀,所有"国家池"出站都以此开头。
// 前端 / 入站编辑识别"订阅池绑定"也按此前缀。
const CountryPoolTagPrefix = "pool-"

// ----- CRUD -----

func (s *SubService) List() ([]model.Sub, error) {
	var subs []model.Sub
	err := database.GetDB().Order("id ASC").Find(&subs).Error
	return subs, err
}

func (s *SubService) Get(id uint) (*model.Sub, error) {
	var sub model.Sub
	err := database.GetDB().First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *SubService) Create(sub *model.Sub) error {
	if sub.URL == "" {
		return errors.New("url 必填")
	}
	if sub.Name == "" {
		sub.Name = sub.URL
	}
	if sub.RefreshInterval <= 0 {
		sub.RefreshInterval = 60
	}
	return database.GetDB().Create(sub).Error
}

func (s *SubService) Update(sub *model.Sub) error {
	if sub.Id == 0 {
		return errors.New("id 必填")
	}
	return database.GetDB().Model(&model.Sub{}).Where("id = ?", sub.Id).
		Updates(map[string]any{
			"name":             sub.Name,
			"url":              sub.URL,
			"enable":           sub.Enable,
			"refresh_interval": sub.RefreshInterval,
		}).Error
}

// Delete 删订阅 + 级联清理:
//  1. 删该订阅的全部 sub_nodes
//  2. 删订阅本身
//  3. 重选 winners(剩余 sub 还有节点的国家,winner 切到那)
//  4. 孤儿 pool_outbound 清理:某国家的 sub_nodes 已经一个都不剩 → 删对应 pool-{cc}
//     (避免 inbound 绑了死 tag 一直跑不通,DB 也越攒越多失效行)
func (s *SubService) Delete(id uint) error {
	subOpsMu.Lock()
	defer subOpsMu.Unlock()
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sub_id = ?", id).Delete(&model.SubNode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Sub{}, id).Error
	}); err != nil {
		return err
	}
	// 重选所有 winners(其他 sub 可能还有同国家节点)
	if err := s.ElectWinners(); err != nil {
		logger.Warning("Delete: ElectWinners failed: ", err)
	}
	// 删孤儿:pool-{cc} 但 sub_nodes 已经没该国家任何节点
	return s.cleanupOrphanPoolOutbounds()
}

// cleanupOrphanPoolOutbounds 删掉所有"剩余 sub_nodes 已不包含该国家"的 pool_outbound 行。
// 决策:不是"没活节点"就删(那种保留旧 winner 是用户规则),是"压根连死节点都没了"才删。
// 触发场景:用户删了最后一个有该国家节点的订阅源 → 国家彻底从池里消失。
func (s *SubService) cleanupOrphanPoolOutbounds() error {
	var pools []model.PoolOutbound
	if err := database.GetDB().Find(&pools).Error; err != nil {
		return err
	}
	for _, po := range pools {
		var cnt int64
		database.GetDB().Model(&model.SubNode{}).
			Where("country = ?", po.Country).Count(&cnt)
		if cnt == 0 {
			// 先从 sing-box 摘掉,再删 DB(摘晚了 cron 还能撞到 ghost outbound)
			if corePtr != nil && corePtr.IsRunning() {
				_ = corePtr.RemoveOutbound(po.Tag)
			}
			if err := database.GetDB().Delete(&model.PoolOutbound{}, po.Id).Error; err != nil {
				logger.Warning("cleanupOrphan: delete ", po.Tag, " failed: ", err)
				continue
			}
			logger.Info("cleanupOrphan: 删孤儿 ", po.Tag, "(该国家已无任何节点)")
		}
	}
	return nil
}

// ResetAll 一键清空订阅池整套数据:
//   - 全部 subs(订阅源)
//   - 全部 sub_nodes(节点池)
//   - 全部 pool_outbounds(国家池出站)
//
// 用户场景:订阅池数据脏了 / 想重新来一遍 / 准备转换机场源。前端有二次确认。
// 不动用户手配的 outbounds 表,也不动 inbounds — inbound 如果之前绑了 pool-* tag,
// 重置后那个 tag 没了,sing-box 转发会失败(转给 direct),需要用户自己重绑。
func (s *SubService) ResetAll() error {
	subOpsMu.Lock()
	defer subOpsMu.Unlock()
	// 先从 sing-box 摘掉所有 pool-* 出站(防止 Reset 后 cron 跑空 outbound 喷错误)
	if corePtr != nil && corePtr.IsRunning() {
		var pools []model.PoolOutbound
		_ = database.GetDB().Find(&pools).Error
		for _, po := range pools {
			_ = corePtr.RemoveOutbound(po.Tag)
		}
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM pool_outbounds").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM sub_nodes").Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM subs").Error
	})
}

// ----- 刷新 + 探测 -----

// RefreshSub 单订阅刷新流程:
//  1. fetch URL → parse URI → 得到 N 条候选
//  2. probe 候选 → 得到 exit IP / 延迟 / country
//  3. upsert sub_nodes(同 sub 下按 server:port 唯一);本轮未出现的节点删
//  4. 更新 sub.last_synced_at / last_status / last_error
//  5. 触发 ElectWinners 重选所有国家 winner
//
// 错误处理:
//   - fetch / parse 整体失败 → 不动 sub_nodes(保留上次结果,避免临时网抖把整池洗空)
//   - 单条 probe 失败 → 该条标 alive=false,但行保留 → CheckWinners 时仍可参考其他活的
func (s *SubService) RefreshSub(ctx context.Context, id uint) (*RefreshResult, error) {
	subOpsMu.Lock()
	defer subOpsMu.Unlock()

	sub, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	rr := &RefreshResult{SubId: id}

	nodes, okParsed, totalCands, err := FetchSub(sub.URL)
	if err != nil {
		s.markSubStatus(id, "failed", err.Error(), 0)
		rr.Error = err.Error()
		return rr, err
	}
	rr.Total = totalCands
	rr.Parsed = okParsed

	// 探测(并发 8)
	outcomes := ProbeNodes(ctx, nodes)
	aliveCount := 0
	for _, o := range outcomes {
		if o.Alive {
			aliveCount++
		}
	}
	rr.Alive = aliveCount

	// upsert + 删除本次未出现的
	if err := s.applyOutcomes(id, outcomes); err != nil {
		s.markSubStatus(id, "failed", err.Error(), aliveCount)
		rr.Error = err.Error()
		return rr, err
	}

	s.markSubStatus(id, "ok", "", aliveCount)
	rr.OK = true

	// 重选 winners(包括其他订阅的国家)
	if err := s.ElectWinners(); err != nil {
		logger.Warning("ElectWinners after RefreshSub: ", err)
	}
	return rr, nil
}

// RefreshResult 单次刷新的 summary,API + cron log 用。
type RefreshResult struct {
	SubId  uint   `json:"sub_id"`
	Total  int    `json:"total"`  // 订阅里候选行数
	Parsed int    `json:"parsed"` // 成功解析的链接数
	Alive  int    `json:"alive"`  // 探测存活数
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// applyOutcomes:
//   - 同 sub_id + server + server_port 已存在 → UPDATE remark/type/options/country/exit_ip/latency/alive/last_check_at
//   - 不存在 → INSERT
//   - 本次未出现在订阅里的旧节点 → DELETE(per 用户规则 + 闸:仅当 outcomes 非空)
func (s *SubService) applyOutcomes(subId uint, outcomes []ProbeOutcome) error {
	if len(outcomes) == 0 {
		return errors.New("no outcomes to apply")
	}
	now := time.Now()
	// 组装本轮全部行 + 收集(server,port)白名单,避免原先"每节点先 First 再 Save"
	// 的 N+1 与"全表 Find 后逐条 Delete"的第二次 N+1 —— 大订阅刷新时这是一个
	// 长写事务持锁的主因,会跟 StatsJob/DepleteJob 抢 SQLite 写锁触发 busy。
	rows := make([]model.SubNode, 0, len(outcomes))
	servers := make([]string, 0, len(outcomes))
	ports := make([]uint16, 0, len(outcomes))
	seen := make(map[string]bool, len(outcomes))
	for _, o := range outcomes {
		key := fmt.Sprintf("%s:%d", o.Node.Server, o.Node.ServerPort)
		if seen[key] {
			continue // 同订阅内重复 server:port,去重避免 upsert 冲突
		}
		seen[key] = true
		rows = append(rows, model.SubNode{
			SubId:       subId,
			Remark:      o.Node.Remark,
			Type:        o.Node.Type,
			Server:      o.Node.Server,
			ServerPort:  o.Node.ServerPort,
			Options:     o.Node.Options,
			Country:     o.Country,
			ExitIP:      o.ExitIP,
			LatencyMs:   o.LatencyMs,
			Alive:       o.Alive,
			LastError:   o.Error,
			LastCheckAt: now,
		})
		servers = append(servers, o.Node.Server)
		ports = append(ports, o.Node.ServerPort)
	}

	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		// 批量 upsert:命中 idx_sub_host(sub_id,server,server_port)唯一索引就更新探测结果,
		// 保留 id / 自增字段不动。分批以避开 SQLite 999 变量上限。
		const batch = 100
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sub_id"}, {Name: "server"}, {Name: "server_port"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"remark", "type", "options", "country", "exit_ip",
				"latency_ms", "alive", "last_error", "last_check_at",
			}),
		}).CreateInBatches(rows, batch).Error; err != nil {
			return err
		}

		// 删除本轮未出现的旧节点:一条 NOT IN 批量删,替代原来的全表扫 + 逐条 Delete。
		// SQLite 无原生元组 IN,用 server||':'||port 拼 key 比对本轮白名单。
		keys := make([]string, 0, len(rows))
		for i := range servers {
			keys = append(keys, fmt.Sprintf("%s:%d", servers[i], ports[i]))
		}
		del := tx.Where("sub_id = ?", subId)
		if len(keys) > 0 {
			del = del.Where("server || ':' || server_port NOT IN ?", keys)
		}
		return del.Delete(&model.SubNode{}).Error
	})
}

func (s *SubService) markSubStatus(id uint, status, errMsg string, nodeCount int) {
	updates := map[string]any{
		"last_synced_at":  time.Now(),
		"last_status":     status,
		"last_error":      errMsg,
		"last_node_count": nodeCount,
	}
	database.GetDB().Model(&model.Sub{}).Where("id = ?", id).Updates(updates)
}

// ----- Winner 选举 + 巡检 -----

// ElectWinners 跨所有订阅按 country 分组,挑延迟最低的 alive 节点作 winner,
// 写入 **pool_outbounds 表**(独立于用户手配的 outbounds 表)。
//
// 表分离原因:用户手配的常驻 outbound 是稳定资产,订阅池 winner 是瞬时计算物,
// 混表会污染出站管理 UI;sing-box config 渲染时 ConfigService.GetConfig 合并两表。
//
// 池空(某 country 一个活节点都没)→ 保留旧 pool_outbound 不动 (per 用户规则),
// 前端通过 /api/subPools 标红"无可用节点"。
func (s *SubService) ElectWinners() error {
	var nodes []model.SubNode
	err := database.GetDB().Where("alive = ?", true).Order("country ASC, latency_ms ASC").
		Find(&nodes).Error
	if err != nil {
		return err
	}
	// 按国家分组,每组取第一条(已按 latency 升序)
	winners := map[string]model.SubNode{}
	for _, n := range nodes {
		cc := strings.ToUpper(strings.TrimSpace(n.Country))
		if cc == "" || cc == "XX" {
			continue // 跳过未识别国家
		}
		if _, ok := winners[cc]; !ok {
			winners[cc] = n
		}
	}

	// upsert pool_outbounds(tag=pool-{cc} 唯一)
	//
	// display_name 处理:首次 INSERT 给默认 "订阅池·XX",已存在的行**不覆盖**(用户可能改过)。
	// OnConflict.DoUpdates 列表故意省略 display_name + created_at,只刷被选举出的新 winner 的
	// 协议字段(type/options)+ 来源 node 引用(winner_node_id/latency)。
	//
	// sing-box 同步:tx 提交后再 sync(失败回滚不会留 ghost 出站),收集要同步的 tags
	// 出 tx 后批量 Remove+Add — 让 sing-box outbound_manager 跟 DB 一致,
	// 否则 CheckWinners cron 5min 跑一次会喷"outbound not found"。
	var toSync []model.PoolOutbound
	if err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		for cc, w := range winners {
			tag := CountryPoolTagPrefix + strings.ToLower(cc)
			po := model.PoolOutbound{
				Tag:           tag,
				Country:       cc,
				Type:          w.Type,
				DisplayName:   fmt.Sprintf("订阅池·%s", cc),
				Options:       w.Options,
				WinnerNodeId:  w.Id,
				WinnerLatency: w.LatencyMs,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "tag"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"type", "country", "options", "winner_node_id", "winner_latency", "updated_at",
				}),
			}).Create(&po).Error; err != nil {
				return err
			}
			// 重读拿到 upsert 后的真实行(含被保留的 display_name)
			var saved model.PoolOutbound
			if err := tx.Where("tag = ?", tag).First(&saved).Error; err != nil {
				return err
			}
			toSync = append(toSync, saved)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, po := range toSync {
		s.syncPoolToCore(po)
	}
	return nil
}

// syncPoolToCore 把 DB 的 pool_outbound 行推到 sing-box 的 outbound_manager。
// 用 Remove + Add 替代"原地更新"(sing-box 没暴露 Update 接口):
//   - RemoveOutbound 幂等(不存在的 tag 也安全)
//   - AddOutbound 用 pool_outbound.options + type + tag 重新生成完整 outbound JSON
//
// 调用时机:
//   - ElectWinners 选完 winner → 同步新出站到 sing-box
//   - CheckWinners 巡检挑次优后 → 同步切换
//   - 删孤儿 / ResetAll → 只 Remove,不 Add
//
// 容错:sing-box 没运行 / Remove/Add 失败都不返错;只 logger.Warning,
// 让用户能看到但不影响主流程。
func (s *SubService) syncPoolToCore(po model.PoolOutbound) {
	if corePtr == nil || !corePtr.IsRunning() {
		return
	}
	var opts map[string]any
	if err := json.Unmarshal(po.Options, &opts); err != nil {
		logger.Warning("syncPoolToCore unmarshal options for "+po.Tag+": ", err)
		return
	}
	opts["type"] = po.Type
	opts["tag"] = po.Tag
	raw, err := json.Marshal(opts)
	if err != nil {
		logger.Warning("syncPoolToCore marshal for "+po.Tag+": ", err)
		return
	}
	_ = corePtr.RemoveOutbound(po.Tag) // 幂等;不存在/已下线都行
	if err := corePtr.AddOutbound(raw); err != nil {
		logger.Warning("syncPoolToCore AddOutbound "+po.Tag+": ", err)
	}
}

// UpdatePoolOutboundDisplayName 用户改"订阅池出站"的中转名称。
// 只允许动 display_name(其他字段由 SubService 自动维护;改了 type/options 会被
// 下次 RefreshSub 选举覆盖,UX 不连贯所以禁止 API 改)。
func (s *SubService) UpdatePoolOutboundDisplayName(id uint, displayName string) error {
	if id == 0 {
		return errors.New("id 必填")
	}
	return database.GetDB().Model(&model.PoolOutbound{}).
		Where("id = ?", id).
		Update("display_name", strings.TrimSpace(displayName)).Error
}

// CheckWinners 巡检所有 pool_outbounds 行:
//   - 跑探测看 winner 还活不活
//   - 死了立刻从 sub_nodes 同国家里挑次优(alive=true 中延迟最低且 ≠ 当前 server)
//   - 一并把死掉的 sub_node 标 alive=false(等下次 RefreshSub 复活探测)
//
// 巡检走 outbound tag — sing-box 必须把 pool_outbounds 行注入到 config 才能 probe;
// 如果 sing-box 还没注入(刚启动或刚选举完未 reload),probe 会"outbound not found",
// 这种情况跳过该轮,等下个 reload 后再巡检。
func (s *SubService) CheckWinners(ctx context.Context) error {
	subOpsMu.Lock()
	defer subOpsMu.Unlock()

	var pools []model.PoolOutbound
	if err := database.GetDB().Find(&pools).Error; err != nil {
		return err
	}
	if len(pools) == 0 {
		return nil
	}
	for _, po := range pools {
		cc := po.Country
		if corePtr == nil || !corePtr.IsRunning() {
			continue
		}
		res := core.ProbeOutboundByTag(ctx, po.Tag)
		if res.OK {
			continue // 还活,跳过
		}
		// 死了:把当前 winner 对应的 sub_node 标 alive=false
		if po.WinnerNodeId > 0 {
			database.GetDB().Model(&model.SubNode{}).
				Where("id = ?", po.WinnerNodeId).
				Updates(map[string]any{"alive": false, "last_error": res.Error})
			logger.Warning(fmt.Sprintf("%s winner(node#%d)死了:%s,尝试 re-elect",
				po.Tag, po.WinnerNodeId, res.Error))
		}
		// re-elect:挑同国家下个最快 alive(排除当前 winner)
		var next model.SubNode
		err := database.GetDB().Where("country = ? AND alive = ? AND id != ?",
			cc, true, po.WinnerNodeId).Order("latency_ms ASC").First(&next).Error
		if err != nil {
			logger.Warning("re-elect: 池 " + cc + " 无可用 alive 节点")
			continue
		}
		database.GetDB().Model(&model.PoolOutbound{}).Where("id = ?", po.Id).Updates(map[string]any{
			"type":           next.Type,
			"options":        next.Options,
			"winner_node_id": next.Id,
			"winner_latency": next.LatencyMs,
		})
		// 同步切换后的 outbound 到 sing-box
		var updated model.PoolOutbound
		if err := database.GetDB().First(&updated, po.Id).Error; err == nil {
			s.syncPoolToCore(updated)
		}
		logger.Info(fmt.Sprintf("%s 已切换到 node#%d (%s:%d, %dms)",
			po.Tag, next.Id, next.Server, next.ServerPort, next.LatencyMs))
	}
	return nil
}

// GetAllPoolOutbounds 给 ConfigService.GetConfig 用 — 拿出所有订阅池出站,
// 渲染时合并进 sing-box outbounds 数组,sing-box 看不到表分布,只看到完整 outbound 列表。
func (s *SubService) GetAllPoolOutbounds() ([]model.PoolOutbound, error) {
	var pools []model.PoolOutbound
	err := database.GetDB().Find(&pools).Error
	return pools, err
}

// GetPoolOutboundConfigs 把 pool_outbounds 表里每一行渲染成 sing-box outbound JSON。
// 渲染规则:options 是 sub_node 阶段已生成的完整 outbound options(含 server/server_port
// /tls/transport 等),只需补 type + tag 即可,跟 OutboundService.GetAllConfig 输出结构一致。
//
// 命名空间隔离:pool_outbounds.tag 已强制 pool- 前缀 + uniqueIndex,跟用户手配的 outbounds
// 表绝对不会撞 tag(用户没法手建 pool- 前缀的出站 —— 实际可以,但本身就是冲突信号)。
func (s *SubService) GetPoolOutboundConfigs() ([]json.RawMessage, error) {
	pools, err := s.GetAllPoolOutbounds()
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(pools))
	for _, po := range pools {
		// options 是个 JSON object,我们要塞 type + tag 进去
		var opts map[string]any
		if err := json.Unmarshal(po.Options, &opts); err != nil {
			logger.Warning("pool outbound options unmarshal: ", err)
			continue
		}
		opts["type"] = po.Type
		opts["tag"] = po.Tag
		raw, err := json.Marshal(opts)
		if err != nil {
			logger.Warning("pool outbound marshal: ", err)
			continue
		}
		out = append(out, raw)
	}
	return out, nil
}
