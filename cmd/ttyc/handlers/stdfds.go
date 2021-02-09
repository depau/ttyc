package handlers

import (
	"bytes"
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/Depau/ttyc/utils"
	"github.com/Depau/ttyc/ws"
	"github.com/containerd/console"
	"os"
	"os/signal"
	"syscall"
)

// Tio-style (https://tio.github.io) console handler

const ClearSequence = "\033c"
const (
	EscapeChar byte = 0x14 // Ctrl+T
	HelpChar   byte = '?'
	QuitChar   byte = 'q'
	ConfigChar byte = 'c'
	ClearChar  byte = 'l'
	CtrlTChar  byte = 't'
)

var cmdsHelp = map[byte]string{
	HelpChar:   "List available key commands",
	ConfigChar: "Show configuration",
	ClearChar:  "Clear screen",
	QuitChar:   "Quit",
	CtrlTChar:  "Send ctrl-t key code",
}

type stdfdsHandler struct {
	client           *ws.Client
	console          *console.Console
	ttyConf          *ttyc.SttyDTO
	expectingCommand bool
}

func NewStdFdsHandler(client *ws.Client, ttyConf *ttyc.SttyDTO) (tty TtyHandler, err error) {
	tty = &stdfdsHandler{
		client:           client,
		console:          nil,
		ttyConf:          ttyConf,
		expectingCommand: false,
	}
	return
}

func (s *stdfdsHandler) handleStdin(closeChan <-chan interface{}, inChan <-chan []byte, outChan chan<- []byte, errChan chan<- error) {
	for {
		var input []byte

		//println("SELECT handleStdin")
		select {
		case <-closeChan:
			return
		case input = <-inChan:
		}
		//println("SELECTED handleStdin")

		// Check for new EscapeChars before handling any pending ones, since we may add one back that needs to be
		// passed through
		escapePos := bytes.Index(input, []byte{EscapeChar})

		// Handle any pending commands, when EscapeChar was the last char of the previous buffer
		if s.expectingCommand {
			replacement := s.handleCommand(input[0], errChan)
			s.expectingCommand = false
			input = append(replacement, input[1:]...)

			if escapePos >= 0 {
				// Adjust the pre-existing escape char position based on the characters we added/removed to/from the
				// input buffer
				escapePos += 1 - len(replacement)
			}
		}

		// Handle new EscapeChars
		if escapePos >= 0 && escapePos == len(input)-1 {
			// Escape char is the last char, we need to handle it at the next iteration
			s.expectingCommand = true
			if len(input) == 1 {
				continue
			}
			input = input[:len(input)-1]
		} else if escapePos >= 0 {
			before := input[:escapePos]
			command := input[escapePos]
			after := input[escapePos+2:]
			replacement := s.handleCommand(command, errChan)
			input = bytes.Join([][]byte{before, after}, replacement)
		}

		// More than one escape char? I hope you're happy with your life.

		outChan <- input
	}
}

func (s *stdfdsHandler) handleCommand(command byte, errChan chan<- error) []byte {
	switch command {
	case QuitChar:
		errChan <- fmt.Errorf("quitting")
	case ConfigChar:
		println("")
		ttyc.TtycPrintf("Configuration:\n")
		ttyc.TtycPrintf(" Remote server: %s\n", s.client.WsClient.RemoteAddr().String())
		if s.ttyConf != nil {
			ttyc.TtycPrintf(" Baudrate: %d\n", *s.ttyConf.Baudrate)
			ttyc.TtycPrintf(" Databits: %d\n", *s.ttyConf.Databits)
			ttyc.TtycPrintf(" Flow: soft\n")
			ttyc.TtycPrintf(" Stopbits: %d\n", *s.ttyConf.Stopbits)
			if s.ttyConf.Parity == nil {
				ttyc.TtycPrintf(" Parity: none\n")
			} else {
				ttyc.TtycPrintf(" Parity: %s\n", *s.ttyConf.Parity)
			}
		}
	case ClearChar:
		// Clear screen using ANSI/VT100 escape code
		print(ClearSequence)
		_ = os.Stdout.Sync()
	case CtrlTChar:
		// Put back escape char into buffer
		return []byte{EscapeChar}
	case HelpChar:
		println("")
		ttyc.TtycPrintf("Key commands:\n")
		for key, val := range cmdsHelp {
			ttyc.TtycPrintf(" ctrl-t %c   %s\n", key, val)
		}
	}

	return []byte{}
}

func (s *stdfdsHandler) Run(errChan chan<- error) {
	current := console.Current()
	s.console = &current
	if err := current.SetRaw(); err != nil {
		errChan <- err
		return
	}
	winSize, err := current.Size()
	if err != nil {
		errChan <- err
		return
	}
	//println("RESIZE TERM")
	s.client.ResizeTerminal(int(winSize.Width), int(winSize.Height))
	//println("TERM RESIZED")

	cmdHandlingChan := make(chan []byte, 1)
	go utils.CopyReaderToChan(s.client.CloseChan, os.Stdin, cmdHandlingChan, errChan)
	go s.handleStdin(s.client.CloseChan, cmdHandlingChan, s.client.Input, errChan)
	go utils.CopyChanToWriter(s.client.CloseChan, s.client.Output, os.Stdout, errChan)

	winch := make(chan os.Signal)
	defer close(winch)
	signal.Notify(winch, syscall.SIGWINCH)

	for {
		//println("SELECT stdfds Run")
		select {
		case <-s.client.CloseChan:
			//println("SELECTED stdfds Run")
			return
		case <-winch:
			//println("SELECTED stdfds Run winch")
			if winSize, err := current.Size(); err != nil {
				errChan <- err
				return
			} else {
				s.client.ResizeTerminal(int(winSize.Width), int(winSize.Height))
			}
		}
	}
}

func (s *stdfdsHandler) Close() error {
	if s.console != nil {
		if err := (*s.console).Reset(); err != nil {
			return err
		}
		s.console = nil
	}
	return nil
}
