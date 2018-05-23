package dnsserver

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/coredns/coredns/plugin/pkg/nonwriter"
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

// DoHWriter is a nonwriter.Writer that adds more specific LocalAddr and RemoteAddr methods.
type DoHWriter struct {
	nonwriter.Writer

	// raddr is the remote's address. This can be optionally set.
	raddr net.Addr
	// laddr is our address. This can be optionally set.
	laddr net.Addr
}

// RemoteAddr returns the remote address.
func (d *DoHWriter) RemoteAddr() net.Addr { return d.raddr }

// LocalAddr returns the local address.
func (d *DoHWriter) LocalAddr() net.Addr { return d.laddr }
