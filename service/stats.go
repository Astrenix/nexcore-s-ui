package service

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

type onlines struct {
	Inbound    []string       `json:"inbound,omitempty"`
	User       []string       `json:"user,omitempty"`
	Outbound   []string       `json:"outbound,omitempty"`
	InboundIPs map[string]int `json:"inbound_ips,omitempty"` // tag → 当前活跃 source IP 数
	UserIPs    map[string]int `json:"user_ips,omitempty"`    // 客户端 name → 当前活跃 source IP 数
}

// onlineResources 由 SaveStats(cron 每 10s)整体重建、被 GetOnlines / 入站列表 API
// 并发读。过去是就地改字段的共享指针 → 读方可能读到半更新的 map / 撕裂 slice
// (-race 必报)。改 atomic.Pointer:写方构造**全新**快照一次性 Store,读方 Load
// 拿到的永远是一致快照,无锁、无撕裂。
var onlineResources atomic.Pointer[onlines]

func init() {
	onlineResources.Store(&onlines{})
}

type StatsService struct {
}

func (s *StatsService) SaveStats(enableTraffic bool) error {
	// 每轮构造全新快照 snap;函数所有返回路径统一在 defer 里一次性发布。
	// 空快照即代表"无在线"——不论核心是否运行,旧快照都不该继续展示。
	snap := &onlines{}
	defer func() { onlineResources.Store(snap) }()

	if corePtr == nil || !corePtr.IsRunning() {
		return nil
	}
	box := corePtr.GetInstance()
	if box == nil {
		return nil
	}
	st := box.StatsTracker()
	if st == nil {
		return nil
	}
	stats := st.GetStats()

	// 在线 IP 快照(60s 窗口)。即使本轮没流量样本(下面 len(*stats)==0 会
	// 提前 return)也想拿到 IP 计数,所以放在 stats 检查之前。
	inbIPs, usrIPs := st.SnapshotOnlineIPs(60)
	if len(inbIPs) > 0 {
		snap.InboundIPs = inbIPs
	}
	if len(usrIPs) > 0 {
		snap.UserIPs = usrIPs
	}

	if len(*stats) == 0 {
		return nil
	}

	var err error
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, stat := range *stats {
		if stat.Resource == "user" {
			if stat.Direction {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("up", gorm.Expr("up + ?", stat.Traffic)).Error
			} else {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("down", gorm.Expr("down + ?", stat.Traffic)).Error
			}
			if err != nil {
				return err
			}
		}
		if stat.Direction {
			switch stat.Resource {
			case "inbound":
				snap.Inbound = append(snap.Inbound, stat.Tag)
			case "outbound":
				snap.Outbound = append(snap.Outbound, stat.Tag)
			case "user":
				snap.User = append(snap.User, stat.Tag)
			}
		}
	}

	if !enableTraffic {
		return nil
	}
	return tx.Create(&stats).Error
}

// GetTotals 按 resource 分组,返回每个 tag 的累计 up / down 字节数。
// resource 取 "inbound" / "outbound" / "user"。给前端列表页用,O(1) 一次 SQL 拿全。
// direction: true=up, false=down(与 SaveStats 保持一致)。
func (s *StatsService) GetTotals(resource string) (map[string]map[string]int64, error) {
	type row struct {
		Tag       string
		Direction bool
		Total     int64
	}
	var rows []row
	db := database.GetDB()
	err := db.Model(model.Stats{}).
		Select("tag, direction, SUM(traffic) AS total").
		Where("resource = ?", resource).
		Group("tag, direction").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[string]int64, len(rows))
	for _, r := range rows {
		m, ok := out[r.Tag]
		if !ok {
			m = map[string]int64{"up": 0, "down": 0}
			out[r.Tag] = m
		}
		if r.Direction {
			m["up"] = r.Total
		} else {
			m["down"] = r.Total
		}
	}
	return out, nil
}

func (s *StatsService) GetStats(resource string, tag string, limit int) ([]model.Stats, error) {
	var err error
	var result []model.Stats

	currentTime := time.Now().Unix()
	timeDiff := currentTime - (int64(limit) * 3600)

	db := database.GetDB()
	resources := []string{resource}
	if resource == "endpoint" {
		resources = []string{"inbound", "outbound"}
	}
	err = db.Model(model.Stats{}).Where("resource in ? AND tag = ? AND date_time > ?", resources, tag, timeDiff).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	result = s.downsampleStats(result, 60) // 60 rows for 30 buckets
	return result, nil
}

