package ws

import "C"
import (
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
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

	// Server messages
	MsgOutput         byte = '0'
	MsgSetWindowTitle byte = '1'
	MsgPreferences    byte = '2'
)

type Client struct {
	WsClient  *websocket.Conn
	HttpResp  *http.Response
	WinTitle  <-chan []byte
	Output    <-chan []byte
	Input     chan<- []byte
	Error     <-chan error
	CloseChan <-chan interface{}

	winTitle chan []byte
	output   chan []byte
	input    chan []byte
	pong     chan interface{}
	error    chan error

	toWs       chan []byte
	fromWs     chan []byte
	shutdown   chan interface{}
	isShutdown bool
	closed     bool
}

type TtyClientOps interface {
	Run()
	ResizeTerminal(cols int, rows int)
	Pause()
	Resume()
}

func DialAndAuth(wsUrl *url.URL, token *string) (client *Client, err error) {
	dialer := websocket.Dialer{
		Subprotocols:     []string{"tty"},
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}
	wsClient, resp, err := dialer.Dial(wsUrl.String(), nil)
	if err != nil {
		ttyc.Trace()
		return
	}

	authDTO := AuthDTO{
		AuthToken: *token,
	}
	message, _ := json.Marshal(authDTO)
	if err = wsClient.WriteMessage(websocket.BinaryMessage, message); err != nil {
		ttyc.Trace()
		return
	}

	client = &Client{
		WsClient:   wsClient,
		HttpResp:   resp,
		winTitle:   make(chan []byte),
		output:     make(chan []byte),
		input:      make(chan []byte),
		pong:       make(chan interface{}),
		error:      make(chan error),
		toWs:       make(chan []byte),
		fromWs:     make(chan []byte),
		shutdown:   make(chan interface{}),
		isShutdown: false,
		closed:     false,
	}
	client.WinTitle = client.winTitle
	client.Output = client.output
	client.Input = client.input
	client.Error = client.error
	client.CloseChan = client.shutdown
	return
}

func (c *Client) Close() error {
	c.doShutdown(nil)
	if c.closed {
		return nil
	}
	c.closed = true

	close(c.winTitle)
	close(c.output)
	close(c.input)
	close(c.error)
	close(c.toWs)
	close(c.fromWs)

	if err := c.WsClient.Close(); err != nil {
		ttyc.Trace()
		return err
	}

	return nil
}

func (c *Client) doShutdown(err error) {
	if !c.isShutdown {
		close(c.shutdown)
		c.isShutdown = true

		if err != nil {
			c.error <- err
		}
	}
}

func (c *Client) readLoop() {
	for !c.closed && !c.isShutdown {
		//println("BLOCKING readLoop")
		msgType, data, err := c.WsClient.ReadMessage()
		//println("Unblocked readLoop")
		if err != nil {
			ttyc.Trace()
			c.doShutdown(err)
			return
		}
		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
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
			if data[0] == MsgOutput {
				c.output <- data[1:]
			} //else if data[0] == MsgSetWindowTitle {
			//	c.winTitle <- data[1:]
			//}
			// Ignore MsgSetPreferences since we're not Xterm.js
		case data := <-c.toWs:
			if len(data) == 0 {
				continue
			}
			err := c.WsClient.WriteMessage(websocket.BinaryMessage, data)
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
			err := c.WsClient.WriteMessage(websocket.BinaryMessage, append([]byte{MsgInput}, data...))
			if err != nil {
				ttyc.Trace()
				c.doShutdown(err)
				return
			}
		case <-c.shutdown:
		}
		//println("SELECTED chanLoop")
	}
}

func (c *Client) watchdog(interval int) {
	pingDuration := time.Duration(interval) * time.Second
	timeoutDuration := time.Duration(interval+3) * time.Second
	nextPing := time.Now().Add(pingDuration)
	// Give some extra time for the first timeout
	nextTimeout := time.Now().Add(timeoutDuration + pingDuration)

	for !c.closed && !c.isShutdown {
		select {
		case <-time.After(nextPing.Sub(time.Now())):
			if err := c.WsClient.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				ttyc.Trace()
				c.doShutdown(err)
				return
			}
			nextPing = time.Now().Add(pingDuration)
		case <-time.After(nextTimeout.Sub(time.Now())):
			ttyc.Trace()
			c.doShutdown(fmt.Errorf("server is not responding, closing"))
			return
		case <-c.pong:
			nextTimeout = time.Now().Add(timeoutDuration)
		case <-c.shutdown:
			return
		}
	}
}

func (c *Client) Run(watchdog int) {
	go c.readLoop()
	if watchdog > 0 {
		c.WsClient.SetPongHandler(func(_ string) error {
			c.pong <- true
			return nil
		})
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
