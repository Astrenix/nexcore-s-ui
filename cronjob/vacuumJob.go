package cronjob

import (
	"gorm.io/gorm"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
)

// sqliteAutoVacuumIncremental 是 PRAGMA auto_vacuum 的取值 2(0=NONE,1=FULL,2=INCREMENTAL)。
const sqliteAutoVacuumIncremental = 2

// VacuumJob 把删除留下的空闲页还给文件系统。
//
// 🩸 没有这一步,缩短保留天数在磁盘上【一个字节都省不下来】:
// DelStatsJob 每天按 trafficAge 删行,但库建出来时 auto_vacuum=0(SQLite 默认),
// 删掉的页只是进 freelist 等着被复用,文件本身永不收缩。生产节点实测
// 315MB 的库里有 145MB(46%)是这种空闲页 —— 清理一直在跑,磁盘一直在涨。
//
// 两种处境:
//   - auto_vacuum 已是 INCREMENTAL(新库,DSN 里带着建的)→ 每天 incremental_vacuum,
//     增量归还,单次耗时与"这天删了多少"成正比,平滑无停顿。
//   - auto_vacuum=0(存量库)→ 必须先切模式。而切模式对已存在的库【只改标记不生效】,
//     必须跟一次全量 VACUUM 重建文件才算数。这是一次性的:重建完写回 INCREMENTAL,
//     之后都走增量分支。放在 cron 而不是启动路径,是为了不让一次几十秒的重建
//     卡住面板启动 —— 节点起不来意味着主控探活判 offline。
type VacuumJob struct{}

func NewVacuumJob() *VacuumJob {
	return &VacuumJob{}
}

func (j *VacuumJob) Run() {
	db := database.GetDB()
	if db == nil {
		return
	}

	var mode int
	if err := db.Raw("PRAGMA auto_vacuum").Scan(&mode).Error; err != nil {
		logger.Warning("VacuumJob: 读取 auto_vacuum 失败: ", err)
		return
	}

	before := dbFileBytes(db)

	if mode == sqliteAutoVacuumIncremental {
		if err := db.Exec("PRAGMA incremental_vacuum").Error; err != nil {
			logger.Warning("VacuumJob: incremental_vacuum 失败: ", err)
			return
		}
	} else {
		// 顺序不能反:先声明目标模式,再用 VACUUM 重建文件让它落地。
		// 只发 PRAGMA 不 VACUUM = 静默无效(库照样 auto_vacuum=0)。
		if err := db.Exec("PRAGMA auto_vacuum=INCREMENTAL").Error; err != nil {
			logger.Warning("VacuumJob: 设置 auto_vacuum 失败: ", err)
			return
		}
		if err := db.Exec("VACUUM").Error; err != nil {
			logger.Warning("VacuumJob: VACUUM 失败: ", err)
			return
		}
		logger.Info("VacuumJob: 已把数据库切到 auto_vacuum=INCREMENTAL(一次性重建,后续走增量回收)")
	}

	after := dbFileBytes(db)
	if before > 0 && after > 0 && before != after {
		logger.Info("VacuumJob: 数据库文件 ", before/1048576, "MB → ", after/1048576, "MB")
	}
}

// dbFileBytes 返回当前数据库文件占用的字节数(page_count × page_size)。
// 取不到时返回 0,调用方据此跳过日志 —— 回收本身不依赖它。
func dbFileBytes(db *gorm.DB) int64 {
	var pageCount, pageSize int64
	if err := db.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		return 0
	}
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		return 0
	}
	return pageCount * pageSize
}
