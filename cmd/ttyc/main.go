package main

import (
	"bufio"
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/Depau/ttyc/cmd/ttyc/handlers"
	"github.com/Depau/ttyc/ws"
	"github.com/mattn/go-isatty"
	"github.com/mkideal/cli"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"
)

func (argv *Config) AutoHelp() bool {
	return argv.Help
}

func (argv *Config) Validate(ctx *cli.Context) error {
	// Windows bullshit
	if argv.GetWaitDebugger() {
		fmt.Print("Attach the debugger now, then press Enter to continue...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}

	if !(argv.User != "" && argv.Pass != "") && !(argv.User == "" && argv.Pass == "") {
		return fmt.Errorf("user and password must be both provided or not provided at all")
	}
	parsedUrl, err := url.Parse(argv.Url)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return fmt.Errorf("invalid URL, must be http or https")
	}
	if argv.GetTty() == "" && (!isatty.IsTerminal(os.Stdout.Fd()) || !isatty.IsTerminal(os.Stdin.Fd())) {
		return fmt.Errorf("cannot launch in terminal mode when standard file descriptors aren't terminals")
	}
	if !(argv.Backoff == "none" || argv.Backoff == "linear" || argv.Backoff == "exponential") {
		return fmt.Errorf("invalid backoff: %d", argv.Baud)
	}
	if argv.Baud != -1 && argv.Baud <= 0 {
		return fmt.Errorf("invalid baud rate: %d", argv.Baud)
	}
	if !(argv.Parity == "even" || argv.Parity == "odd" || argv.Parity == "none" || argv.Parity == "") {
		return fmt.Errorf("invalid parity: %s", argv.Parity)
	}
	if !(argv.Databits == -1 || (argv.Databits >= 5 && argv.Databits <= 8)) {
		return fmt.Errorf("invalid data bits: %d", argv.Databits)
	}
	if !(argv.Stopbits == -1 || argv.Stopbits == 1 || argv.Stopbits == 2) {
		return fmt.Errorf("invalid stop bits: %d", argv.Stopbits)
	}
	return nil
}

func stty(config *Config, sttyUrl *url.URL, credentials *url.Userinfo) error {
	dto := ttyc.SttyDTO{
		Baudrate: nil,
		Databits: nil,
		Stopbits: nil,
		Parity:   nil,
	}
	baud := uint(config.Baud)
	bits := uint8(config.Databits)
	stop := uint8(config.Stopbits)
	paramsToUpdate := 0
	if config.Baud > 0 {
		dto.Baudrate = &baud
		paramsToUpdate++
	}
	if config.Databits > 0 {
		dto.Databits = &bits
		paramsToUpdate++
	}
	if config.Stopbits > 0 {
		dto.Stopbits = &stop
		paramsToUpdate++
	}
	if config.Parity != "" {
		dto.Parity = &config.Parity
		paramsToUpdate++
	}
	if paramsToUpdate == 0 {
		return nil
	}

	_, err := ttyc.Stty(sttyUrl, credentials, &dto)
	if err != nil {
		ttyc.Trace()
		return err
	}
	return nil
}

func doHandshakeAndSetTerminal(baseUrl *url.URL, credentials *url.Userinfo, config *Config) (token string, implementation ttyc.Implementation, server string, err error) {
	tokenUrl := ttyc.GetUrlFor(ttyc.UrlForToken, baseUrl)
	sttyHttpUrl := ttyc.GetUrlFor(ttyc.UrlForStty, baseUrl)

	token, implementation, server, err = ttyc.Handshake(tokenUrl, credentials)
	if err != nil {
		err = fmt.Errorf("handshake failed (unable to connect or wrong user/pass): %v\n", err)
		return
	}

	if implementation == ttyc.ImplementationWiSe {
		if err := stty(config, sttyHttpUrl, credentials); err != nil {
			err = fmt.Errorf("unable to set remote UART parameters: %v\n", err)
		}
	}
	return
}

func nextBackoff(curBsckoff time.Duration, config *Config) time.Duration {
	if config.Backoff == "none" {
		return time.Duration(config.Reconnect) * time.Second
	} else if config.Backoff == "linear" {
		return curBsckoff + time.Duration(config.BackoffValue)*time.Second
	} else if config.Backoff == "exponential" {
		return curBsckoff * time.Duration(config.BackoffValue)
	}
	panic("fix command line argument validator")
}

func main() {
	config := Config{}

	ret := cli.Run(&config, func(ctx *cli.Context) error {
		return nil
	}, "ttyd protocol client")

	if ret != 0 || config.Help {
		os.Exit(ret)
	}
	if config.Version {
		fmt.Printf("ttyc %s\n", ttyc.VERSION)
		println(ttyc.COPYRIGHT)

		os.Exit(0)
	}

	//fmt.Printf("%+v\n", config);

	baseUrl, _ := url.Parse(config.Url)

	var credentials *url.Userinfo = nil
	if config.User != "" {
		credentials = url.UserPassword(config.User, config.Pass)
	} else if config.User == "" && baseUrl.User != nil {
		credentials = baseUrl.User
	}
	baseUrl.User = nil

	// Reduce HTTP timeout so that the client doesn't stall on reconnection when the server is down for a few seconds
	http.DefaultClient.Timeout = time.Duration(math.Max(math.Min(float64(config.Reconnect), 5.0), 2.0)) * time.Second

	token, implementation, server, err := doHandshakeAndSetTerminal(baseUrl, credentials, &config)
	if err != nil {
		ttyc.TtycAngryPrintf("%v\n", err)
		os.Exit(1)
	}

	client, err := ws.DialAndAuth(baseUrl, &token, config.Watchdog)
	if err != nil {
		ttyc.TtycAngryPrintf("unable to connect or authenticate to server: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	go client.Run(config.Watchdog)

	handlerErrChan := make(chan error, 1)
	defer close(handlerErrChan)

	var handler handlers.TtyHandler
	if config.GetTty() == "" {
		handler, err = handlers.NewStdFdsHandler(client, implementation, credentials, server)
		if err != nil {
			ttyc.TtycAngryPrintf("Unable to launch console handler: %v\n", err)
			os.Exit(1)
		}
		ttyc.TtycPrintf("ttyc %s\n", ttyc.VERSION)
		ttyc.TtycPrintf("Press ctrl-t q to quit, ctrl-t ? for help\n")
		ttyc.TtycPrintf("Connected\n")
	} else {
		handler, err = handlers.NewPtyHandler(client, config.GetTty())
		if err != nil {
			ttyc.TtycAngryPrintf("Unable to launch PTY handler: %v\n", err)
			os.Exit(1)
		}
	}
	defer handler.Close()
	go handler.Run(handlerErrChan)

	var fatalError error

	reconnect := time.Duration(config.Reconnect) * time.Second
	for {
		select {
		case fatalError = <-handlerErrChan:
			if err := handler.HandleDisconnect(); err != nil {
				ttyc.TtycAngryPrintf("Error while handling disconnection: %v\n", err)
			}
			ttyc.TtycAngryPrintf("%v\n", fatalError)
			return
		case fatalError = <-client.Error:
			// Restore terminal, if any
			if err := handler.HandleDisconnect(); err != nil {
				ttyc.TtycAngryPrintf("Error while handling disconnection: %v\n", err)
				return
			}

			println()
			ttyc.TtycAngryPrintf("Server disconnected: %v\n", fatalError)
			if err := client.SoftClose(); err != nil {
				ttyc.TtycAngryPrintf("Error while cleaning up the WebSocket: %v\n", err)
			}
			if config.Reconnect < 0 {
				return
			}

			for {
				if reconnect.Seconds() <= 0 {
					ttyc.TtycPrintf("Reconnecting\n")
				} else {
					ttyc.TtycPrintf("Reconnecting in %d seconds\n", int(reconnect.Seconds()))
					<-time.After(reconnect)
				}
				reconnect = nextBackoff(reconnect, &config)

				token, _, _, err := doHandshakeAndSetTerminal(baseUrl, credentials, &config)
				if err != nil {
					ttyc.TtycAngryPrintf("Unable to perform authentication: %v\n", err)
					continue
				}
				if err := client.Redial(&token); err != nil {
					ttyc.TtycAngryPrintf("Unable to connect or authenticate to server: %v\n", err)
					continue
				}
				break
			}
			ttyc.TtycPrintf("Reconnected\n")
			go client.Run(config.Watchdog)

			// Put back terminal into raw mode
			if err := handler.HandleReconnect(); err != nil {
				ttyc.TtycAngryPrintf("Error while handling reconnection: %v\n", err)
				return
			}
		}
	}

}
