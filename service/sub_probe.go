package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/core"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

const (
	// probeConcurrency = 8:跟主流客户端(v2rayN / Clash)的并发测速保持同量级。
	// 之前怀疑"机场单 password 并发限制"是误判 — 实测降到 2 也没救回 SG/US,
	// 说明失败原因不是并发,而是 IP 信誉(数据中心 IP 进部分节点风控)或网络可达性
	// (yidong/2aspx 这种节点本机 nc 就 timeout)。失败原因落 sub_nodes.last_error
	// 给前端展示,用户能直接区分"我探测器问题"vs"节点真不通"。
	probeConcurrency = 8
	probeTagPrefix   = "__probe_"
	// 失败重试:第一次拨号失败 sleep 后再试一次(只重试一次,避免拖死整批)。
	// 主要救场景:CF anycast 路由瞬时抖动、TLS 握手随机失败、TCP 半握手丢包。
	probeRetrySleep = 2 * time.Second
)

// ProbeOutcome ProbeNodes 单条节点的完整探测结果。
type ProbeOutcome struct {
	Node      ParsedNode
	ExitIP    string
	LatencyMs int
	Country   string
	Alive     bool
	Error     string

	// Skipped 表示【这条压根没探测】,不是「探测了判定为死」。
	// 两者绝不能混:没探测就写 Alive=false 会把上一轮的真实存活状态覆盖掉,
	// 而 ElectWinners 只认 alive=true —— 于是某国家可能一个 winner 都选不出来。
	Skipped bool
}

// ProbeNodes 并发探测一批解析后的节点。
//
// 实现:
//   - 每条节点临时挂载到 sing-box outbound_manager(throwaway tag `__probe_<rand>_<i>`)
//   - 通过 core.ProbeOutboundByTag 跑 cloudflare trace 拿 exit IP + 延迟
//   - 拿完立即 RemoveOutbound(失败也 remove,避免悬挂)
//   - Concurrency = probeConcurrency 信号量限并发
//   - 国家识别:remark 关键词 → exit_ip 兜底(当前 keyword-only)
//
// 注意:依赖 corePtr 已 running。调用前需自己保证。
func ProbeNodes(ctx context.Context, nodes []ParsedNode) []ProbeOutcome {
	out := make([]ProbeOutcome, len(nodes))
	if corePtr == nil || !corePtr.IsRunning() {
		// 🩸 core 不在跑 = 一条都没探成,不是"全部探测出来是死的"。
		// 不标 Skipped 的话,applyOutcomes 会把【整个订阅】写成 alive=false,
		// ElectWinners 随即摘掉所有 pool-{cc} 出站 —— 而 core 只是短暂不在
		// (CheckCoreJob 每 5s 自愈,SubRefreshJob 每 1min 跑,两者会撞上)。
		markRemainingSkipped(out, nodes, 0, "sing-box not running")
		return out
	}

	sem := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup
	// salt 让多次调用之间 tag 不撞;每次 ProbeNodes 之内 i 已唯一
	salt := common.Random(6)
	var probedOk atomic.Int64
	skipped := 0
	for i, n := range nodes {
		// 🩸 时间预算用完就【停下来,别继续探】。
		//
		// 调用方给的 ctx 是有限的(SubRefreshJob 给 5 分钟)。一旦它过期,
		// probeOne 里所有 context.WithTimeout(ctx, ...) 派生出来的都【立刻】done ——
		// 剩下的节点会在毫秒内全部"探测失败",然后被 applyOutcomes 写成 alive=false。
		//
		// 这不是截断几条:并发 8、单节点要跑三次代理往返(延迟命中 + 测稳定 RTT +
		// 取出口 IP),而 subMaxNodes 允许 1000 条 —— 125 波 × 每波 2.4 秒才够,
		// 大订阅必然撞上。撞上之后【剩余全部】被判死,而节点顺序稳定,
		// 于是每轮死的都是同一批尾部节点;某国家的节点若都在尾部,
		// ElectWinners 就给不出 winner,pool-{cc} 出站被摘 —— 用户直接断线。
		//
		// 标 Skipped 交给 applyOutcomes:那些行不改探测相关的列,保留上一轮结果。
		if ctx.Err() != nil {
			markRemainingSkipped(out, nodes, i, "probe budget exhausted: "+ctx.Err().Error())
			skipped = len(nodes) - i
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, n ParsedNode) {
			// E564:probeOne 处理【外部节点的响应】,panic 面在别人手里。
			// Recover 放最前(defer 是 LIFO,它最后执行),wg.Done 与信号量归还照常先跑,
			// 否则 wg.Wait() 会永远挂着 —— 那比进程退出更难查。
			defer common.Recover("sub_probe: 并发探测节点")
			defer wg.Done()
			defer func() { <-sem }()
			outcome := probeOne(ctx, salt, i, n)
			if outcome.Alive {
				probedOk.Add(1)
			}
			out[i] = outcome
		}(i, n)
	}
	wg.Wait()
	if skipped > 0 {
		// 必须出声:没有这条,"大订阅每轮尾部节点被判死"完全没有可观测信号 ——
		// 库里只会看到一批 alive=false,和真的挂了长得一模一样。
		logger.Warningf("ProbeNodes: 时间预算耗尽,%d/%d 个节点未探测(已保留上一轮存活状态);"+
			"考虑调大 RefreshSub 的超时或减少订阅节点数", skipped, len(nodes))
	}
	return out
}

