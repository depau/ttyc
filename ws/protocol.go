package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc"
	"io"
	"net/http"
	"net/url"
	"nhooyr.io/websocket"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AuthDTO struct {
	AuthToken string
}

type ResizeTerminalDTO struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

const (
	// Client messages
	MsgInput          byte = '0'
	MsgResizeTerminal byte = '1'
	MsgPause          byte = '2'
	MsgResume         byte = '3'
	MsgJsonData       byte = '{'
	MsgBreak          byte = 'b'

	// Both
	MsgDetectBaudrate byte = 'B'

	// Server messages
	MsgOutput         byte = '0'
	MsgSetWindowTitle byte = '1'
	MsgPreferences    byte = '2'
	MsgServerPause    byte = 'S'
	MsgServerResume   byte = 'Q'
)

type Client struct {
	BaseUrl          *url.URL
	WsClient         *websocket.Conn
	HttpResp         *http.Response
	WinTitle         <-chan []byte
	Output           <-chan []byte
	Input            chan<- []byte
	DetectedBaudrate <-chan [2]int64
	Error            <-chan error
	CloseChan        <-chan interface{}

	mainCtx            context.Context
	mainCtxCancel      context.CancelFunc
	wsHttpClient       http.Client
	winTitle           chan []byte
	detectedBaudrate   chan [2]int64
	output             chan []byte
	input              chan []byte
	flowControl        sync.Mutex
	flowControlEngaged bool
	error              chan error

	watchdogInterval int
	toWs             chan []byte
	fromWs           chan []byte
	shutdown         chan interface{}
	closeChan        chan interface{}
	isShutdown       bool
	closed           bool
}

type TtyClientOps interface {
	io.Closer

	Redial(token *string, watchdog int) error
	Run()
	ResizeTerminal(cols int, rows int)
	RequestBaudrateDetect()
	Pause()
	Resume()
	SendBreak()
	SoftClose() error
}

func DialAndAuth(baseUrl *url.URL, token *string, watchdog int) (client *Client, err error) {
	client = &Client{
		BaseUrl:            baseUrl,
		winTitle:           make(chan []byte),
		output:             make(chan []byte),
		input:              make(chan []byte),
		detectedBaudrate:   make(chan [2]int64),
		flowControlEngaged: false,
		wsHttpClient:       http.Client{},
		error:              make(chan error),
		toWs:               make(chan []byte),
		fromWs:             make(chan []byte),
		closeChan:          make(chan interface{}),
		isShutdown:         true,
		closed:             false,
		watchdogInterval:   watchdog,
	}
	client.mainCtx, client.mainCtxCancel = context.WithCancel(context.Background())
	if err := client.Redial(token); err != nil {
		return nil, err
	}
	client.CloseChan = client.closeChan
	client.WinTitle = client.winTitle
	client.DetectedBaudrate = client.detectedBaudrate
	client.Output = client.output
	client.Input = client.input
	client.Error = client.error
	return
}

func (c *Client) getWriteContext() (context.Context, context.CancelFunc) {
	if c.watchdogInterval > 0 {
		return context.WithTimeout(c.mainCtx, time.Duration(c.watchdogInterval)*time.Second)
	}
	return c.getReadContext()
}

func (c *Client) getReadContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(c.mainCtx)
}

func (c *Client) Redial(token *string) error {
	if c.closed {
		return fmt.Errorf("not allowed to redial on closed client")
	}

	dialOpts := websocket.DialOptions{
		HTTPClient:   &c.wsHttpClient,
		Subprotocols: []string{"tty"},
	}
	wsUrl := ttyc.GetUrlFor(ttyc.UrlForWebSocket, c.BaseUrl)

	ctx, cancel := c.getWriteContext()
	wsClient, resp, err := websocket.Dial(ctx, wsUrl.String(), &dialOpts)
	cancel()
	if err != nil {
		ttyc.Trace()
		return err
	}
	authDTO := AuthDTO{
		AuthToken: *token,
	}
	message, _ := json.Marshal(authDTO)

	ctx, cancel = c.getWriteContext()
	err = wsClient.Write(ctx, websocket.MessageBinary, message)
	cancel()
	if err != nil {
		ttyc.Trace()
		return err
	}

	c.WsClient = wsClient
	c.HttpResp = resp
	c.shutdown = make(chan interface{})
	c.isShutdown = false
	return nil
}

func (c *Client) SoftClose() error {
	if !c.isShutdown {
		return fmt.Errorf("can only soft-close in order to redial if the client is already shut down")
	}
	err := c.WsClient.Close(websocket.StatusGoingAway, "")
	if err != nil {
		ttyc.Trace()
		return err
	}
	return nil
}

func (c *Client) Close() error {
	c.doShutdown(nil)
	if c.closed {
		return nil
	}
	c.closed = true

	close(c.closeChan)
	close(c.winTitle)
	close(c.output)
	close(c.input)
	close(c.error)
	close(c.toWs)
	close(c.fromWs)

	if err := c.SoftClose(); err != nil {
		ttyc.Trace()
		return err
	}

	c.mainCtxCancel()

	return nil
}

func (c *Client) doShutdown(err error) {
	if !c.isShutdown {
		close(c.shutdown)
		c.isShutdown = true

		if c.flowControlEngaged {
			c.flowControlEngaged = false
			c.flowControl.Unlock()
		}

		if err != nil {
			c.error <- err
		}
	}
}

