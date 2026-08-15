package service

import (
	"sync"
	"testing"
)

// 用 `go test -race` 跑:验证 onlineResources 的 atomic.Pointer 改造消除了
// SaveStats(写快照)与 GetOnlines(读快照)之间的 data race。
// corePtr 为 nil 时 SaveStats 走"构造空快照 → Store"路径,不触碰 sing-box。
func TestOnlineResourcesRace(t *testing.T) {
	svc := &StatsService{}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = svc.GetOnlines()
				}
			}
		}()
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = svc.SaveStats(false)
				}
			}
		}()
	}

	for i := 0; i < 100000; i++ {
		_, _ = svc.GetOnlines()
	}
	close(stop)
	wg.Wait()
}
