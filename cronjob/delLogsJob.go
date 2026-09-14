package cronjob

import (
	"time"

	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
)

// DelLogsJob 清理两张"只涨不减"的日志表:api_logs 与 changes。
//
// 为什么需要它:
//   - api_logs 此前【没有任何自动清理】。ApiLogService.PruneOlderThan 早就写好,
//     注释还写着"后台 cron 用",但全仓零调用方 —— 典型的「备好了一个没人调的能力」。
//     实测生产节点 50171 行。
//   - changes 同样没有清理路径,只有写入(config/client/cloudflare 三处 tx.Create)。
//
// 与 DelStatsJob 的差别:保留天数在 Run 里【每次现读】而不是启动时固定。
// 面板上改完设置当天就生效,不必重启进程;代价只是每天多两次 settings 查询。
type DelLogsJob struct {
	settingService service.SettingService
	apiLogService  service.ApiLogService
	configService  service.ConfigService
}

func NewDelLogsJob() *DelLogsJob {
	return &DelLogsJob{}
}

func (j *DelLogsJob) Run() {
	// 读设置失败就跳过本轮,不要退回默认值去删 —— 删除不可逆,而"读不到配置"
	// 本身是异常状态,此时猜一个天数去删是拿用户数据赌。下一轮(次日)会重试。
	apiLogAge, err := j.settingService.GetApiLogAge()
	if err != nil {
		logger.Warning("DelLogsJob: 读取 apiLogAge 失败,本轮跳过 api_logs 清理: ", err)
	} else if apiLogAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -apiLogAge).Unix()
		if err := j.apiLogService.PruneOlderThan(cutoff); err != nil {
			logger.Warning("DelLogsJob: 清理 api_logs 失败: ", err)
		} else {
			logger.Debug("DelLogsJob: api_logs 已清理 ", apiLogAge, " 天前的记录")
		}
	}

	changesAge, err := j.settingService.GetChangesAge()
	if err != nil {
		logger.Warning("DelLogsJob: 读取 changesAge 失败,本轮跳过 changes 清理: ", err)
	} else if changesAge > 0 {
		if err := j.configService.DelOldChanges(changesAge); err != nil {
			logger.Warning("DelLogsJob: 清理 changes 失败: ", err)
		} else {
			logger.Debug("DelLogsJob: changes 已清理 ", changesAge, " 天前的记录")
		}
	}
}
