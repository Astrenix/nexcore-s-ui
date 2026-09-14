package app

import (
	"log"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/cronjob"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"
	"github.com/alireza0/s-ui/web"

	"github.com/op/go-logging"
)

type APP struct {
	service.SettingService
	configService *service.ConfigService
	webServer     *web.Server
	cronJob       *cronjob.CronJob
	logger        *logging.Logger
	core          *core.Core
}

func NewApp() *APP {
	return &APP{}
}

func (a *APP) Init() error {
	log.Printf("%v %v", config.GetName(), config.GetVersion())

	a.initLog()

	// 按本机内存设 Go 软内存上限。节点常是 1 核 1G 的小机器,不设上限时
	// 堆涨到哪算哪,最后由内核 OOM killer 决定 —— 表现是周期性掉线而日志
	// 里什么都没有(被 SIGKILL 的进程写不出遗言)。放在 initLog 之后是为了
	// 这条结论能进日志;放在 InitDB 之前是因为 sqlite 一开就开始占内存。
	if limit, src := util.ConfigureMemoryLimit(); limit > 0 {
		logger.Info("Go 内存软上限已设为 ", limit>>20, " MiB(依据:", src, ")")
	} else if src != "" {
		logger.Debug("未设置 Go 内存软上限:", src)
	}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}

	// AUDIT.md C1:DB 起来后立即把 users.password 字段从明文升级到 bcrypt。
	// 启动时跑一次成本很低(只对未升级的行做 GenerateFromPassword);后续启动
	// 检测到全部 hash 直接空跑返回。
	service.UpgradePlaintextPasswords()

	// Init Setting
	a.SettingService.GetAllSetting()

	a.core = core.NewCore()

	a.cronJob = cronjob.NewCronJob()
	a.webServer = web.NewServer()

	a.configService = service.NewConfigService(a.core)

	return nil
}

func (a *APP) Start() error {
	loc, err := a.SettingService.GetTimeLocation()
	if err != nil {
		return err
	}

	trafficAge, err := a.SettingService.GetTrafficAge()
	if err != nil {
		return err
	}

	err = a.cronJob.Start(loc, trafficAge)
	if err != nil {
		return err
	}

	err = a.webServer.Start()
	if err != nil {
		return err
	}

	err = a.configService.StartCore()
	if err != nil {
		logger.Error(err)
	}

	return nil
}

func (a *APP) Stop() {
	a.cronJob.Stop()
	err := a.webServer.Stop()
	if err != nil {
		logger.Warning("stop Web Server err:", err)
	}
	err = a.configService.StopCore()
	if err != nil {
		logger.Warning("stop Core err:", err)
	}
}

func (a *APP) initLog() {
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatal("unknown log level:", config.GetLogLevel())
	}
}

func (a *APP) RestartApp() {
	a.Stop()
	a.Start()
}

func (a *APP) GetCore() *core.Core {
	return a.core
}
