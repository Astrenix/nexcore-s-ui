package core

import (
	"context"
	"sync"

	"github.com/alireza0/s-ui/logger"

	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	_ "github.com/sagernet/sing-box/experimental/clashapi"
	_ "github.com/sagernet/sing-box/experimental/v2rayapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	_ "github.com/sagernet/sing-box/transport/v2rayquic"
	"github.com/sagernet/sing/service"
)

var (
	globalCtx        context.Context
	inbound_manager  adapter.InboundManager
	outbound_manager adapter.OutboundManager
	service_manager  adapter.ServiceManager
	endpoint_manager adapter.EndpointManager
	router           adapter.Router
	factory          log.Factory
)

// Core 的 isRunning/instance/lastErr 以及上面这些包级 manager 变量,过去被
// Start/Stop(Save 触发的重启 goroutine)与 AddX/RemoveX(Save 直接调)、
// IsRunning/GetInstance(CheckCoreJob 5s / StatsJob 10s)多 goroutine 无锁并发
// 读写 —— `-race` 必报,生产上偶发把入站加到已 Close 的 box 或读到撕裂的
// interface 值 → 整个面板进程崩溃。mu 统一串行化这些访问:
//   - 写锁:Start / Stop(整段重建,含改写包级 manager)
//   - 读锁:IsRunning / GetInstance / LastError / GetCtx 及全部 AddX/RemoveX
// RWMutex 不可重入,故 AddX/RemoveX 持读锁期间**直接读包级变量**,不再回调
// 持锁的 GetCtx(),避免自死锁。
type Core struct {
	mu        sync.RWMutex
	isRunning bool
	instance  *Box
	lastErr   string // sing-box 最近一次 start 失败的原因 — 给 /server/status xray.errorMsg 用
}

func NewCore() *Core {
	globalCtx = context.Background()
	globalCtx = sb.Context(globalCtx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
	return &Core{
		isRunning: false,
		instance:  nil,
	}
}

// LastError 返回 sing-box 最近一次 Start 错误描述。Start 成功时清空。
// 给 /server/status xray.errorMsg 字段(x-ui 兼容契约)用。
func (c *Core) LastError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastErr
}

func (c *Core) GetCtx() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return globalCtx
}

func (c *Core) GetInstance() *Box {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instance
}

func (c *Core) Start(sbConfig []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var opt option.Options
	err := opt.UnmarshalJSONContext(globalCtx, sbConfig)
	if err != nil {
		logger.Error("Unmarshal config err:", err.Error())
	}

	c.instance, err = NewBox(Options{
		Context: globalCtx,
		Options: opt,
	})
	if err != nil {
		c.lastErr = err.Error()
		return err
	}

	err = c.instance.Start()
	if err != nil {
		c.lastErr = err.Error()
		_ = c.instance.Close()
		c.instance = nil
		return err
	}
	c.lastErr = ""

	globalCtx = service.ContextWith(globalCtx, c)
	inbound_manager = service.FromContext[adapter.InboundManager](globalCtx)
	outbound_manager = service.FromContext[adapter.OutboundManager](globalCtx)
	service_manager = service.FromContext[adapter.ServiceManager](globalCtx)
	endpoint_manager = service.FromContext[adapter.EndpointManager](globalCtx)
	router = service.FromContext[adapter.Router](globalCtx)

	c.isRunning = true
	return nil
}

func (c *Core) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isRunning = false
	if c.instance == nil {
		return nil
	}
	err := c.instance.Close()
	c.instance = nil
	return err
}

func (c *Core) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}
