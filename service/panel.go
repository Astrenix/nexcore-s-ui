package service

import (
	"github.com/alireza0/s-ui/util/common"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/alireza0/s-ui/logger"
)

type PanelService struct {
}

func (s *PanelService) RestartPanel(delay time.Duration) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	go func() {
		defer common.Recover("panel: 延迟重启面板")
		time.Sleep(delay)
		if runtime.GOOS == "windows" {
			err = p.Kill()
		} else {
			err = p.Signal(syscall.SIGHUP)
		}
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
