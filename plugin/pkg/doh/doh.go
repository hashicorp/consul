package doh

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/miekg/dns"
)

// MimeType is the DoH mimetype that should be used.
const MimeType = "application/dns-message"

// Path is the URL path that should be used.
const Path = "/dns-query"

// NewRequest returns a new DoH request given a method, URL (without any paths, so exclude /dns-query) and dns.Msg.
func NewRequest(method, url string, m *dns.Msg) (*http.Request, error) {
	buf, err := m.Pack()
	if err != nil {
		return nil, err
	}

	switch method {
	case http.MethodGet:
		b64 := base64.RawURLEncoding.EncodeToString(buf)

		req, err := http.NewRequest(http.MethodGet, "https://"+url+Path+"?dns="+b64, nil)
		if err != nil {
			return req, err
		}

		req.Header.Set("content-type", MimeType)
		req.Header.Set("accept", MimeType)
		return req, nil

	case http.MethodPost:
		req, err := http.NewRequest(http.MethodPost, "https://"+url+Path+"?bla=foo:443", bytes.NewReader(buf))
		if err != nil {
			return req, err
		}

		req.Header.Set("content-type", MimeType)
		req.Header.Set("accept", MimeType)
		return req, nil

	default:
		return nil, fmt.Errorf("method not allowed: %s", method)
	}

}

// ResponseToMsg converts a http.Response to a dns message.
func ResponseToMsg(resp *http.Response) (*dns.Msg, error) {
	defer resp.Body.Close()

	return toMsg(resp.Body)
}

// RequestToMsg converts a http.Request to a dns message.
func RequestToMsg(req *http.Request) (*dns.Msg, error) {
	switch req.Method {
	case http.MethodGet:
		return requestToMsgGet(req)

	case http.MethodPost:
		return requestToMsgPost(req)

	default:
		return nil, fmt.Errorf("method not allowed: %s", req.Method)
	}

}

// requestToMsgPost extracts the dns message from the request body.
func requestToMsgPost(req *http.Request) (*dns.Msg, error) {
	defer req.Body.Close()
	return toMsg(req.Body)
}

// requestToMsgGet extract the dns message from the GET request.
func requestToMsgGet(req *http.Request) (*dns.Msg, error) {
	values := req.URL.Query()
	b64, ok := values["dns"]
	if !ok {
		return nil, fmt.Errorf("no 'dns' query parameter found")
	}
	if len(b64) != 1 {
		return nil, fmt.Errorf("multiple 'dns' query values found")
	}
	return base64ToMsg(b64[0])
}

func toMsg(r io.ReadCloser) (*dns.Msg, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	m := new(dns.Msg)
	err = m.Unpack(buf)
	return m, err
}

func base64ToMsg(b64 string) (*dns.Msg, error) {
	buf, err := b64Enc.DecodeString(b64)
	if err != nil {
		return nil, err
	}

	m := new(dns.Msg)
	err = m.Unpack(buf)

	return m, err
}

var b64Enc = base64.RawURLEncoding
