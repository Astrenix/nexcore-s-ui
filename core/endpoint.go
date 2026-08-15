package core

import (
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

// 本文件所有方法全程持 c.mu.RLock:读 isRunning + 使用包级 manager/globalCtx/factory
// 期间,Start/Stop(写锁)不能并发换掉这些变量。RWMutex 不可重入,故内部一律直接
// 读包级 globalCtx,不回调持锁的 c.GetCtx()。RLock 允许多个 AddX/RemoveX 并发,
// sing-box 的 manager 自身线程安全。

func (c *Core) AddInbound(config []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var inbound_config option.Inbound
	err = inbound_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = inbound_manager.Create(
		globalCtx,
		router,
		factory.NewLogger("inbound/"+inbound_config.Type+"["+inbound_config.Tag+"]"),
		inbound_config.Tag,
		inbound_config.Type,
		inbound_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveInbound(tag string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove inbound: ", tag)
	return inbound_manager.Remove(tag)
}

func (c *Core) AddOutbound(config []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var outbound_config option.Outbound

	err = outbound_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	outboundCtx := adapter.WithContext(globalCtx, &adapter.InboundContext{
		Outbound: outbound_config.Tag,
	})

	err = outbound_manager.Create(
		outboundCtx,
		router,
		factory.NewLogger("outbound/"+outbound_config.Type+"["+outbound_config.Tag+"]"),
		outbound_config.Tag,
		outbound_config.Type,
		outbound_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveOutbound(tag string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove outbound: ", tag)
	return outbound_manager.Remove(tag)
}

func (c *Core) AddEndpoint(config []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var endpoint_config option.Endpoint

	err = endpoint_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = endpoint_manager.Create(
		globalCtx,
		router,
		factory.NewLogger("endpoint/"+endpoint_config.Type+"["+endpoint_config.Tag+"]"),
		endpoint_config.Tag,
		endpoint_config.Type,
		endpoint_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveEndpoint(tag string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove endpoint: ", tag)
	return endpoint_manager.Remove(tag)
}

func (c *Core) AddService(config []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var srv_config option.Service

	err = srv_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = service_manager.Create(
		globalCtx,
		factory.NewLogger("service/"+srv_config.Type+"["+srv_config.Tag+"]"),
		srv_config.Tag,
		srv_config.Type,
		srv_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveService(tag string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove service: ", tag)
	return service_manager.Remove(tag)
}
