package cronjob

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
)

// setRetention 直接写 settings 表摆保留天数。
func setRetention(t *testing.T, key string, days string) {
	t.Helper()
	if err := database.GetDB().Exec(
		"INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, days).Error; err != nil {
		t.Fatalf("设置 %s=%s 失败: %v", key, days, err)
	}
}

// 夹具时间刻意远离边界:保留窗是 7 天,就用 60 天前(必删)和 1 天前(必留)。
// 贴着 7 天造样本会在跨日界那几小时飘红,而飘忽的假红比恒红更糟 —— 它让
// 真正的新红淹没在噪声里。
func seedLogRows(t *testing.T) {
	t.Helper()
	d := database.GetDB()
	old := time.Now().AddDate(0, 0, -60).Unix()
	fresh := time.Now().AddDate(0, 0, -1).Unix()

	for _, ts := range []int64{old, fresh} {
		if err := d.Create(&model.ApiLog{DateTime: ts, Method: "GET", Path: "/x"}).Error; err != nil {
			t.Fatalf("造 api_log 失败: %v", err)
		}
		if err := d.Create(&model.Changes{DateTime: ts, Actor: "t", Key: "k", Action: "a"}).Error; err != nil {
			t.Fatalf("造 change 失败: %v", err)
		}
	}
}

func countRows(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := database.GetDB().Table(table).Count(&n).Error; err != nil {
		t.Fatalf("数 %s 失败: %v", table, err)
	}
	return n
}

// api_logs 与 changes 此前【没有任何自动清理】:PruneOlderThan 写好了却零调用方,
// changes 连删除函数都没有。本用例钉住 DelLogsJob 确实在删,而且只删超期的。
func TestDelLogsJobPrunesOnlyExpired(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "logs.db")); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	setRetention(t, "apiLogAge", "7")
	setRetention(t, "changesAge", "7")
	seedLogRows(t)

	if got := countRows(t, "api_logs"); got != 2 {
		t.Fatalf("夹具不成立:api_logs 应有 2 行,实得 %d", got)
	}

	NewDelLogsJob().Run()

	if got := countRows(t, "api_logs"); got != 1 {
		t.Fatalf("api_logs 应只剩 1 行(60 天前那条被删),实得 %d", got)
	}
	if got := countRows(t, "changes"); got != 1 {
		t.Fatalf("changes 应只剩 1 行,实得 %d", got)
	}
}

// 0 = 不清理。与 trafficAge 的既有语义保持一致 —— 把"没配/关闭"解读成"全删"
// 是不可逆的数据损失。
func TestDelLogsJobZeroAgeKeepsEverything(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "logs0.db")); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	setRetention(t, "apiLogAge", "0")
	setRetention(t, "changesAge", "0")
	seedLogRows(t)

	NewDelLogsJob().Run()

	if got := countRows(t, "api_logs"); got != 2 {
		t.Fatalf("apiLogAge=0 时不该删任何行,api_logs 剩 %d(应为 2)", got)
	}
	if got := countRows(t, "changes"); got != 2 {
		t.Fatalf("changesAge=0 时不该删任何行,changes 剩 %d(应为 2)", got)
	}
}
