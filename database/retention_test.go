package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/alireza0/s-ui/database/model"
)

// 新建的库必须是 auto_vacuum=INCREMENTAL,否则 DelStatsJob / DelLogsJob 删掉的页
// 只会进 freelist 永不归还文件系统 —— 生产实测 315MB 的库里 145MB 是这么来的:
// 清理天天在跑,磁盘天天在涨。
//
// 自证:先用【不带该参数】的 DSN 开一个库当对照组。它必须是 0 —— 否则说明
// SQLite 本来就默认 INCREMENTAL,本用例的 ==2 断言就什么都没测到(恒绿)。
func TestNewDBIsIncrementalAutoVacuum(t *testing.T) {
	ctlPath := filepath.Join(t.TempDir(), "control.db")
	ctl, err := gorm.Open(sqlite.Open(ctlPath+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("对照组开库失败: %v", err)
	}
	if err := ctl.Exec("CREATE TABLE probe(a INTEGER)").Error; err != nil {
		t.Fatalf("对照组建表失败: %v", err)
	}
	var ctlMode int
	if err := ctl.Raw("PRAGMA auto_vacuum").Scan(&ctlMode).Error; err != nil {
		t.Fatalf("对照组读 pragma 失败: %v", err)
	}
	if ctlMode != 0 {
		t.Fatalf("对照组 auto_vacuum 应为 0(NONE),实得 %d —— "+
			"SQLite 默认已变,下面的断言不再能证明 DSN 参数有效,请重写本用例", ctlMode)
	}

	dbPath := filepath.Join(t.TempDir(), "av.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	var mode int
	if err := GetDB().Raw("PRAGMA auto_vacuum").Scan(&mode).Error; err != nil {
		t.Fatalf("读 auto_vacuum 失败: %v", err)
	}
	if mode != 2 {
		t.Fatalf("新库 auto_vacuum 应为 2(INCREMENTAL),实得 %d —— "+
			"多半是 OpenDB 的 DSN 掉了 _auto_vacuum=incremental", mode)
	}
}

// migrateDefaultTrafficAge 只能把【继承自旧默认值 30】的那份降到 7。
// 人为配置过的值(14 / 60 / 0=关闭统计)都是明确意图,覆盖它们就是拿运营的
// 配置当默认值改。0 尤其致命:那是"关闭流量统计"的既有语义,改成 7 等于
// 把用户关掉的功能又打开,还开始删他本来想留着的数据。
func TestMigrateDefaultTrafficAgeOnlyTouchesLegacyDefault(t *testing.T) {
	cases := []struct {
		name  string
		start string
		want  string
	}{
		{"旧默认值 30 → 7", "30", "7"},
		{"人为设成 14,不动", "14", "14"},
		{"人为设成 60,不动", "60", "60"},
		{"0=关闭统计,绝不能动", "0", "0"},
		{"已经是 7,幂等", "7", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "mig.db")
			if err := InitDB(dbPath); err != nil {
				t.Fatalf("InitDB 失败: %v", err)
			}
			d := GetDB()
			// InitDB 自己会跑一次迁移,这里重新把值摆成待测状态
			if err := d.Exec(
				"INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
				"trafficAge", tc.start).Error; err != nil {
				t.Fatalf("摆状态失败: %v", err)
			}
			// 同时放一个同值的别的 key,确保迁移的 WHERE 带了 key 限定
			if err := d.Exec(
				"INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
				"sessionMaxAge", "30").Error; err != nil {
				t.Fatalf("摆邻居 key 失败: %v", err)
			}

			if err := migrateDefaultTrafficAge(d); err != nil {
				t.Fatalf("迁移失败: %v", err)
			}

			var got model.Setting
			if err := d.Where("key = ?", "trafficAge").First(&got).Error; err != nil {
				t.Fatalf("回读失败: %v", err)
			}
			if got.Value != tc.want {
				t.Fatalf("trafficAge: 起始 %s,期望 %s,实得 %s", tc.start, tc.want, got.Value)
			}

			var neighbor model.Setting
			if err := d.Where("key = ?", "sessionMaxAge").First(&neighbor).Error; err != nil {
				t.Fatalf("回读邻居失败: %v", err)
			}
			if neighbor.Value != "30" {
				t.Fatalf("迁移改到了别的 key:sessionMaxAge 从 30 变成 %s —— "+
					"WHERE 少了 key 限定", neighbor.Value)
			}

			// 幂等:再跑一次不应再变
			if err := migrateDefaultTrafficAge(d); err != nil {
				t.Fatalf("第二次迁移失败: %v", err)
			}
			var again model.Setting
			if err := d.Where("key = ?", "trafficAge").First(&again).Error; err != nil {
				t.Fatalf("二次回读失败: %v", err)
			}
			if again.Value != tc.want {
				t.Fatalf("不幂等:第二次跑完变成 %s", again.Value)
			}
		})
	}
}

