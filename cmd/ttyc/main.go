package main

import (
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/Depau/ttyc/cmd/ttyc/handlers"
	"github.com/Depau/ttyc/ws"
	"github.com/mattn/go-isatty"
	"github.com/mkideal/cli"
	"net/url"
	"os"
	"time"
)

type Config struct {
	Help         bool   `cli:"!h,help" usage:"Show help"`
	Host         string `cli:"*H,host" usage:"Server hostname"`
	Port         int    `cli:"*P,port" usage:"Server port"`
	Tls          bool   `cli:"t,tls" usage:"Use TLS" dft:"false"`
	Watchdog     int    `cli:"w,watchdog" usage:"WebSocket ping interval in seconds, 0 to disable, default 10." dft:"10"`
	Reconnect    int    `cli:"r,reconnect" usage:"Reconnection interval in seconds, -1 to disable, default 3." dft:"3"`
	Backoff      string `cli:"backoff" usage:"Backoff type, none, linear, exponential, defaults to linear" dft:"linear"`
	BackoffValue uint   `cli:"backoff-value" usage:"For linear backoff, increase reconnect interval by this amount of seconds after each iteration. For exponential backoff, multiply reconnect interval by this amount. Default 2" dft:"2"`
	User         string `cli:"u,user" usage:"Username for authentication" dft:""`
	Pass         string `cli:"k,pass" usage:"Password for authentication" dft:""`
	Tty          string `cli:"T,tty" usage:"Do not launch terminal, create terminal device at given location (i.e. /tmp/ttyd)" dft:""`
	Baud         int    `cli:"b,baudrate" usage:"(Wi-Se only) Set remote baud rate [bps]" dft:"-1"`
	Parity       string `cli:"p,parity" usage:"(Wi-Se only) Set remote parity [odd|even|none]" dft:""`
	Databits     int    `cli:"d,databits" usage:"(Wi-Se only) Set remote data bits [5|6|7|8]" dft:"-1"`
	Stopbits     int    `cli:"s,stopbits" usage:"(Wi-Se only) Set remote stop bits [1|2]" dft:"-1"`
}

func (argv *Config) AutoHelp() bool {
	return argv.Help
}

func (argv *Config) Validate(ctx *cli.Context) error {
	if !(argv.User != "" && argv.Pass != "") && !(argv.User == "" && argv.Pass == "") {
		return fmt.Errorf("user and password must be both provided or not provided at all")
	}
	if argv.Port <= 0 || argv.Port > 0xFFFF {
		return fmt.Errorf("invalid port: %d", argv.Port)
	}
	if argv.Tty == "" && (!isatty.IsTerminal(os.Stdout.Fd()) || !isatty.IsTerminal(os.Stdin.Fd())) {
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

func stty(config *Config, sttyUrl *url.URL) error {
	dto := ttyc.SttyDTO{
		Baudrate: nil,
		Databits: nil,
		Stopbits: nil,
		Parity:   nil,
	}
	baud := uint(config.Baud)
	bits := uint8(config.Databits)
	stop := uint8(config.Stopbits)
	if config.Baud > 0 {
		dto.Baudrate = &baud
	}
	if config.Databits > 0 {
		dto.Databits = &bits
	}
	if config.Stopbits > 0 {
		dto.Stopbits = &stop
	}
	if config.Parity != "" {
		dto.Parity = &config.Parity
	}

	err := ttyc.Stty(sttyUrl, &dto)
	if err != nil {
		ttyc.Trace()
		return err
	}
	return nil
}

func doHandshakeAndSetTerminal(tokenUrl *url.URL, sttyHttpUrl *url.URL, config *Config) (token string, ttyConf *ttyc.SttyDTO, server string, err error) {
	token, implementation, server, err := ttyc.Handshake(tokenUrl)
	if err != nil {
		err = fmt.Errorf("handshake failed (unable to connect or wrong user/pass): %v\n", err)
		return "", nil, "", err
	}

	ttyConf = nil
	if implementation == ttyc.ImplementationWiSe {
		if err := stty(config, sttyHttpUrl); err != nil {
			err = fmt.Errorf("unable to set remote UART parameters: %v\n", err)
		}
		if cfg, err := ttyc.GetStty(sttyHttpUrl); err != nil {
			err = fmt.Errorf("unable to retrieve remote UART parameters: %v\n", err)
		} else {
			ttyConf = &cfg
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

	//fmt.Printf("%+v\n", config);

	var httpScheme string
	var wsScheme string

	if config.Tls {
		httpScheme = "https"
		wsScheme = "wss"
	} else {
		httpScheme = "http"
		wsScheme = "ws"
	}

	tokenHttpUrl := ttyc.GetBaseUrl(&httpScheme, &config.Host, config.Port, &config.User, &config.Pass)
	tokenHttpUrl.Path = "/token"
	sttyHttpUrl := ttyc.GetBaseUrl(&httpScheme, &config.Host, config.Port, &config.User, &config.Pass)
	sttyHttpUrl.Path = "/stty"
	// Auth is performed using token, and the WebSocket library doesn't support auth data in the URL anyway
	wsUrl := ttyc.GetBaseUrl(&wsScheme, &config.Host, config.Port, nil, nil)
	wsUrl.Path = "/ws"

	token, ttyConf, server, err := doHandshakeAndSetTerminal(&tokenHttpUrl, &sttyHttpUrl, &config)
	if err != nil {
		ttyc.TtycAngryPrintf("%v\n", err)
		os.Exit(1)
	}

	client, err := ws.DialAndAuth(&wsUrl, &token)
	if err != nil {
		ttyc.TtycAngryPrintf("unable to connect or authenticate to server: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()
	go client.Run(config.Watchdog)

	handlerErrChan := make(chan error, 1)
	defer close(handlerErrChan)

	var handler handlers.TtyHandler
	if config.Tty == "" {
		handler, err = handlers.NewStdFdsHandler(client, ttyConf, server)
		if err != nil {
			ttyc.TtycAngryPrintf("unable to launch console handler: %v\n", err)
			os.Exit(1)
		}
		ttyc.TtycPrintf("Press ctrl-t q to quit, ctrl-t ? for help\n")
		ttyc.TtycPrintf("Connected\n")
		println()
	} else {
		handler, err = handlers.NewPtyHandler(client, config.Tty)
		if err != nil {
			ttyc.TtycAngryPrintf("unable to launch PTY handler: %v\n", err)
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
			ttyc.TtycAngryPrintf("%v\n", fatalError)
			return
		case fatalError = <-client.Error:
			println()
			ttyc.TtycAngryPrintf("server disconnected: %v\n", fatalError)
			if err := client.SoftClose(); err != nil {
				ttyc.TtycAngryPrintf("error while cleaning up the WebSocket: %v\n", err)
			}
			if config.Reconnect < 0 {
				return
			}
			if reconnect.Seconds() <= 0 {
				ttyc.TtycPrintf("reconnecting\n")
			} else {
				ttyc.TtycPrintf("reconnecting in %d seconds\n", int(reconnect.Seconds()))
				<-time.After(reconnect)
			}
			reconnect = nextBackoff(reconnect, &config)

			token, _, _, err := doHandshakeAndSetTerminal(&tokenHttpUrl, &sttyHttpUrl, &config)
			if err != nil {
				ttyc.TtycAngryPrintf("unable to perform authentication: %v\n", err)
				continue
			}
			if err := client.Redial(&wsUrl, &token); err != nil {
				ttyc.TtycAngryPrintf("unable to connect or authenticate to server: %v\n", err)
				continue
			}
			ttyc.TtycPrintf("connected")
			go client.Run(config.Watchdog)
		}
	}

}
