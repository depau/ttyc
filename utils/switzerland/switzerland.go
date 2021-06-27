// +build !windows

package switzerland

import (
	"os"
	"os/signal"
	"syscall"
)

type WinchSignal = os.Signal

type nixSwitzerland struct{}

var switzerlandInstance Switzerland = &nixSwitzerland{}

func (s *nixSwitzerland) Notify(c chan<- WinchSignal) {
	signal.Notify(c, syscall.SIGWINCH)
}

func (s *nixSwitzerland) Stop(c chan<- WinchSignal) {
	signal.Stop(c)
}
