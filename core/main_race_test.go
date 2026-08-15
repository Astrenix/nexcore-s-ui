package core

import (
	"sync"
	"testing"
)

// 用 `go test -race` 跑:验证 Core 的 isRunning/instance 读写在
// Start/Stop(写)与 IsRunning/GetInstance/GetCtx/LastError(读)并发下被 mu
// 正确串行化。加锁前这里必报 data race。
//
// 不触发真实 sing-box 启动(NewBox 需要合法配置);用 nil-instance 的 Stop 路径
// 模拟写方对共享字段的写入,足以让 race detector 检出未同步访问。
func TestCoreConcurrentAccessRace(t *testing.T) {
	c := NewCore()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 读方:多 goroutine 持续读 4 个 getter
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.IsRunning()
					_ = c.GetInstance()
					_ = c.GetCtx()
					_ = c.LastError()
				}
			}
		}()
	}

	// 写方:反复 Stop(instance==nil 分支只写 isRunning,不需要真实 box)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.Stop()
				}
			}
		}()
	}

	// 让并发跑一小会儿(不用时钟,循环计数即可)
	for i := 0; i < 200000; i++ {
		_ = c.IsRunning()
	}
	close(stop)
	wg.Wait()
}
