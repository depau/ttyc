package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/Depau/ttyc/cmd/ttyc/handlers/shenanigans"
	"github.com/Depau/ttyc/utils"
	"github.com/Depau/ttyc/utils/switzerland"
	"github.com/Depau/ttyc/ws"
	"github.com/TwinProduction/go-color"
	"github.com/containerd/console"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"time"
)

// Tio-style (https://tio.github.io) console handler

const ClearSequence = "\033c"
const (
	EscapeChar     byte = 0x14 // Ctrl+T
	HelpChar       byte = '?'
	QuitChar       byte = 'q'
	ConfigChar     byte = 'c'
	BreakChar      byte = 'b'
	DetectBaudChar byte = 'B'
	ClearChar      byte = 'l'
	CtrlTChar      byte = 't'
	VersionChar    byte = 'v'
	StatsChar      byte = 's'
	LocalEchoChar  byte = 'e'
	HexModeChar    byte = 'h'
	TimestampsChar byte = 'T'
)

type StatsDTO struct {
	Tx     int64 `json:"tx"`
	Rx     int64 `json:"rx"`
	TxRate int64 `json:"txRateBps"`
	RxRate int64 `json:"rxRateBps"`
}

type cmdInfo struct {
	HelpText    string
	NonStandard bool
}

var cmdsInfo = map[byte]cmdInfo{
	// Available for all implementations
	QuitChar:       {"Quit", false},
	ClearChar:      {"Clear screen", false},
	CtrlTChar:      {"Send ctrl-t key code", false},
	HelpChar:       {"List available key commands", false},
	ConfigChar:     {"Show configuration", false},
	VersionChar:    {"Show version", false},
	LocalEchoChar:  {"Toggle local echo mode", false},
	HexModeChar:    {"Toggle hexadecimal mode", false},
	TimestampsChar: {"Toggle timestamps", false},
	// Available on Wi-Se server only
	BreakChar:      {"Send break", true},
	DetectBaudChar: {"Request baudrate detection", true},
	StatsChar:      {"Show statistics", true},
}

type stdfdsHandler struct {
	client           *ws.Client
	console          *console.Console
	implementation   ttyc.Implementation
	credentials      *url.Userinfo
	server           string
	expectingCommand bool
	localEchoMode    bool
	hexMode          bool
	showTimestamps   bool
	nextIsTimestamp  bool
}

func NewStdFdsHandler(client *ws.Client, implementation ttyc.Implementation, credentials *url.Userinfo, server string) (tty TtyHandler, err error) {
	tty = &stdfdsHandler{
		client:           client,
		implementation:   implementation,
		credentials:      credentials,
		server:           server,
		console:          nil,
		expectingCommand: false,
		localEchoMode:    false,
		hexMode:          false,
		showTimestamps:   false,
		nextIsTimestamp:  false,
	}
	return
}

func (s *stdfdsHandler) rawTtyPrintfLn(angry bool, format string, args ...interface{}) {
	var newLineFile *os.File
	if angry {
		ttyc.TtycAngryPrintf(format, args...)
		newLineFile = os.Stderr
	} else {
		ttyc.TtycPrintf(format, args...)
		newLineFile = os.Stdout
	}
	if s.console == nil {
		_, _ = newLineFile.WriteString("\n")
	} else {
		_, _ = newLineFile.WriteString("\r\n")
	}
	_ = newLineFile.Sync()
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

		if s.localEchoMode {
			for _, char := range input {
				// If character is printable
				if (char >= 32 && char <= 126) || char == '\r' || char == '\n' {
					_, _ = os.Stdout.Write([]byte{char})
				}
			}
			_ = os.Stdout.Sync()
		}

		outChan <- input
	}
}

func (s *stdfdsHandler) printStats() {
	statsUrl := ttyc.GetUrlFor(ttyc.UrlForStats, s.client.BaseUrl)
	res, err := http.Get(statsUrl.String())
	if err != nil {
		ttyc.Trace()
		s.rawTtyPrintfLn(true, "Failed to get stats: %v", err)
		return
	}
	res, err = utils.EnsureAuth(res, s.credentials, nil)
	if err != nil {
		ttyc.Trace()
		s.rawTtyPrintfLn(true, "Failed to get stats: %v", err)
		return
	}
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		ttyc.Trace()
		s.rawTtyPrintfLn(true, "Failed to get stats: %v", err)
		return
	}
	stats := StatsDTO{}
	err = json.Unmarshal(buf, &stats)
	if err != nil {
		ttyc.Trace()
		s.rawTtyPrintfLn(true, "Failed to get stats: %v", err)
		return
	}
	s.rawTtyPrintfLn(false, "Statistics:")
	s.rawTtyPrintfLn(false, " Sent %d bytes, received %d bytes, tx %d bps, rx %d bps", stats.Tx, stats.Rx, stats.TxRate, stats.RxRate)
}

