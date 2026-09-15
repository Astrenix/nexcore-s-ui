package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

type ApiLogService struct{}

// === 异步批量写入 ===
//
// 🩸 为什么不能同步写(2026-09-15):
// AccessLogMiddleware 挂在 /api/v1 整组最外层,**每一次 API 调用都会走到这里**。
// 旧实现是 `db.Create(entry)` —— 单行 INSERT、GORM 隐式事务,于是每个请求都在
// **请求路径上同步做一次 sqlite 写事务**(WAL 追加 + fsync),延迟直接计进主控
// 看到的 API 耗时。而主控的调用频率是固定的:每 60 秒对每个节点探活一次
// (每节点 1440 行/天),外加 5 分钟一轮的订阅用量调度与入站探测。
//
// 改成攒批异步:请求侧只做一次 channel 发送(纳秒级),真正的落库由单个后台
// goroutine 攒够 apiLogBatchSize 或每 apiLogFlushEvery 冲一次。写事务数量从
// 「每请求 1 次」降到「每批 1 次」。
//
// 三条约束:
//  1. **channel 满了就丢,绝不阻塞业务请求** —— 这是审计日志,不是资金流水;
//     为了记一条日志把 API 卡住是本末倒置。丢弃有计数且会打日志,不静默。
//  2. **不在 init 里起 goroutine** —— 用 sync.Once 在首次 Add 时懒启动,
//     否则 `go test` 加载本包就会拉起一个写库的后台协程。
//  3. **必须留一个可调用的 Flush** —— 异步化之后「写了就能查到」不再成立,
//     测试和优雅退出都需要一个同步点(见 FlushApiLogs)。
const (
	apiLogQueueSize  = 512
	apiLogBatchSize  = 64
	apiLogFlushEvery = 2 * time.Second
)

var (
	apiLogCh      chan *model.ApiLog
	apiLogOnce    sync.Once
	apiLogDropped atomic.Int64
	// apiLogFlushed 仅用于让调用方等待「已入队的都落库了」,见 FlushApiLogs
	apiLogFlushReq chan chan struct{}
)

func startApiLogWriter() {
	apiLogCh = make(chan *model.ApiLog, apiLogQueueSize)
	apiLogFlushReq = make(chan chan struct{}, 4)
	go func() {
		// 🩸 必须是函数体第一行:节点端任一 goroutine 的未捕获 panic 会让
		// **整个面板进程退出**,该节点全部入站随之下线,而主控侧只看得到「节点失联」。
		// 守卫 TestEveryGoroutineHasPanicRecover 钉着这条(本次改动正是被它抓出来的)。
		//
		// 降级语义:真的 panic 了,这个 writer 就停了 —— 后续 Add 会把 channel 填满
		// 并开始丢弃。这是**有意的取舍**:审计日志停掉可以接受,拖垮节点不行。
		// 丢弃不会静默(apiLogDropped 有计数),但 ticker 也随之停止,所以那条汇总
		// 日志不会再打 —— 排障时以「api_logs 停止增长」为信号,别指望有告警。
		defer common.Recover("apilog: 异步写入协程")

		batch := make([]*model.ApiLog, 0, apiLogBatchSize)
		ticker := time.NewTicker(apiLogFlushEvery)
		defer ticker.Stop()

		flush := func() {
			if len(batch) == 0 {
				return
			}
			db := database.GetDB()
			if db != nil {
				// 一个事务写完整批。失败只记 warning:日志落不了库不能影响业务,
				// 但也不能像从前那样连错误都吞掉 —— 那会让「审计表为空」无从解释。
				if err := db.CreateInBatches(batch, apiLogBatchSize).Error; err != nil {
					logger.Warning("api log batch insert failed:", err)
				}
			}
			batch = batch[:0]
		}

		for {
			select {
			case entry := <-apiLogCh:
				batch = append(batch, entry)
				if len(batch) >= apiLogBatchSize {
					flush()
				}
			case <-ticker.C:
				flush()
				if n := apiLogDropped.Swap(0); n > 0 {
					logger.Warning("api log queue full, dropped entries:", n)
				}
			case done := <-apiLogFlushReq:
				// 把队列里已排队的也一并收走,再落库 —— 否则调用方仍可能查不到
				for {
					select {
					case entry := <-apiLogCh:
						batch = append(batch, entry)
						continue
					default:
					}
					break
				}
				flush()
				close(done)
			}
		}
	}()
}

// Add 记录一条 API 调用(异步)。失败时静默 — 日志写不进去也不能拖崩业务调用。
func (s *ApiLogService) Add(entry *model.ApiLog) {
	if entry == nil {
		return
	}
	if database.GetDB() == nil {
		return
	}
	if entry.DateTime == 0 {
		entry.DateTime = time.Now().Unix()
	}
	apiLogOnce.Do(startApiLogWriter)
	select {
	case apiLogCh <- entry:
	default:
		// 队列满:丢这一条,计数留给 ticker 汇总打一次日志。
		// 绝不在这里阻塞 —— 调用点在 HTTP 请求路径上。
		apiLogDropped.Add(1)
	}
}

// FlushApiLogs 等待「此刻之前入队的日志」全部落库。
// 给优雅退出与测试用;writer 还没启动过时直接返回(没有任何待写数据)。
func FlushApiLogs() {
	if apiLogFlushReq == nil {
		return
	}
	done := make(chan struct{})
	select {
	case apiLogFlushReq <- done:
	default:
		return // writer 正忙且请求队列已满,放弃等待,不阻塞调用方
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// ApiLogDroppedCount 已丢弃条数(被 ticker 汇总清零前的瞬时值),供排障读取。
func ApiLogDroppedCount() int64 { return apiLogDropped.Load() }

// List 分页查询。method / path / username 任一非空时按精确(path 用 like)过滤;
// since/until 是 unix 秒,0 表示不过滤。返回 (logs, total)。
func (s *ApiLogService) List(method, path, username string, since, until int64, limit, offset int) ([]model.ApiLog, int64, error) {
	db := database.GetDB()
	q := db.Model(&model.ApiLog{})
	if method != "" {
		q = q.Where("method = ?", method)
	}
	if path != "" {
		q = q.Where("path LIKE ?", "%"+path+"%")
	}
	if username != "" {
		q = q.Where("username = ?", username)
	}
	if since > 0 {
		q = q.Where("date_time >= ?", since)
	}
	if until > 0 {
		q = q.Where("date_time <= ?", until)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}

	var logs []model.ApiLog
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// Clear 清空所有日志(管理员手动维护用)。
func (s *ApiLogService) Clear() error {
	db := database.GetDB()
	return db.Exec("DELETE FROM api_logs").Error
}

// PruneOlderThan 删除 unix 秒之前的日志,后台 cron 用。
func (s *ApiLogService) PruneOlderThan(ts int64) error {
	db := database.GetDB()
	return db.Where("date_time < ?", ts).Delete(&model.ApiLog{}).Error
}
