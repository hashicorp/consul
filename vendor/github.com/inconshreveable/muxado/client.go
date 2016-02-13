package muxado

import (
	"crypto/tls"
	"github.com/inconshreveable/muxado/proto"
	"github.com/inconshreveable/muxado/proto/ext"
	"net"
)

// Client returns a new muxado client-side connection using conn as the transport.
func Client(conn net.Conn) Session {
	return &sessionAdaptor{proto.NewSession(conn, proto.NewStream, true, []proto.Extension{ext.NewDefaultHeartbeat()})}
}

// Dial opens a new connection to the given network/address and then beings a muxado client session on it.
func Dial(network, addr string) (sess Session, err error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return
	}
	return Client(conn), nil
}

// DialTLS opens a new TLS encrytped connection with the givent configuration
// to the network/address and then beings a muxado client session on it.
func DialTLS(network, addr string, tlsConfig *tls.Config) (sess Session, err error) {
	conn, err := tls.Dial(network, addr, tlsConfig)
	if err != nil {
		return
	}
	return Client(conn), nil
}
