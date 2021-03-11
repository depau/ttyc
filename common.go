package ttyc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Depau/ttyc/utils"
	strftimeMod "github.com/lestrrat-go/strftime"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var strftime, _ = strftimeMod.New("%H:%M:%S")

type TokenDTO struct {
	Token string `json:"token"`
}

type Implementation uint8

const (
	ImplementationTtyd = iota
	ImplementationWiSe
)

type SttyDTO struct {
	Baudrate *uint
	Databits *uint8
	Stopbits *uint8
	Parity   *string
}

type sttyInDTO struct {
	Baudrate uint  `json:"baudrate"`
	Databits uint8 `json:"bits"`
	Stopbits uint8 `json:"stop"`
	Parity   *int  `json:"parity"`
}

func GetBaseUrl(scheme *string, host *string, port int) url.URL {
	ret := url.URL{
		Scheme: *scheme,
		Host:   fmt.Sprintf("%s:%d", *host, port),
	}
	return ret
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
	sttyIn := sttyInDTO{}
	err = json.Unmarshal(buf, &sttyIn)
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

func Stty(url *url.URL, credentials *url.Userinfo, dto *SttyDTO) error {
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
		return err
	}
	resp, err = utils.EnsureAuth(resp, credentials, bytes.NewBuffer([]byte(sb.String())))
	if err != nil {
		Trace()
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return nil
}

func TtycErrFprintf(w io.Writer, format string, a ...interface{}) {
	// Ignore fprintf errors here since I wasn't planning to care anywhere else regardless
	_, _ = fmt.Fprintf(w, "\u001B[31m[ttyc %s] ", strftime.FormatString(time.Now()))
	_, _ = fmt.Fprintf(w, format, a...)
	_, _ = fmt.Fprint(w, "\u001b[0m")
}

func TtycFprintf(w io.Writer, format string, a ...interface{}) {
	// Ignore fprintf errors here since I wasn't planning to care anywhere else regardless
	_, _ = fmt.Fprintf(w, "\u001B[33;1m[ttyc %s] ", strftime.FormatString(time.Now()))
	_, _ = fmt.Fprintf(w, format, a...)
	_, _ = fmt.Fprint(w, "\u001b[0m")
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
