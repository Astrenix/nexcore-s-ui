package core

import (
	"context"
	"time"

	urltest "github.com/sagernet/sing-box/common/urltest"
)

const checkTimeout = 15 * time.Second

// defaultProbeURL — sing-box urltest 默认目标是 www.gstatic.com/generate_204,
// 但 Google 部分机房(尤其国内 / 东亚 IDC)对全球 Google IP 段有 BGP 路由黑洞,
// DNS 还经常解析到不可达的 CN GFE IP(203.208.x.x),dial 直接 timeout 让用户
// 看到的延迟测试是空。
//
// cp.cloudflare.com/generate_204 是 cloudflare 官方 captive portal 探测地址,
// anycast 全球边缘,在韩国 / 日本 / 新加坡机房实测 < 20ms,基本不会被 BGP
// 黑洞;协议上跟 Google 那条等价(http 200/204 + 空 body)。
const defaultProbeURL = "http://cp.cloudflare.com/generate_204"

type CheckOutboundResult struct {
	OK    bool
	Delay uint16
	Error string
}

func CheckOutbound(ctx context.Context, tag string, link string) (result CheckOutboundResult) {
	if outbound_manager == nil {
		result.Error = "core not running"
		return result
	}
	ob, ok := outbound_manager.Outbound(tag)
	if !ok {
		result.Error = "outbound not found"
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	if link == "" {
		link = defaultProbeURL
	}
	delay, err := urltest.URLTest(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.Delay = delay
	return result
}
