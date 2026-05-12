package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/service"

	"github.com/gin-gonic/gin"
)

// ---------- 订阅源 CRUD ----------

func (a *Controller) listSubs(c *gin.Context) {
	subs, err := a.subSvc.List()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, subs)
}

func (a *Controller) createSub(c *gin.Context) {
	var sub model.Sub
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "bad json: " + err.Error()})
		return
	}
	sub.Id = 0
	if err := a.subSvc.Create(&sub); err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, sub)
}

func (a *Controller) updateSub(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "bad id"})
		return
	}
	var sub model.Sub
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "bad json: " + err.Error()})
		return
	}
	sub.Id = uint(id)
	if err := a.subSvc.Update(&sub); err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, sub)
}

func (a *Controller) deleteSub(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "bad id"})
		return
	}
	if err := a.subSvc.Delete(uint(id)); err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, gin.H{"deleted": id})
}

// refreshSub 手动触发单订阅刷新(同步,等结果返回)。
// 同步是为了让用户在 UI 上看到刷新进度;后端 5min 超时兜底。
func (a *Controller) refreshSub(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "bad id"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	res, err := a.subSvc.RefreshSub(ctx, uint(id))
	if err != nil {
		// 返 200 + result 里带 error,前端按 ok=false 处理:UX 比 5xx 强,
		// 用户能立刻看到原因(网络 / 解析失败 / 0 节点 ...)
		OK(c, res)
		return
	}
	OK(c, res)
}

// ---------- 节点池 ----------

// listSubNodes 查节点池;支持 query 过滤:sub_id=N、country=HK、alive=true。
func (a *Controller) listSubNodes(c *gin.Context) {
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
	if err := q.Order("country ASC, latency_ms ASC").Find(&nodes).Error; err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, nodes)
}

// listSubPools 给前端展示各国家池状态:
// 返 [{country, winner_node_id, winner_server, winner_port, winner_latency, alive_count, total_count, pool_outbound_tag, pool_outbound_id}]
func (a *Controller) listSubPools(c *gin.Context) {
	db := database.GetDB()

	// 按国家聚合 alive / total 计数
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
		Internal(c, "db_error", err)
		return
	}

	// 每国最快的 alive(winner 候选)
	type winnerRow struct {
		Country    string `json:"country"`
		Id         uint   `json:"id"`
		Remark     string `json:"remark"`
		Server     string `json:"server"`
		ServerPort uint16 `json:"server_port"`
		ExitIP     string `json:"exit_ip"`
		LatencyMs  int    `json:"latency_ms"`
	}
	winnersByCC := map[string]winnerRow{}
	// 用 GORM 子查询拿"分组最小 latency 的行" — SQLite 兼容写法:
	// SELECT ... FROM sub_nodes a WHERE alive=1 AND latency_ms = (SELECT MIN(latency_ms) FROM sub_nodes WHERE alive=1 AND country=a.country)
	rows, err := db.Raw(`
		SELECT a.country, a.id, a.remark, a.server, a.server_port, a.exit_ip, a.latency_ms
		FROM sub_nodes a
		WHERE a.alive = 1 AND a.country != '' AND a.country != 'XX'
		  AND a.latency_ms = (
		    SELECT MIN(b.latency_ms) FROM sub_nodes b
		    WHERE b.alive = 1 AND b.country = a.country
		  )
		GROUP BY a.country
	`).Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w winnerRow
			_ = rows.Scan(&w.Country, &w.Id, &w.Remark, &w.Server, &w.ServerPort, &w.ExitIP, &w.LatencyMs)
			winnersByCC[w.Country] = w
		}
	}

	// 当前 pool-{cc} outbound 状态
	type poolOB struct {
		Id  uint
		Tag string
	}
	var pools []poolOB
	db.Model(&model.Outbound{}).Where("tag LIKE ?", service.CountryPoolTagPrefix+"%").
		Select("id, tag").Scan(&pools)
	poolByCC := map[string]poolOB{}
	for _, p := range pools {
		cc := strings.ToUpper(strings.TrimPrefix(p.Tag, service.CountryPoolTagPrefix))
		poolByCC[cc] = p
	}

	type rowOut struct {
		Country    string `json:"country"`
		Total      int    `json:"total"`
		Alive      int    `json:"alive"`
		Winner     *winnerRow `json:"winner,omitempty"`
		OutboundId uint   `json:"outbound_id,omitempty"`
		OutboundTag string `json:"outbound_tag,omitempty"`
	}
	var out []rowOut
	for _, ag := range aggs {
		row := rowOut{
			Country: ag.Country,
			Total:   ag.Total,
			Alive:   ag.Alive,
		}
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
	OK(c, out)
}

// electWinners 手动触发选举(主要给开发 / 故障排查用)。
func (a *Controller) electWinners(c *gin.Context) {
	if err := a.subSvc.ElectWinners(); err != nil {
		Internal(c, "elect_failed", err)
		return
	}
	OK(c, gin.H{"ok": true})
}