// markRemainingSkipped 把 out[from:] 全部标成【没探测】。
//
// 两个调用方(core 不在跑 / 时间预算耗尽)收敛到这里,是因为它们要表达的是
// 同一件事:这些节点【没被测过】。这跟"测过了,是死的"必须在数据上可区分 ——
// applyOutcomes 靠 Skipped 决定要不要覆盖 alive/exit_ip/latency 那几列。
// 曾经两处都只留下 Alive 的零值,于是"没测成"被写成了"测出来是死的"。
func markRemainingSkipped(out []ProbeOutcome, nodes []ParsedNode, from int, reason string) {
	for j := from; j < len(nodes); j++ {
		out[j] = ProbeOutcome{Node: nodes[j], Skipped: true, Error: "skipped: " + reason}
	}
}

func probeOne(ctx context.Context, salt string, idx int, n ParsedNode) ProbeOutcome {
	out := ProbeOutcome{Node: n}
	tag := fmt.Sprintf("%s%s_%d", probeTagPrefix, salt, idx)

	// SSRF 防护:节点 server 来自不可信机场订阅,若指向内网/回环/metadata,
	// 拨号会把面板变成内网端口探测器。直接判 dead,不挂 outbound。
	if probeTargetBlocked(n.Server) {
		out.Error = "blocked: internal/unreachable server address"
		out.Country = detectCountry(n.Remark, "")
		return out
	}

	// 构造完整 outbound config(type/tag + options)
	var optMap map[string]any
	if err := json.Unmarshal(n.Options, &optMap); err != nil {
		out.Error = "options unmarshal: " + err.Error()
		out.Country = detectCountry(n.Remark, "")
		return out
	}
	optMap["type"] = n.Type
	optMap["tag"] = tag
	cfg, err := json.Marshal(optMap)
	if err != nil {
		out.Error = "options marshal: " + err.Error()
		out.Country = detectCountry(n.Remark, "")
		return out
	}

	// AddOutbound 失败 → 配置非法,这种节点直接判 dead 不用 probe
	if err := corePtr.AddOutbound(cfg); err != nil {
		out.Error = "AddOutbound: " + err.Error()
		out.Country = detectCountry(n.Remark, "")
		return out
	}
	// 无论 probe 成败都要 remove,避免 outbound_manager 越来越脏
	defer func() {
		_ = corePtr.RemoveOutbound(tag)
	}()

	res := core.ProbeOutboundByTag(ctx, tag)
	// 第一次失败 → sleep 后重试一次(同一 outbound 还挂着,无需重新 AddOutbound)
	// 重试主要救:并发节流的瞬时 RST、机场风控引擎的"首次拒绝"模式
	if !res.OK {
		// 上下文已超时就别再试,直接返
		select {
		case <-ctx.Done():
			out.Error = res.Error + " (ctx done)"
			out.Country = detectCountry(n.Remark, "")
			return out
		case <-time.After(probeRetrySleep):
		}
		res = core.ProbeOutboundByTag(ctx, tag)
	}
	if !res.OK {
		out.Error = res.Error
		out.Country = detectCountry(n.Remark, "")
		return out
	}
	out.ExitIP = res.ExitIP
	out.LatencyMs = res.LatencyMs
	out.Alive = true
	out.Country = detectCountry(n.Remark, res.ExitIP)
	return out
}
