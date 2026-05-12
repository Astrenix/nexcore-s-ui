package api

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
)

// ---------- 订阅池(机场订阅导入 + 国家分组 winner 选举)----------
// 这些 handler 给前端用,鉴权走 panel cookie session(在 apiHandler.checkLogin)。
// 后端逻辑见 service/sub.go + cronjob/subJob.go。
//
// 路由通过 apiHandler.go 的 :postAction / :getAction 注入 switch,
// action 名 sub* / electWinners(POST),subs / subNodes / subPools(GET)。

// ApiSubList GET /api/subs — 全部订阅源
func (a *ApiService) ApiSubList(c *gin.Context) {
	subs, err := a.SubService.List()
	jsonObj(c, subs, err)
}

// ApiSubSave POST /api/subSave — 新增或更新(以 id 是否 > 0 判定)
func (a *ApiService) ApiSubSave(c *gin.Context) {
	var sub model.Sub
	if err := c.ShouldBind(&sub); err != nil {
		jsonMsg(c, "", err)
		return
	}
	var err error
	if sub.Id > 0 {
		err = a.SubService.Update(&sub)
	} else {
		err = a.SubService.Create(&sub)
	}
	jsonObj(c, sub, err)
}

// ApiSubDelete POST /api/subDelete — body id=<n>
func (a *ApiService) ApiSubDelete(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	err = a.SubService.Delete(uint(id))
	jsonMsg(c, "subDelete", err)
}

// ApiSubRefresh POST /api/subRefresh — 手动触发刷新一条订阅(同步,5min 超时)
func (a *ApiService) ApiSubRefresh(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	res, err := a.SubService.RefreshSub(ctx, uint(id))
	jsonObj(c, res, err)
}

// ApiElectWinners POST /api/electWinners — 手动重选 winners(debug)
func (a *ApiService) ApiElectWinners(c *gin.Context) {
	err := a.SubService.ElectWinners()
	jsonMsg(c, "electWinners", err)
}

// ApiSubNodes GET /api/subNodes — 节点池;支持 ?sub_id=N&country=HK&alive=true
func (a *ApiService) ApiSubNodes(c *gin.Context) {
	db := database.GetDB()
	q := db.Model(&model.SubNode{})
	if v := c.Query("sub_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			q = q.Where("sub_id = ?", id)
		}
	}
	if v := c.Query("country"); v != "" {
		q = q.Where("country = ?", strings.ToUpper(v))
	}
	if v := c.Query("alive"); v != "" {
		q = q.Where("alive = ?", v == "true" || v == "1")
	}
	var nodes []model.SubNode
	err := q.Order("country ASC, latency_ms ASC").Find(&nodes).Error
	jsonObj(c, nodes, err)
}

// ApiPoolOutbounds GET /api/poolOutbounds — 仅订阅池出站(独立于「出站管理」)
//
// 给前端「订阅池出站」section 显示用。**不出现在 /api/outbounds**(那是用户手配出站)。
// 不允许 PUT type/options(被 SubService 自动覆盖,UX 不连贯);只允许改 display_name
// 走 ApiPoolOutboundSave。
func (a *ApiService) ApiPoolOutbounds(c *gin.Context) {
	pools, err := a.SubService.GetAllPoolOutbounds()
	jsonObj(c, pools, err)
}

// ApiPoolOutboundSave POST /api/poolOutboundSave — 只改 display_name(中转名称)。
// body: id=<n> display_name=<text>
// 其它字段(type/tag/options 等)拒改:让用户在「订阅池」UI 改这些字段会被下次 RefreshSub
// 自动重选覆盖,体验不连贯,所以 API 层面就限死 display_name。
func (a *ApiService) ApiPoolOutboundSave(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	dn := c.PostForm("display_name")
	err = a.SubService.UpdatePoolOutboundDisplayName(uint(id), dn)
	jsonMsg(c, "poolOutboundSave", err)
}

// ApiSubPools GET /api/subPools — 各国家 winner + alive/total 计数
func (a *ApiService) ApiSubPools(c *gin.Context) {
	db := database.GetDB()
	// 按国家聚合 alive / total
	type aggRow struct {
		Country string `json:"country"`
		Total   int    `json:"total"`
		Alive   int    `json:"alive"`
	}
	var aggs []aggRow
	err := db.Model(&model.SubNode{}).
		Select("country, COUNT(*) as total, SUM(CASE WHEN alive THEN 1 ELSE 0 END) as alive").
		Where("country != '' AND country != 'XX'").
		Group("country").
		Scan(&aggs).Error
	if err != nil {
		jsonObj(c, nil, err)
		return
	}

	// 每国最快 alive(winner 候选)— 子查询取分组最小 latency
	type winnerRow struct {
		Country    string `json:"country"`
		Id         uint   `json:"id"`
		Remark     string `json:"remark"`
		Type       string `json:"type"`
		Server     string `json:"server"`
		ServerPort uint16 `json:"server_port"`
		ExitIP     string `json:"exit_ip"`
		LatencyMs  int    `json:"latency_ms"`
	}
	winnersByCC := map[string]winnerRow{}
	rows, rerr := db.Raw(`
		SELECT a.country, a.id, a.remark, a.type, a.server, a.server_port, a.exit_ip, a.latency_ms
		FROM sub_nodes a
		WHERE a.alive = 1 AND a.country != '' AND a.country != 'XX'
		  AND a.latency_ms = (
		    SELECT MIN(b.latency_ms) FROM sub_nodes b
		    WHERE b.alive = 1 AND b.country = a.country
		  )
		GROUP BY a.country
	`).Rows()
	if rerr == nil {
		for rows.Next() {
			var w winnerRow
			_ = rows.Scan(&w.Country, &w.Id, &w.Remark, &w.Type, &w.Server, &w.ServerPort, &w.ExitIP, &w.LatencyMs)
			winnersByCC[w.Country] = w
		}
		rows.Close()
	}

	// 当前 pool_outbounds(独立表;v1.7.26 之前在 outbounds 表,v1.7.26 拆出来)
	type poolOB struct {
		Id  uint
		Tag string
	}
	var pools []poolOB
	db.Model(&model.PoolOutbound{}).
		Select("id, tag").Scan(&pools)
	poolByCC := map[string]poolOB{}
	for _, p := range pools {
		cc := strings.ToUpper(strings.TrimPrefix(p.Tag, service.CountryPoolTagPrefix))
		poolByCC[cc] = p
	}

	type rowOut struct {
		Country     string     `json:"country"`
		Total       int        `json:"total"`
		Alive       int        `json:"alive"`
		Winner      *winnerRow `json:"winner,omitempty"`
		OutboundId  uint       `json:"outbound_id,omitempty"`
		OutboundTag string     `json:"outbound_tag,omitempty"`
	}
	var out []rowOut
	for _, ag := range aggs {
		row := rowOut{Country: ag.Country, Total: ag.Total, Alive: ag.Alive}
		if w, ok := winnersByCC[ag.Country]; ok {
			tmp := w
			row.Winner = &tmp
		}
		if p, ok := poolByCC[ag.Country]; ok {
			row.OutboundId = p.Id
			row.OutboundTag = p.Tag
		}
		out = append(out, row)
	}
	jsonObj(c, out, nil)
}
