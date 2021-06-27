// +build !windows
// +build !darwin

package main

type Config struct {
	Help         bool   `cli:"!h,help" usage:"Show help"`
	Url          string `cli:"U,url" usage:"Server URL"`
	Watchdog     int    `cli:"w,watchdog" usage:"WebSocket ping interval in seconds, 0 to disable, default 2." dft:"2"`
	Reconnect    int    `cli:"r,reconnect" usage:"Reconnection interval in seconds, -1 to disable, default 3." dft:"2"`
	Backoff      string `cli:"backoff" usage:"Backoff type, none, linear, exponential, defaults to linear" dft:"none"`
	BackoffValue uint   `cli:"backoff-value" usage:"For linear backoff, increase reconnect interval by this amount of seconds after each iteration. For exponential backoff, multiply reconnect interval by this amount. Default 2" dft:"2"`
	User         string `cli:"u,user" usage:"Username for authentication" dft:""`
	Pass         string `cli:"k,pass" usage:"Password for authentication" dft:""`
	Tty          string `cli:"T,tty" usage:"Do not launch terminal, create terminal device at given location (i.e. /tmp/ttyd)" dft:""`
	Baud         int    `cli:"b,baudrate" usage:"(Wi-Se only) Set remote baud rate [bps]" dft:"-1"`
	Parity       string `cli:"p,parity" usage:"(Wi-Se only) Set remote parity [odd|even|none]" dft:""`
	Databits     int    `cli:"d,databits" usage:"(Wi-Se only) Set remote data bits [5|6|7|8]" dft:"-1"`
	Stopbits     int    `cli:"s,stopbits" usage:"(Wi-Se only) Set remote stop bits [1|2]" dft:"-1"`
	Version      bool   `cli:"!v,version" usage:"Show version"`
}

func (config *Config) GetTty() string {
	return (*config).Tty
}

func (config *Config) GetWaitDebugger() bool {
	return false
}