func (c *Client) readLoop() {
	for !c.closed && !c.isShutdown {
		//println("BLOCKING readLoop")
		ctx, cancel := c.getReadContext()
		msgType, data, err := c.WsClient.Read(ctx)
		cancel()
		//println("Unblocked readLoop")
		if err != nil {
			ttyc.Trace()
			c.doShutdown(err)
			return
		}
		if msgType != websocket.MessageBinary && msgType != websocket.MessageText {
			continue
		}
		c.fromWs <- data
	}
}

func (c *Client) chanLoop() {
	for !c.closed && !c.isShutdown {
		//println("SELECT chanLoop")
		select {
		case data := <-c.fromWs:
			if len(data) <= 0 {
				continue
			}
			switch data[0] {
			case MsgOutput:
				c.output <- data[1:]
			case MsgServerPause:
				if !c.flowControlEngaged {
					c.flowControlEngaged = true
					c.flowControl.Lock()
				}
			case MsgServerResume:
				if c.flowControlEngaged {
					c.flowControlEngaged = false
					c.flowControl.Unlock()
				}
			case MsgSetWindowTitle:
			EmptyWinTitleChanLoop:
				// Empty channel so we don't block if the user is not reading
				for {
					select {
					case <-c.winTitle:
					default:
						break EmptyWinTitleChanLoop
					}
				}
				c.winTitle <- data[1:]
			case MsgDetectBaudrate:
				// Empty channel so we don't block if the user is not reading
			EmptyBaudChanLoop:
				for {
					select {
					case <-c.detectedBaudrate:
					default:
						break EmptyBaudChanLoop
					}
				}
				dataStr := string(data[1:])
				var result [2]int64
				if strings.Contains(dataStr, ",") {
					split := strings.SplitN(dataStr, ",", 2)
					if len(split) != 2 {
						ttyc.TtycAngryPrintf("Received invalid detected baudrate: %s\n", dataStr)
						break
					}
					for index, item := range split {
						i, err := strconv.ParseInt(item, 10, 64)
						if err != nil {
							ttyc.TtycAngryPrintf("Unable to parse detected baudrate: %v\n", err)
							break
						}
						result[index] = i
					}
				} else {
					i, err := strconv.ParseInt(string(data[1:]), 10, 64)
					if err != nil {
						ttyc.TtycAngryPrintf("Unable to parse detected baudrate: %v\n", err)
						break
					}
					result[0] = i
					result[1] = 0
				}
				c.detectedBaudrate <- result
			}
			if data[0] == MsgOutput {
			}
			// Ignore WinTitle since it caused an issue and we're not using it anyway anywhere
			// Ignore MsgSetPreferences since we're not Xterm.js

		case data := <-c.toWs:
			if len(data) == 0 {
				continue
			}
			c.flowControl.Lock()
			ctx, cancel := c.getWriteContext()
			err := c.WsClient.Write(ctx, websocket.MessageBinary, data)
			cancel()
			c.flowControl.Unlock()
			if err != nil {
				ttyc.Trace()
				c.doShutdown(err)
				return
			}
		case data := <-c.input:
			if len(data) == 0 {
				continue
			}
			// I could avoid duplicating the code but I'd rather avoid the additional copy, since writing to the
			// WebSocket is this goroutine's job anyway.
			c.flowControl.Lock()
			ctx, cancel := c.getWriteContext()
			err := c.WsClient.Write(ctx, websocket.MessageBinary, append([]byte{MsgInput}, data...))
			cancel()
			c.flowControl.Unlock()
			if err != nil {
				ttyc.Trace()
				c.doShutdown(err)
				return
			}
		case <-c.closeChan:
		case <-c.shutdown:
		}
		//println("SELECTED chanLoop")
	}
}

func (c *Client) watchdog(interval int) {
	pingDuration := time.Duration(interval) * time.Second
	nextPing := time.Now().Add(pingDuration)

	for !c.closed && !c.isShutdown {
		select {
		case <-time.After(nextPing.Sub(time.Now())):
			ctx, cancel := c.getWriteContext()
			err := c.WsClient.Ping(ctx)
			cancel()
			if err != nil {
				ttyc.Trace()
				c.doShutdown(err)
				return
			}
			nextPing = time.Now().Add(pingDuration)
		case <-c.closeChan:
		case <-c.shutdown:
			return
		}
	}
}

func (c *Client) Run(watchdog int) {
	go c.readLoop()
	if watchdog > 0 {
		go c.watchdog(watchdog)
	}
	c.chanLoop()
}

func (c *Client) ResizeTerminal(cols int, rows int) {
	dto := ResizeTerminalDTO{
		Columns: cols,
		Rows:    rows,
	}
	msg, _ := json.Marshal(&dto)
	c.toWs <- append([]byte{MsgResizeTerminal}, msg...)
}

func (c *Client) Pause() {
	c.toWs <- []byte{MsgPause}
}

func (c *Client) Resume() {
	c.toWs <- []byte{MsgResume}
}

func (c *Client) RequestBaudrateDetection() {
	c.toWs <- []byte{MsgDetectBaudrate}
}

func (c *Client) SendBreak() {
	c.toWs <- []byte{MsgBreak}
}
