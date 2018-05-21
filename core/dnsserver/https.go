package dnsserver

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/miekg/dns"
)

// mimeTypeDOH is the DoH mimetype that should be used.
const mimeTypeDOH = "application/dns-message"

// pathDOH is the URL path that should be used.
const pathDOH = "/dns-query"

// postRequestToMsg extracts the dns message from the request body.
func postRequestToMsg(req *http.Request) (*dns.Msg, error) {
	defer req.Body.Close()

	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	m := new(dns.Msg)
	err = m.Unpack(buf)
	return m, err
}

// getRequestToMsg extract the dns message from the GET request.
func getRequestToMsg(req *http.Request) (*dns.Msg, error) {
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
