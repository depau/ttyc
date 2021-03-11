package handlers

import (
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/Depau/ttyc/utils"
	"github.com/Depau/ttyc/ws"
	"github.com/containerd/console"
	"os"
)

type ptyHandler struct {
	client    *ws.Client
	pty       console.Console
	slavePath string
}

func NewPtyHandler(client *ws.Client, linkTo string) (tty TtyHandler, err error) {
	stat, statErr := os.Lstat(linkTo)
	if statErr == nil {
		if stat.Mode()&os.ModeSymlink != 0 {
			err = os.Remove(linkTo)
			if err != nil {
				ttyc.Trace()
				err = fmt.Errorf("tty filename exists and it can't be removed: %v", err)
				return nil, err
			}
		} else {
			err = fmt.Errorf("tty file exists: %s", linkTo)
			return nil, err
		}
	}

	pty, slavePath, err := console.NewPty()
	if err != nil {
		ttyc.Trace()
		return nil, err
	}
	if err = os.Symlink(slavePath, linkTo); err != nil {
		ttyc.TtycAngryPrintf("Warning: unaable to create link to %s as requested: %v\n", linkTo, err)
		ttyc.TtycAngryPrintf("You can still access it at %s\n", slavePath)
	} else {
		ttyc.TtycPrintf("TTY connected to remote terminal, available at %s\n", linkTo)
	}

	tty = &ptyHandler{
		client:    client,
		slavePath: slavePath,
		pty:       pty,
	}

	return
}

func (p *ptyHandler) Run(errChan chan<- error) {
	go utils.CopyChanToWriter(p.client.CloseChan, p.client.Output, p.pty, errChan)
	go utils.CopyReaderToChan(p.client.CloseChan, p.pty, p.client.Input, errChan)
	for {
		select {
		case <-p.client.CloseChan:
			return
		case title := <-p.client.WinTitle:
			ttyc.TtycPrintf("Title: %s\n", title)
		case baudResult := <-p.client.DetectedBaudrate:
			approx := baudResult[0]
			measured := baudResult[1]
			if approx <= 0 {
				ttyc.TtycAngryPrintf("Baudrate detection was not successful (detection only works while input is received)\n")
				break
			}
			if measured > 0 {
				ttyc.TtycPrintf("Detected baudrate: likely %d bps (measured rate: %d bps)\n", approx, measured)
			} else {
				ttyc.TtycPrintf("Detected baudrate: likely %d bps\n", approx)
			}
		}
	}
}

func (p *ptyHandler) HandleDisconnect() error {
	return nil
}

func (p *ptyHandler) HandleReconnect() error {
	return nil
}

func (p *ptyHandler) Close() error {
	return p.pty.Close()
}
