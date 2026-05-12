package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alireza0/s-ui/core"
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
		for i, n := range nodes {
			out[i] = ProbeOutcome{Node: n, Error: "sing-box not running"}
		}
		return out
	}

	sem := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup
	// salt 让多次调用之间 tag 不撞;每次 ProbeNodes 之内 i 已唯一
	salt := common.Random(6)
	var probedOk atomic.Int64
	for i, n := range nodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, n ParsedNode) {
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
	return out
}

func probeOne(ctx context.Context, salt string, idx int, n ParsedNode) ProbeOutcome {
	out := ProbeOutcome{Node: n}
	tag := fmt.Sprintf("%s%s_%d", probeTagPrefix, salt, idx)

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
