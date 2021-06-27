package switzerland

import (
	"fmt"
	"github.com/containerd/console"
	"sync"
	"time"
)

type WinchSignal = interface{}

type winSwitzerland struct {
	handlers  []chan<- WinchSignal
	isRunning bool
	lock      sync.Mutex
}

var switzerlandInstance Switzerland = &winSwitzerland{
	[]chan<- WinchSignal{},
	false,
	sync.Mutex{},
}

func (s *winSwitzerland) swissWorker() {
	stdin := console.Current()
	prevSize, err := stdin.Size()
	if err != nil {
		panic(fmt.Sprintf("failed to retrieve Windows console size: %v", err))
	}

	s.lock.Lock()
	s.isRunning = true
	s.lock.Unlock()

	for {
		s.lock.Lock()
		if len(s.handlers) > 0 {
			newSize, err := stdin.Size()
			if err != nil {
				panic(fmt.Sprintf("failed to retrieve Windows console size: %v", err))
			}
			if newSize.Height != prevSize.Height || newSize.Width != prevSize.Width {
				for _, c := range s.handlers {
					c <- nil
				}
			}
			prevSize = newSize
		} else {
			s.isRunning = false
			s.lock.Unlock()
			return
		}
		s.lock.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *winSwitzerland) Notify(c chan<- WinchSignal) {
	s.lock.Lock()
	defer s.lock.Unlock()

	exists := false
	for _, i := range s.handlers {
		if i == c {
			exists = true
			break
		}
	}
	if !exists {
		s.handlers = append(s.handlers, c)
	}

	if !s.isRunning {
		go s.swissWorker()
	}
}

func (s *winSwitzerland) Stop(c chan<- WinchSignal) {
	s.lock.Lock()
	for i := 0; i < len(s.handlers); i++ {
		if s.handlers[i] == c {
			if len(s.handlers) > 1 {
				s.handlers[i] = s.handlers[len(s.handlers)-1]
			}
			s.handlers = s.handlers[:len(s.handlers)-1]
			break
		}
	}
	s.lock.Unlock()
}