func (s *stdfdsHandler) handleCommand(command byte, errChan chan<- error) []byte {
	switch command {
	case QuitChar:
		println("")
		errChan <- fmt.Errorf("quitting")
	case ConfigChar:
		println("")
		s.rawTtyPrintfLn(false, "Configuration:")

		additionalServerInfo := ""
		if s.server != "" {
			additionalServerInfo = fmt.Sprintf(" (%s)", s.server)
		}
		wsUrl := ttyc.GetUrlFor(ttyc.UrlForWebSocket, s.client.BaseUrl)
		s.rawTtyPrintfLn(false, " Remote server: %s%s", wsUrl.String(), additionalServerInfo)

		if s.implementation == ttyc.ImplementationWiSe {
			sttyUrl := ttyc.GetUrlFor(ttyc.UrlForStty, s.client.BaseUrl)
			ttyConf, err := ttyc.GetStty(sttyUrl, s.credentials)
			if err == nil {
				s.rawTtyPrintfLn(false, " Baudrate: %d", *ttyConf.Baudrate)
				s.rawTtyPrintfLn(false, " Databits: %d", *ttyConf.Databits)
				s.rawTtyPrintfLn(false, " Flow: soft")
				s.rawTtyPrintfLn(false, " Stopbits: %d", *ttyConf.Stopbits)
				if ttyConf.Parity == nil {
					s.rawTtyPrintfLn(false, " Parity: none")
				} else {
					s.rawTtyPrintfLn(false, " Parity: %s", *ttyConf.Parity)
				}
			} else {
				s.rawTtyPrintfLn(false, "Failed to retrieve remote terminal configuration: %v", err)
			}
		}
	case DetectBaudChar:
		println("")
		if s.implementation == ttyc.ImplementationWiSe {
			s.rawTtyPrintfLn(false, "Requesting baud rate detection (it may take up to 10 seconds)")
			s.client.RequestBaudrateDetection()
		} else {
			s.rawTtyPrintfLn(true, "Baud rate detection is only available for Wi-Se")
		}
	case BreakChar:
		s.client.SendBreak()
	case ClearChar:
		// Clear screen using ANSI/VT100 escape code
		print(ClearSequence)
		_ = os.Stdout.Sync()
	case CtrlTChar:
		// Put back escape char into buffer
		return []byte{EscapeChar}
	case LocalEchoChar:
		s.localEchoMode = !s.localEchoMode
	case HexModeChar:
		s.hexMode = !s.hexMode
	case TimestampsChar:
		s.showTimestamps = !s.showTimestamps
		s.nextIsTimestamp = false
	case HelpChar:
		println("")
		s.rawTtyPrintfLn(false, "Key commands:")
		cmdsHelpOrder := make([]int, len(cmdsInfo))
		i := 0
		for key := range cmdsInfo {
			cmdsHelpOrder[i] = int(key)
			i++
		}
		// Sort chars with lowercase next to uppercase
		keyFn := func(index int) int {
			char := cmdsHelpOrder[index]
			ret := char * 2
			if char >= 'a' && char <= 'z' {
				ret = (char-32)*2 - 1
			}
			return ret
		}
		comparator := func(c1 int, c2 int) bool { return keyFn(c1) < keyFn(c2) }
		sort.Slice(cmdsHelpOrder, comparator)

		for _, key := range cmdsHelpOrder {
			info := cmdsInfo[byte(key)]
			if s.implementation != ttyc.ImplementationWiSe && info.NonStandard {
				continue
			}
			s.rawTtyPrintfLn(false, " ctrl-t %c   %s", key, info.HelpText)
		}
	case StatsChar:
		s.printStats()
	case VersionChar:
		println()
		s.rawTtyPrintfLn(false, "ttyc %s", ttyc.VERSION)
	}

	return []byte{}
}

