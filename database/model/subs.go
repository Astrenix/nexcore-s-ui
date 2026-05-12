package model

import (
	"encoding/json"
	"time"
)

// Sub 订阅源:一个机场订阅链接 = 一行。
//
// 设计要点:
//   - URL 唯一(防止重复导入)
//   - RefreshInterval 单位分钟(默认 60min);0 = 用户禁用自动刷新,仅手动刷
//   - LastSyncedAt / LastStatus / LastError 记录上次刷新结果,前端展示
//   - 删订阅时级联删 SubNodes(GORM constraint OnDelete:CASCADE)
type Sub struct {
	Id              uint      `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name            string    `json:"name" form:"name" gorm:"size:128;not null"`
	URL             string    `json:"url" form:"url" gorm:"size:1024;uniqueIndex;not null"`
	Enable          bool      `json:"enable" form:"enable" gorm:"default:true"`
	RefreshInterval int       `json:"refresh_interval" form:"refresh_interval" gorm:"default:60"` // 分钟
	LastSyncedAt    time.Time `json:"last_synced_at"`
	LastStatus      string    `json:"last_status" gorm:"size:32"` // ok | failed | partial
	LastError       string    `json:"last_error" gorm:"size:512"`
	LastNodeCount   int       `json:"last_node_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SubNode 订阅展开后的单个节点:订阅每条 URI 链接 → 一行。
//
// 设计要点:
//   - 唯一键 (sub_id, server, server_port) — 同 sub 下同 host:port 视为同一节点
//   - Options 是 sing-box outbound 完整 JSON(已去掉 type/tag),winner 写入 outbound 时直接拿
//   - Country:ISO-2 大写优先(HK/JP/US…),识别不出来留 "XX",前端按 country 分组
//   - ExitIP:探测时通过 outbound 访问 cloudflare/cdn-cgi/trace 拿到的真落地 IP
//     主要用于:1) 去重(同 country 下同 exit_ip 只留延迟最低的)2) UI 展示
//   - LatencyMs:探测拉取 trace 的端到端时延(ms),用于 winner 排序
//   - Alive:本轮探测是否成功;winner 选举只看 Alive=true 的节点
//   - LastCheckAt:上次探测时间
// PoolOutbound 订阅池"国家 winner"出站 — 跟用户手配的 model.Outbound 完全分表。
//
// 为什么分表:
//   - 用户在「出站管理」里手配的常驻出站,语义是"长期稳定、用户拥有",对 UI 增删改要敏感
//   - 订阅池 winner 是 sub 系统按 country 自动维护的"瞬时"出站,刷新订阅就可能换内容
//   - 混在一张表里既污染出站列表,又让用户误以为可以编辑(改了下次刷新被覆盖)
//
// sing-box 看不到表分布:ConfigService.GetConfig 渲染最终 outbounds 时 union 两表。
// Tag 命名仍是 pool-{cc}(下游 inbound 绑这个 tag 不变),uniqueIndex 保证不重。
//
// WinnerNodeId 指向当前选中的 sub_nodes.id — 用于:
//   1. CheckWinners 巡检时找到对应 sub_node 标 alive=false
//   2. UI 显示当前 winner 来自哪条订阅
//   3. 订阅删除时级联清理(虽然 CASCADE 是 sub_node 那边的事,这里仅 FK 跟踪)
type PoolOutbound struct {
	Id           uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Tag          string          `json:"tag" gorm:"size:64;uniqueIndex;not null"` // pool-{cc}
	Country      string          `json:"country" gorm:"size:8;index;not null"`
	Type         string          `json:"type" gorm:"size:32;not null"`
	DisplayName  string          `json:"display_name" gorm:"size:128"`
	Options      json.RawMessage `json:"options" gorm:"type:blob"`
	WinnerNodeId uint            `json:"winner_node_id"`
	WinnerLatency int            `json:"winner_latency"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CreatedAt    time.Time       `json:"created_at"`
}

type SubNode struct {
	Id          uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	SubId       uint            `json:"sub_id" gorm:"not null;uniqueIndex:idx_sub_host"`
	Remark      string          `json:"remark" gorm:"size:256"`
	Type        string          `json:"type" gorm:"size:32;not null"`
	Server      string          `json:"server" gorm:"size:256;not null;uniqueIndex:idx_sub_host"`
	ServerPort  uint16          `json:"server_port" gorm:"not null;uniqueIndex:idx_sub_host"`
	Options     json.RawMessage `json:"options" gorm:"type:blob"`
	Country     string          `json:"country" gorm:"size:8;index"`
	ExitIP      string          `json:"exit_ip" gorm:"size:64;index"`
	LatencyMs   int             `json:"latency_ms"`
	Alive       bool            `json:"alive" gorm:"index"`
	LastError   string          `json:"last_error" gorm:"size:256"` // 探测失败原因(timeout / tls failure / http 403...)
	LastCheckAt time.Time       `json:"last_check_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
