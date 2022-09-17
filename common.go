package ttyc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc/utils"
	"github.com/TwinProduction/go-color"
	strftimeMod "github.com/lestrrat-go/strftime"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const VERSION = "v0.4"
const COPYRIGHT = "Copyright (c) 2022 Davide Depau\n\n" +
	"License: GNU GPL version 3.0 or later <https://www.gnu.org/licenses/gpl-3.0.html>.\n" +
	"This is free software; see the source for copying conditions.  There is NO\n" +
	"warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE."

var Strftime, _ = strftimeMod.New("%H:%M:%S")

type TokenDTO struct {
	Token string `json:"token"`
}

type Implementation uint8

const (
	ImplementationTtyd = iota
	ImplementationWiSe
)

type SttyDTO struct {
	Baudrate *uint   `json:"baudrate"`
	Databits *uint8  `json:"databits"`
	Stopbits *uint8  `json:"stopbits"`
	Parity   *string `json:"parity"`
}

type sttyInDTO struct {
	Baudrate uint  `json:"baudrate"`
	Databits uint8 `json:"bits"`
	Stopbits uint8 `json:"stop"`
	Parity   *int  `json:"parity"`
}

const (
	UrlForToken = iota
	UrlForWebSocket
	UrlForStty
	UrlForStats
	UrlForWhoami
)

func GetUrlFor(urlFor int, baseURL *url.URL) (outUrl *url.URL) {
	outUrl, _ = url.Parse(baseURL.String())

	switch urlFor {
	case UrlForToken:
		outUrl.Path = path.Join(baseURL.Path, "token")
	case UrlForStty:
		outUrl.Path = path.Join(baseURL.Path, "stty")
	case UrlForStats:
		outUrl.Path = path.Join(baseURL.Path, "stats")
	case UrlForWhoami:
		outUrl.Path = path.Join(baseURL.Path, "whoami")
	case UrlForWebSocket:
		if baseURL.Scheme == "https" {
			outUrl.Scheme = "wss"
		} else {
			outUrl.Scheme = "ws"
		}
		outUrl.Path = path.Join(baseURL.Path, "ws")
	default:
		panic("invalid urlfor\n")
	}

	return
}

func Handshake(url *url.URL, credentials *url.Userinfo) (token string, impl Implementation, server string, err error) {
	var resp *http.Response
	var body []byte

	if resp, err = http.Get(url.String()); err != nil {
		Trace()
		return
	}
	resp, err = utils.EnsureAuth(resp, credentials, nil)
	if err != nil {
		Trace()
		return
	}
	defer resp.Body.Close()

	impl = ImplementationTtyd
	server = ""
	if srv := resp.Header.Get("Server"); srv != "" {
		server = srv
		if strings.Contains(strings.ToLower(srv), "wi-se") {
			impl = ImplementationWiSe
		}
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		Trace()
		return
	}
	dto := TokenDTO{}
	if err = json.Unmarshal(body, &dto); err != nil {
		Trace()
		return
	}
	token = dto.Token
	return
}

func parseStty(body []byte) (stty SttyDTO, err error) {
	sttyIn := sttyInDTO{}
	err = json.Unmarshal(body, &sttyIn)
	if err != nil {
		Trace()
		return
	}

	stty = SttyDTO{
		Baudrate: &sttyIn.Baudrate,
		Databits: &sttyIn.Databits,
		Stopbits: &sttyIn.Stopbits,
	}
	if sttyIn.Parity == nil {
		stty.Parity = nil
	} else {
		var parity string
		if *sttyIn.Parity == 0 {
			parity = "even"
		} else {
			parity = "odd"
		}
		stty.Parity = &parity
	}
	return
}

func GetStty(url *url.URL, credentials *url.Userinfo) (stty SttyDTO, err error) {
	httpClient := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := httpClient.Get(url.String())
	if err != nil {
		Trace()
		return
	}
	resp, err = utils.EnsureAuth(resp, credentials, nil)
	if err != nil {
		Trace()
		return
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Trace()
		return
	}
	stty, err = parseStty(buf)
	return
}

func Stty(url *url.URL, credentials *url.Userinfo, dto *SttyDTO) (stty SttyDTO, err error) {
	// Generate json manually since golang can't generate it properly
	var jsonItems []string
	if dto.Baudrate != nil {
		jsonItems = append(jsonItems, fmt.Sprintf("\"baudrate\": %d", *dto.Baudrate))
	}
	if dto.Stopbits != nil {
		jsonItems = append(jsonItems, fmt.Sprintf("\"stop\": %d", *dto.Stopbits))
	}
	if dto.Databits != nil {
		jsonItems = append(jsonItems, fmt.Sprintf("\"bits\": %d", *dto.Databits))
	}
	if dto.Parity != nil {
		if *dto.Parity == "none" {
			jsonItems = append(jsonItems, "\"parity\": null")
		} else if *dto.Parity == "even" {
			jsonItems = append(jsonItems, "\"parity\": 0")
		} else if *dto.Parity == "odd" {
			jsonItems = append(jsonItems, "\"parity\": 1")
		}
	}
	var sb strings.Builder
	sb.WriteString("{")
	sb.WriteString(strings.Join(jsonItems, ","))
	sb.WriteString("}")

	resp, err := http.Post(url.String(), "application/json", bytes.NewBuffer([]byte(sb.String())))
	if err != nil {
		Trace()
		return
	}
	resp, err = utils.EnsureAuth(resp, credentials, bytes.NewBuffer([]byte(sb.String())))
	if err != nil {
		Trace()
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("HTTP status %d", resp.StatusCode)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Trace()
		return
	}
	stty, err = parseStty(buf)
	return
}

func PlatformGray() string {
	if runtime.GOOS == "windows" {
		return color.Gray
	}
	return "\u001B[1;30m"
}

func PlatformYellow() string {
	if runtime.GOOS == "windows" {
		return color.Yellow
	}
	return "\u001B[31m"
}

func TtycErrFprintf(w io.Writer, format string, a ...interface{}) {
	// Ignore fprintf errors here since I wasn't planning to care anywhere else regardless
	_, _ = fmt.Fprintf(w, color.Red+"[ttyc %s] ", Strftime.FormatString(time.Now()))
	_, _ = fmt.Fprintf(w, format, a...)
	_, _ = fmt.Fprint(w, color.Reset)
}

func TtycFprintf(w io.Writer, format string, a ...interface{}) {
	// Ignore fprintf errors here since I wasn't planning to care anywhere else regardless
	_, _ = fmt.Fprintf(w, PlatformYellow()+"[ttyc %s] ", Strftime.FormatString(time.Now()))
	_, _ = fmt.Fprintf(w, format, a...)
	_, _ = fmt.Fprint(w, color.Reset)
}

func TtycErrPrintf(format string, args ...interface{}) {
	TtycErrFprintf(os.Stderr, format, args...)
}

// Cause why not
func TtycAngryPrintf(format string, args ...interface{}) {
	TtycErrPrintf(format, args...)
}

func TtycPrintf(format string, args ...interface{}) {
	TtycFprintf(os.Stdout, format, args...)
}