// downsampleStats reduces stats to maxRows rows.
// Each bucket outputs two rows (direction false and true) with average Traffic.
func (s *StatsService) downsampleStats(stats []model.Stats, maxRows int) []model.Stats {
	if len(stats) <= maxRows {
		return stats
	}
	numBuckets := int(maxRows / 2)
	sort.Slice(stats, func(i, j int) bool { return stats[i].DateTime < stats[j].DateTime })
	timeMin, timeMax := stats[0].DateTime, stats[len(stats)-1].DateTime
	bucketSpan := (timeMax - timeMin) / int64(numBuckets)
	if bucketSpan == 0 {
		bucketSpan = 1
	}
	downsampled := make([]model.Stats, 0, maxRows)
	for i := 0; i < numBuckets; i++ {
		bucketStart := timeMin + int64(i)*bucketSpan
		bucketEnd := timeMin + int64(i+1)*bucketSpan
		if i == numBuckets-1 {
			bucketEnd = timeMax + 1
		}
		for _, dir := range []bool{false, true} {
			var sum int64
			var count int
			for _, r := range stats {
				if r.DateTime >= bucketStart && r.DateTime < bucketEnd && r.Direction == dir {
					sum += r.Traffic
					count++
				}
			}
			avg := int64(0)
			if count > 0 {
				avg = sum / int64(count)
			}
			downsampled = append(downsampled, model.Stats{
				DateTime:  bucketStart,
				Resource:  stats[0].Resource,
				Tag:       stats[0].Tag,
				Direction: dir,
				Traffic:   avg,
			})
		}
	}
	return downsampled
}

func (s *StatsService) GetOnlines() (onlines, error) {
	if snap := onlineResources.Load(); snap != nil {
		return *snap, nil
	}
	return onlines{}, nil
}

// GetLiveTotals 返当前 sing-box StatsTracker 内存里的累计字节(cum + pending),
// resource 取 "inbound"/"outbound"/"user"。给前端"实时网速"卡片用 — 1.5s 高频
// 拉单调递增,自己算 diff/dt 得到精确速率。
//
// 跟 GetTotals 的区别:GetTotals 是 DB 累计(SaveStats cron 每 10s 落库一次),
// 1.5s diff 通常 = 0;GetLiveTotals 是内存实时,无周期延迟。
//
// sing-box 没运行时返空 map(前端不会因此爆错)。
func (s *StatsService) GetLiveTotals(resource string) map[string]map[string]int64 {
	if corePtr == nil || !corePtr.IsRunning() {
		return map[string]map[string]int64{}
	}
	box := corePtr.GetInstance()
	if box == nil {
		return map[string]map[string]int64{}
	}
	st := box.StatsTracker()
	if st == nil {
		return map[string]map[string]int64{}
	}
	all := st.SnapshotCounters()
	if r, ok := all[resource]; ok {
		return r
	}
	return map[string]map[string]int64{}
}

// GetOnlineIPs 查询单个 inbound 或 user 当前 60s 窗口内的活跃 source IP 列表。
// 用法:resource="user" + tag=客户端 name → 该账号被哪些 IP 同时在用(跨入站
// 自动汇总去重,因为 userIPs 按 name 索引)。给"限制 IP 数"功能取数据。
func (s *StatsService) GetOnlineIPs(resource, tag string) []string {
	if corePtr == nil || !corePtr.IsRunning() {
		return []string{}
	}
	box := corePtr.GetInstance()
	if box == nil {
		return []string{}
	}
	st := box.StatsTracker()
	if st == nil {
		return []string{}
	}
	return st.QueryOnlineIPs(resource, tag, 60)
}
// ResetByTag 清掉单个 tag 在某 resource(inbound/outbound/user)下的全部
// 历史流量样本。"重置流量"按钮调这个 — UI 上等同于把进度条归零。
// 不存在的 tag 静默成功(idempotent),不报错。
//
// 注:user 资源除了删 stats 行,还要把 client.up/down 字段同步清零 ——
// SaveStats 每 10s 把 user delta 累加进 client.up/down 维护"客户端总流量",
// 这是另一个数据源(UsageStats / 客户端列表用)。只删 stats 不清字段
// 会让两份数字不一致,用户重置后还看到旧值,以为没生效。
func (s *StatsService) ResetByTag(resource, tag string) error {
	db := database.GetDB()
	if err := db.Where("resource = ? AND tag = ?", resource, tag).Delete(&model.Stats{}).Error; err != nil {
		return err
	}
	if resource == "user" {
		return db.Model(&model.Client{}).Where("name = ?", tag).
			Updates(map[string]interface{}{"up": 0, "down": 0}).Error
	}
	return nil
}

func (s *StatsService) DelOldStats(days int) error {
	oldTime := time.Now().AddDate(0, 0, -(days)).Unix()
	db := database.GetDB()
	return db.Where("date_time < ?", oldTime).Delete(model.Stats{}).Error
}
