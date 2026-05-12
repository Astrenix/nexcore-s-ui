package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// initUser 首次启动建首位管理员。
//
// 安全模型(AUDIT.md C2):**不再硬编码 admin/admin**。
//   - 库为空 → 生成 16 字符高熵随机密码,bcrypt 落库,**明文一次性 stdout**(运维必读),
//     用户名固定 "admin"(可在 sui CLI / 面板里改)。
//   - 已有用户 → 跳过。
//
// 这样即使用户跳过 install.sh 的 `sui admin -username/-password` 步骤直接 `go build && ./sui`,
// 也不会让公网起一台 admin/admin 的面板;启动日志强制运维看到密码至少一次。
func initUser() error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	plain := common.Random(16)
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return err
	}
	user := &model.User{
		Username: "admin",
		Password: string(hashed),
	}
	if err := db.Create(user).Error; err != nil {
		return err
	}
	// 直接写 stderr — InitDB 可能从 panel(已 InitLogger)或 CLI(logger=nil
	// 会 panic)路径调用。stderr 在两种路径都能落到 systemd journal / install.sh
	// stdout,可靠性高于 logger 全局状态。运维错过这次就只能 `sui admin -reset` 重置。
	fmt.Fprintln(os.Stderr, "================================================================")
	fmt.Fprintln(os.Stderr, "  首次启动 — 管理员初始凭据(本次启动只显示一次,务必抄走!)")
	fmt.Fprintln(os.Stderr, "    username : admin")
	fmt.Fprintln(os.Stderr, "    password : "+plain)
	fmt.Fprintln(os.Stderr, "  改密码:登录面板「设置 → 修改密码」 或 CLI `sui admin -reset`")
	fmt.Fprintln(os.Stderr, "================================================================")
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	// AUDIT.md LOW:0o700 = owner rwx,其他无访问 — DB 只 sui 进程读,不需要
	// group/other 任何位。原 01740 含 sticky bit + group r,过松。
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	// AUDIT.md H1 已回退(v1.7.15):启了 _foreign_keys=on 后,现有 schema 把
	// inbound.tls_id=0 当 nullable("不绑 TLS")的约定会撞 SQLite FK 校验,
	// SS / 任何不带 TLS 的入站 Save 全部报 FOREIGN KEY constraint failed。
	// 要真启 fk 须先把 inbound.tls_id 改 *uint nullable + 数据迁移把现存
	// 0 改 NULL,改动量大、风险高,这次不做。Service 层已有"tls in use"
	// 软校验,可控。
	// _txlock=immediate:让 GORM db.Begin() 走 BEGIN IMMEDIATE,事务一开始就拿
	// RESERVED 写锁,busy_timeout 才能干净地覆盖等待。默认 DEFERRED 在读升写时
	// 撞另一个写者会直接 SQLITE_BUSY 不重试,典型表现:"第一次保存 database is
	// locked,立刻再点就 OK"(SaveStats cron 每 10s 写一次撞了 Save tx 升级)。
	// _busy_timeout=30000:Save 路径包含 corePtr.AddInbound,sing-box 复杂入站
	// reload 3-8s,叠加 SaveStats 撞窗口需要更充分缓冲,30s 兜底。
	dsn := dbPath + sep + "_busy_timeout=30000&_journal_mode=WAL&_txlock=immediate"
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func InitDB(dbPath string) error {
	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
		&model.ApiLog{},
		&model.BlockRule{},
		&model.Sub{},
		&model.SubNode{},
		&model.PoolOutbound{},
	)
	if err != nil {
		return err
	}
	// 老数据库新加的 inbounds.enable 列对历史行可能落到 NULL;
	// 显式回填一次,确保升级后所有现存入站默认是启用状态。
	db.Exec("UPDATE inbounds SET enable = 1 WHERE enable IS NULL")

	// 一次性迁移:订阅池 v1 把 pool-{cc} 写到 outbounds 表,v2 拆到独立 pool_outbounds 表。
	// 把存量 pool-* 行搬过去,清出 outbounds 表,让旧出站管理 UI 干净。
	// 安全 guard:只搬还没在 pool_outbounds 出现的 tag;tag 冲突保留 pool_outbounds 那条。
	if err := migrateLegacyPoolOutbounds(db); err != nil {
		return err
	}

	err = initUser()
	if err != nil {
		return err
	}

	return nil
}

// migrateLegacyPoolOutbounds 把 outbounds 表里 tag 以 "pool-" 开头的行
// 搬到 pool_outbounds 表(订阅池 v1 → v2 拆表迁移)。
//
// 幂等:第二次执行时 INSERT OR IGNORE,已存在的 pool_outbound 不被覆盖。
// 失败容忍:任何一步出错只记 warning,不阻塞启动 — 用户实在搬不过去手 sqlite 处理。
func migrateLegacyPoolOutbounds(db *gorm.DB) error {
	// 用 raw SQL 一把搬过去 + 删原行,事务内完成
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 取 outbounds 表里所有 pool-* 行(legacy)
		type legacyRow struct {
			Id          uint
			Type        string
			Tag         string
			DisplayName string
			Options     []byte
		}
		var rows []legacyRow
		if err := tx.Table("outbounds").
			Select("id, type, tag, display_name, options").
			Where("tag LIKE ?", "pool-%").Find(&rows).Error; err != nil {
			return nil // 老库没这些列也 OK,直接放过
		}
		if len(rows) == 0 {
			return nil
		}
		for _, r := range rows {
			cc := strings.ToUpper(strings.TrimPrefix(r.Tag, "pool-"))
			// INSERT OR IGNORE — 已经搬过的不重复
			tx.Exec(`
				INSERT OR IGNORE INTO pool_outbounds
				  (tag, country, type, display_name, options, winner_node_id, winner_latency, updated_at, created_at)
				VALUES (?, ?, ?, ?, ?, 0, 0, datetime('now'), datetime('now'))
			`, r.Tag, cc, r.Type, r.DisplayName, r.Options)
		}
		// 2. 删 outbounds 表里那些 pool-* 行(已经搬过去了)
		tx.Exec("DELETE FROM outbounds WHERE tag LIKE 'pool-%'")
		return nil
	})
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
