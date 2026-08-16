package v1

// 节点交付 API —— 主控(NexCore 综合)驱动本节点自动成型的入口。
//
// 这三个端点合起来让主控可以在**不了解 sing-box 配置**的前提下把一台
// 刚装好的空面板变成可售节点:
//
//	GET  /api/v1/sui/provision/presets  能力发现:本节点支持哪些交付预设
//	POST /api/v1/sui/provision/apply    执行交付(建 DNS + 签证书 + 建入站)
//	GET  /api/v1/sui/provision/state    对账:节点当前实际有哪些入站
//
// 安全:全部挂在 authed 组下(Bearer API Token)。apply 会写 DNS 与入站配置,
// 属于高权限操作 —— 主控的透传 allowlist 不应该放开它,只允许主控后端直调。

import (
	"github.com/gin-gonic/gin"

	"github.com/alireza0/s-ui/service"
)

// provisionPresets 能力发现。
//
// 主控在下发交付之前先拉一次:老版本节点不支持新 preset 时,主控能在
// 装机向导里就提示"该节点版本过低",而不是交付跑到一半才失败。
func (a *Controller) provisionPresets(c *gin.Context) {
	OK(c, service.PresetCatalog)
}

// provisionApply 执行一次交付。
//
// 语义是**幂等的声明式 apply**,不是"创建":重复调用同一份意图不会产生
// 第二套入站(锚点是 inbound tag)。主控可以放心重试。
//
// 返回 200 而不是 201:交付可能是"全部复用"(什么都没新建),也可能部分
// 成功部分失败 —— 逐条结果在 data.inbounds[].error 里,HTTP 层只表示
// "这次编排跑完了"。只有前置条件不成立(缺 token / 缺邮箱 / 意图为空)
// 才是 400。
func (a *Controller) provisionApply(c *gin.Context) {
	var req service.ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "invalid_body", err.Error())
		return
	}
	if len(req.Inbounds) == 0 {
		BadRequest(c, "invalid_body", "inbounds 至少要有一条")
		return
	}

	res, err := a.provisionSvc.Apply(req)
	if err != nil {
		// Apply 只在前置条件不成立时返回 error(缺 cf_token / acme_email /
		// 拿不到公网 IP 等),这些都是主控传参问题 → 400 让主控知道该改请求,
		// 而不是无脑重试。
		BadRequest(c, "provision_failed", err.Error())
		return
	}
	OK(c, res)
}

// provisionState 汇报本节点当前的交付状态,供主控对账。
//
// 主控用它发现"漂移":运营在面板里手工删了某条入站,而主控还在按套餐
// 给用户发这条入站的订阅 —— 对账能把这种静默故障暴露出来。
func (a *Controller) provisionState(c *gin.Context) {
	st, err := a.provisionSvc.State()
	if err != nil {
		Internal(c, "db_error", err)
		return
	}
	OK(c, st)
}
