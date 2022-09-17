package utils

// Adapted from https://github.com/ryanjdew/http-digest-auth-client
// License: Apache 2.0

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DigestHeaders tracks the state of authentication
type DigestHeaders struct {
	Realm     string
	Qop       string
	Method    string
	Nonce     string
	Opaque    string
	Algorithm string
	HA1       string
	HA2       string
	Cnonce    string
	Path      string
	Nc        int16
	Username  string
	Password  string
}

func (d *DigestHeaders) digestChecksum() {
	switch d.Algorithm {
	case "MD5":
		// A1
		h := md5.New()
		A1 := fmt.Sprintf("%s:%s:%s", d.Username, d.Realm, d.Password)
		io.WriteString(h, A1)
		d.HA1 = fmt.Sprintf("%x", h.Sum(nil))

		// A2
		h = md5.New()
		A2 := fmt.Sprintf("%s:%s", d.Method, d.Path)
		io.WriteString(h, A2)
		d.HA2 = fmt.Sprintf("%x", h.Sum(nil))
	case "MD5-sess":
		// A1
		h := md5.New()
		A1 := fmt.Sprintf("%s:%s:%s", d.Username, d.Realm, d.Password)
		io.WriteString(h, A1)
		haPre := fmt.Sprintf("%x", h.Sum(nil))
		h = md5.New()
		A1 = fmt.Sprintf("%s:%s:%s", haPre, d.Nonce, d.Cnonce)
		io.WriteString(h, A1)
		d.HA1 = fmt.Sprintf("%x", h.Sum(nil))

		// A2
		h = md5.New()
		A2 := fmt.Sprintf("%s:%s", d.Method, d.Path)
		io.WriteString(h, A2)
		d.HA2 = fmt.Sprintf("%x", h.Sum(nil))
	default:
		//token
	}
}

// ApplyAuth adds proper auth header to the passed request
func (d *DigestHeaders) ApplyAuth(req *http.Request) {
	d.Nc += 0x1
	d.Cnonce = randomKey()
	d.Method = req.Method
	d.Path = req.URL.RequestURI()
	d.digestChecksum()
	response := h(strings.Join([]string{d.HA1, d.Nonce, fmt.Sprintf("%08x", d.Nc),
		d.Cnonce, d.Qop, d.HA2}, ":"))
	AuthHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", cnonce="%s", nc=%08x, qop=%s, response="%s", algorithm=%s`,
		d.Username, d.Realm, d.Nonce, d.Path, d.Cnonce, d.Nc, d.Qop, response, d.Algorithm)
	if d.Opaque != "" {
		AuthHeader = fmt.Sprintf(`%s, opaque="%s"`, AuthHeader, d.Opaque)
	}
	req.Header.Set("Authorization", AuthHeader)
}

func EnsureAuth(resp *http.Response, auth *url.Userinfo, body io.Reader) (outResp *http.Response, err error) {
	if resp.StatusCode != 401 {
		return resp, nil
	}
	_ = resp.Body.Close()

	if auth == nil {
		return nil, fmt.Errorf("authentication required but credentials not provided")
	}
	if _, ok := auth.Password(); !ok {
		return nil, fmt.Errorf("authentication is required but password was not provided")
	}

	client := &http.Client{}
	wwwAuth := resp.Header.Get("Www-Authenticate")
	if strings.HasPrefix(strings.ToLower(wwwAuth), "basic") {
		reqUrl := *resp.Request.URL
		reqUrl.User = auth
		req, err := http.NewRequest(resp.Request.Method, resp.Request.URL.String(), body)
		if err != nil {
			return nil, err
		}
		req.Header = resp.Request.Header.Clone()
		outResp, err = client.Do(req)

		if outResp.StatusCode >= 400 {
			err = fmt.Errorf("unauthorized (HTTP %d)", outResp.StatusCode)
		}
		return outResp, err
	}

	authn := digestAuthParams(resp)
	if authn == nil {
		return nil, fmt.Errorf("unable to retrieve www-auth data from server")
	}

	algorithm := authn["algorithm"]
	d := &DigestHeaders{}
	d.Path = resp.Request.URL.RequestURI()
	d.Realm = authn["realm"]
	d.Qop = authn["qop"]
	d.Nonce = authn["nonce"]
	d.Opaque = authn["opaque"]
	if algorithm == "" {
		d.Algorithm = "MD5"
	} else {
		d.Algorithm = authn["algorithm"]
	}
	d.Nc = 0x0
	d.Username = auth.Username()
	pass, _ := auth.Password()
	d.Password = pass

	req, err := http.NewRequest(resp.Request.Method, resp.Request.URL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header = resp.Request.Header.Clone()
	d.ApplyAuth(req)
	outResp, err = client.Do(req)

	if outResp.StatusCode >= 400 {
		err = fmt.Errorf("unauthorized (HTTP %d)", outResp.StatusCode)
	}
	return
}

/*
Parse Authorization header from the http.Request. Returns a map of
auth parameters or nil if the header is not a valid parsable Digest
auth header.
*/
func digestAuthParams(r *http.Response) map[string]string {
	s := strings.SplitN(r.Header.Get("Www-Authenticate"), " ", 2)
	if len(s) != 2 || s[0] != "Digest" {
		return nil
	}

	result := map[string]string{}
	for _, kv := range strings.Split(s[1], ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.Trim(parts[0], "\" ")] = strings.Trim(parts[1], "\" ")
	}
	return result
}

func randomKey() string {
	k := make([]byte, 12)
	for bytes := 0; bytes < len(k); {
		n, err := rand.Read(k[bytes:])
		if err != nil {
			panic("rand.Read() failed")
		}
		bytes += n
	}
	return base64.StdEncoding.EncodeToString(k)
}

/*
H function for MD5 algorithm (returns a lower-case hex MD5 digest)
*/
func h(data string) string {
	digest := md5.New()
	digest.Write([]byte(data))
	return fmt.Sprintf("%x", digest.Sum(nil))
}