func bufferToHex(inBuf []byte) (outBuf []byte) {
	outBuf = []byte{}
	for _, value := range inBuf {
		if value == '\n' || value == '\r' {
			outBuf = append(outBuf, value)
		} else {
			byteStr := fmt.Sprintf("%02x ", value)
			outBuf = append(outBuf, []byte(byteStr)...)
		}
	}
	return
}

func (s *stdfdsHandler) injectTimestamps(inBuf []byte) (outBuf []byte) {
	outBuf = inBuf
	tstamp := []byte(fmt.Sprintf(ttyc.PlatformGray()+"[%s]"+color.Reset+" ", ttyc.Strftime.FormatString(time.Now())))

	i := 0

	for i < len(outBuf) {
		if s.nextIsTimestamp && i != len(outBuf)-1 {
			end := append([]byte{}, outBuf[i:]...)
			outBuf = append(outBuf[:i], tstamp...)
			outBuf = append(outBuf, end...)
			i += len(tstamp)
			s.nextIsTimestamp = false
		}
		if outBuf[i] == '\n' {
			s.nextIsTimestamp = true
		}
		i++
	}
	return
}

func (s *stdfdsHandler) printOutput(errChan chan<- error) {
	for {
		select {
		case <-s.client.CloseChan:
			return
		case buf := <-s.client.Output:
			if s.hexMode {
				buf = bufferToHex(buf)
			}
			if s.showTimestamps {
				buf = s.injectTimestamps(buf)
			}
			written := 0
			for written < len(buf) {
				bWritten, err := os.Stdout.Write(buf[written:])
				if err != nil {
					errChan <- err
					return
				}
				written += bWritten
			}
			_ = os.Stdout.Sync()
		}
	}
}

func (s *stdfdsHandler) Run(errChan chan<- error) {
	if err := s.HandleReconnect(); err != nil {
		errChan <- err
		return
	}

	cmdHandlingChan := make(chan []byte, 1)
	go utils.CopyReaderToChan(s.client.CloseChan, os.Stdin, cmdHandlingChan, errChan)
	go s.handleStdin(s.client.CloseChan, cmdHandlingChan, s.client.Input, errChan)
	go s.printOutput(errChan)

	winch := make(chan switzerland.WinchSignal)
	switz := switzerland.GetSwitzerland()
	defer switz.Stop(winch)
	defer close(winch)
	switz.Notify(winch)

	for {
		select {
		case <-s.client.CloseChan:
			return
		case <-winch:
			if winSize, err := (*s.console).Size(); err != nil {
				ttyc.Trace()
				errChan <- err
				return
			} else {
				height := int(winSize.Height)
				// Windows seems to include the row below the bottom scrollbar
				if runtime.GOOS == "windows" {
					height -= 1
				}
				s.client.ResizeTerminal(int(winSize.Width), height)
			}
		case title := <-s.client.WinTitle:
			s.rawTtyPrintfLn(false, "Title: %s", title)
		case baudResult := <-s.client.DetectedBaudrate:
			approx := baudResult[0]
			measured := baudResult[1]
			if approx <= 0 {
				s.rawTtyPrintfLn(true, "Baudrate detection was not successful (detection only works while input is received)")
				break
			}
			if measured > 0 {
				s.rawTtyPrintfLn(false, "Detected baudrate: likely %d bps (measured %d bps)", approx, measured)
			} else {
				s.rawTtyPrintfLn(false, "Detected baudrate: likely %d bps", approx)
			}
		}
	}
}

func (s *stdfdsHandler) HandleDisconnect() error {
	if s.console != nil {
		if err := (*s.console).Reset(); err != nil {
			ttyc.Trace()
			return err
		}
		s.console = nil
		print("\r")

		if err := shenanigans.WindowsClearConsole(); err != nil {
			ttyc.Trace()
			return err
		}
	}
	return nil
}

func (s *stdfdsHandler) HandleReconnect() error {
	current := console.Current()
	s.console = &current
	if err := current.SetRaw(); err != nil {
		ttyc.Trace()
		return err
	}
	winSize, err := current.Size()
	if err != nil {
		ttyc.Trace()
		return err
	}
	//println("RESIZE TERM")
	s.client.ResizeTerminal(int(winSize.Width), int(winSize.Height))
	//println("TERM RESIZED")
	return nil
}

func (s *stdfdsHandler) Close() error {
	if err := s.HandleDisconnect(); err != nil {
		return err
	}
	return nil
}
