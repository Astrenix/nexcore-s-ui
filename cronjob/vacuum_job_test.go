package cronjob

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/alireza0/s-ui/database"
)

// 存量节点的库是 auto_vacuum=0 建出来的,而 DSN 里的 _auto_vacuum 参数
// 【对已存在的库不起作用】—— SQLite 只在建库那一刻定这个模式。
// 所以 VacuumJob 必须自己把存量库切过来(PRAGMA + 一次全量 VACUUM),
// 否则每天的清理照跑、文件照涨,而且不会有任何报错。
//
// 本用例同时钉住那个前提:第一段先证实"打开存量库后 auto_vacuum 仍是 0",
// 如果哪天驱动变得能就地切换,这里会先红,提醒重新评估 VacuumJob 的必要性。
func TestVacuumJobConvertsLegacyDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	// 1. 用不带 _auto_vacuum 的 DSN 建一个"存量库",并写入数据让它真有页
	legacy, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("建存量库失败: %v", err)
	}
	if err := legacy.Exec("CREATE TABLE junk(a TEXT)").Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	for i := 0; i < 200; i++ {
		if err := legacy.Exec("INSERT INTO junk(a) VALUES(?)",
			"padding-padding-padding-padding-padding").Error; err != nil {
			t.Fatalf("塞数据失败: %v", err)
		}
	}
	sqlDB, _ := legacy.DB()
	_ = sqlDB.Close()

	// 2. 走正式路径打开它 —— DSN 带着 _auto_vacuum=incremental
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("InitDB 失败: %v", err)
	}
	d := database.GetDB()

	var beforeMode int
	if err := d.Raw("PRAGMA auto_vacuum").Scan(&beforeMode).Error; err != nil {
		t.Fatalf("读 auto_vacuum 失败: %v", err)
	}
	if beforeMode != 0 {
		t.Fatalf("前提不成立:打开存量库后 auto_vacuum 已是 %d 而非 0 —— "+
			"若驱动现在能就地切换,请重新评估 VacuumJob 的存量分支是否还需要", beforeMode)
	}

	// 3. 制造空闲页:删光数据。此时文件不会变小(auto_vacuum=0)
	if err := d.Exec("DELETE FROM junk").Error; err != nil {
		t.Fatalf("删数据失败: %v", err)
	}
	var freeBefore int
	if err := d.Raw("PRAGMA freelist_count").Scan(&freeBefore).Error; err != nil {
		t.Fatalf("读 freelist 失败: %v", err)
	}
	if freeBefore == 0 {
		t.Fatal("前提不成立:删完数据 freelist_count 仍为 0,造不出待回收的页")
	}

	// 4. 跑作业
	NewVacuumJob().Run()

	var afterMode int
	if err := database.GetDB().Raw("PRAGMA auto_vacuum").Scan(&afterMode).Error; err != nil {
		t.Fatalf("回读 auto_vacuum 失败: %v", err)
	}
	if afterMode != 2 {
		t.Fatalf("VacuumJob 跑完 auto_vacuum 应为 2(INCREMENTAL),实得 %d —— "+
			"多半是只发了 PRAGMA 没跟 VACUUM 重建,那样是静默无效", afterMode)
	}

	var freeAfter int
	if err := database.GetDB().Raw("PRAGMA freelist_count").Scan(&freeAfter).Error; err != nil {
		t.Fatalf("回读 freelist 失败: %v", err)
	}
	if freeAfter >= freeBefore {
		t.Fatalf("空闲页没被回收:%d → %d", freeBefore, freeAfter)
	}
}
