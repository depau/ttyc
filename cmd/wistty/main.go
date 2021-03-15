package main

import (
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc"
	"github.com/mkideal/cli"
	"log"
	"net/url"
	"os"
)

type Config struct {
	Help     bool   `cli:"!h,help" usage:"Show help"`
	Url      string `cli:"U,url" usage:"Server URL"`
	User     string `cli:"u,user" usage:"Username for authentication" dft:""`
	Pass     string `cli:"k,pass" usage:"Password for authentication" dft:""`
	Json     bool   `cli:"j,json" usage:"Return machine-readable JSON output"`
	Baud     int    `cli:"b,baudrate" usage:"Set remote baud rate [bps]" dft:"-1"`
	Parity   string `cli:"p,parity" usage:"Set remote parity [odd|even|none]" dft:""`
	Databits int    `cli:"d,databits" usage:"Set remote data bits [5|6|7|8]" dft:"-1"`
	Stopbits int    `cli:"s,stopbits" usage:"Set remote stop bits [1|2]" dft:"-1"`
	Version  bool   `cli:"!v,version" usage:"Show version"`
}

func (argv *Config) AutoHelp() bool {
	return argv.Help
}

func (argv *Config) Validate(ctx *cli.Context) error {
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

func stty(config *Config, sttyUrl *url.URL, credentials *url.Userinfo) (stty ttyc.SttyDTO, err error) {
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
		return ttyc.GetStty(sttyUrl, credentials)
	}

	stty, err = ttyc.Stty(sttyUrl, credentials, &dto)
	return
}

func main() {
	config := Config{}

	ret := cli.Run(&config, func(ctx *cli.Context) error {
		return nil
	}, "wistty - Manage Wi-Se remote terminal parameters")

	if ret != 0 || config.Help {
		os.Exit(ret)
	}
	if config.Version {
		fmt.Printf("wistty %s\n", ttyc.VERSION)
		println(ttyc.COPYRIGHT)

		os.Exit(0)
	}

	baseUrl, _ := url.Parse(config.Url)

	var credentials *url.Userinfo = nil
	if config.User != "" {
		credentials = url.UserPassword(config.User, config.Pass)
	} else if config.User == "" && baseUrl.User != nil {
		credentials = baseUrl.User
	}
	baseUrl.User = nil

	sttyHttpUrl := ttyc.GetUrlFor(ttyc.UrlForStty, baseUrl)
	stty, err := stty(&config, sttyHttpUrl, credentials)
	if err != nil {
		log.Fatalln("could not set terminal:", err)
	}

	if config.Json {
		strStty, err := json.Marshal(stty)
		if err != nil {
			log.Fatalln("could not parse stty output:", err)
		}
		println(string(strStty))
	} else {
		var parityChar rune
		if stty.Parity == nil {
			parityChar = 'n'
		} else if *stty.Parity == "even" {
			parityChar = 'e'
		} else if *stty.Parity == "odd" {
			parityChar = 'o'
		}
		fmt.Printf("%d %d%c%d\n", *stty.Baudrate, *stty.Databits, parityChar, *stty.Stopbits)
	}
}
