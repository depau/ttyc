// +build windows darwin

// This package provides mock implementations for Windows and macOS since PTY doesn't work

package handlers

import (
	"fmt"
	"github.com/Depau/ttyc/ws"
)

type ptyHandler struct{}

func NewPtyHandler(client *ws.Client, linkTo string) (tty TtyHandler, err error) {
	err = fmt.Errorf("PTY backend is not available on Windows")
	return
}

func (p *ptyHandler) Run(errChan chan<- error) {
	errChan <- fmt.Errorf("PTY backend is not available on Windows")
}

func (p *ptyHandler) HandleDisconnect() error {
	return fmt.Errorf("PTY backend is not available on Windows")
}

func (p *ptyHandler) HandleReconnect() error {
	return fmt.Errorf("PTY backend is not available on Windows")
}

func (p *ptyHandler) Close() error {
	return fmt.Errorf("PTY backend is not available on Windows")
}
