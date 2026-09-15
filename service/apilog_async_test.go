package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// ApiLogService.Add 在 2026-09-15 从「每次调用同步写一行 sqlite」改成了攒批异步。
//
// 理由在 apilog.go 的注释里:AccessLogMiddleware 挂在 /api/v1 整组最外层,
// 每个请求都会走到,旧实现把一次写事务放在了请求路径上。
//
// 但异步化把一条隐含保证弄丢了 ——「Add 完立刻就能查到」不再成立。
// 这个用例钉住新的契约:**Add 入队的日志,经 FlushApiLogs 之后必须全部落库,一条不少**。
// 少了就是审计数据静默丢失,而审计表的特点是「没人会发现它少了几行」。
func TestApiLogAsyncWriteDoesNotLoseEntries(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "apilog.db")); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}

	const marker = "/api/v1/__async_probe__"
	const n = 7

	// 夹具自证:开始前这个 path 必须一条都没有,否则下面的计数毫无信息量
	var before int64
	if err := database.GetDB().Model(&model.ApiLog{}).Where("path = ?", marker).Count(&before).Error; err != nil {
		t.Fatalf("预查失败: %v", err)
	}
	if before != 0 {
		t.Fatalf("夹具不干净:已有 %d 条 %s", before, marker)
	}

	svc := &ApiLogService{}
	for i := 0; i < n; i++ {
		svc.Add(&model.ApiLog{
			Method: "GET",
			Path:   marker,
			Status: 200,
			Err:    fmt.Sprintf("seq-%d", i),
		})
	}

	// 异步写的同步点。没有它,下面的查询就是在跟后台协程赛跑。
	FlushApiLogs()

	var after int64
	if err := database.GetDB().Model(&model.ApiLog{}).Where("path = ?", marker).Count(&after).Error; err != nil {
		t.Fatalf("复查失败: %v", err)
	}
	if after != n {
		t.Fatalf("异步写丢数据:入队 %d 条,落库 %d 条", n, after)
	}

	// DateTime 必须被补上 —— 它是 PruneOlderThan 唯一的判据,
	// 留 0 会让这行日志永远被当成 1970 年的,下一次清理就删掉。
	var zeroTime int64
	if err := database.GetDB().Model(&model.ApiLog{}).
		Where("path = ? AND (date_time IS NULL OR date_time = 0)", marker).
		Count(&zeroTime).Error; err != nil {
		t.Fatalf("查 date_time 失败: %v", err)
	}
	if zeroTime != 0 {
		t.Fatalf("有 %d 条日志的 date_time 没被补上,会被保留期清理误删", zeroTime)
	}
}

// Add 不能因为数据库没初始化就 panic —— 它挂在每个 API 请求的路径上,
// 而 panic 会让整个面板进程退出(见 goroutine_recover_guard_test 的说明)。
func TestApiLogAddSurvivesNilDB(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Add 在 DB 不可用时 panic 了: %v", r)
		}
	}()
	svc := &ApiLogService{}
	svc.Add(nil) // nil entry 也不能炸
}
