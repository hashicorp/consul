package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// PublicListener provides an implementation of Proxier that listens for inbound
// mTLS connections, authenticates them with the local agent, and if successful
// forwards them to the locally configured app.
type PublicListener struct {
	cfg *PublicListenerConfig
}

// PublicListenerConfig contains the most basic parameters needed to start the
// proxy.
//
// Note that the tls.Configs here are expected to be "dynamic" in the sense that
// they are expected to use `GetConfigForClient` (added in go 1.8) to return
// dynamic config per connection if required.
type PublicListenerConfig struct {
	// BindAddress is the host:port the public mTLS listener will bind to.
	BindAddress string `json:"bind_address" hcl:"bind_address"`

	// LocalServiceAddress is the host:port for the proxied application. This
	// should be on loopback or otherwise protected as it's plain TCP.
	LocalServiceAddress string `json:"local_service_address" hcl:"local_service_address"`

	// TLSConfig config is used for the mTLS listener.
	TLSConfig *tls.Config

	// LocalConnectTimeout is the timeout for establishing connections with the
	// local backend. Defaults to 1000 (1s).
	LocalConnectTimeoutMs int `json:"local_connect_timeout_ms" hcl:"local_connect_timeout_ms"`

	// HandshakeTimeout is the timeout for incoming mTLS clients to complete a
	// handshake. Setting this low avoids DOS by malicious clients holding
	// resources open. Defaults to 10000 (10s).
	HandshakeTimeoutMs int `json:"handshake_timeout_ms" hcl:"handshake_timeout_ms"`

	logger *log.Logger
}

func (plc *PublicListenerConfig) applyDefaults() {
	if plc.LocalConnectTimeoutMs == 0 {
		plc.LocalConnectTimeoutMs = 1000
	}
	if plc.HandshakeTimeoutMs == 0 {
		plc.HandshakeTimeoutMs = 10000
	}
	if plc.logger == nil {
		plc.logger = log.New(os.Stdout, "", log.LstdFlags)
	}
}

// NewPublicListener returns a proxy instance with the given config.
func NewPublicListener(cfg PublicListenerConfig) *PublicListener {
	p := &PublicListener{
		cfg: &cfg,
	}
	p.cfg.applyDefaults()
	return p
}

// Listener implements Proxier
func (p *PublicListener) Listener() (net.Listener, error) {
	l, err := net.Listen("tcp", p.cfg.BindAddress)
	if err != nil {
		return nil, err
	}

	return tls.NewListener(l, p.cfg.TLSConfig), nil
}

// HandleConn implements Proxier
func (p *PublicListener) HandleConn(conn net.Conn) error {
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return fmt.Errorf("non-TLS conn")
	}

	// Setup Handshake timer
	to := time.Duration(p.cfg.HandshakeTimeoutMs) * time.Millisecond
	err := tlsConn.SetDeadline(time.Now().Add(to))
	if err != nil {
		return err
	}

	// Force TLS handshake so that abusive clients can't hold resources open
	err = tlsConn.Handshake()
	if err != nil {
		return err
	}

	// Handshake OK, clear the deadline
	err = tlsConn.SetDeadline(time.Time{})
	if err != nil {
		return err
	}

	// Huzzah, open a connection to the backend and let them talk
	// TODO maybe add a connection pool here?
	to = time.Duration(p.cfg.LocalConnectTimeoutMs) * time.Millisecond
	dst, err := net.DialTimeout("tcp", p.cfg.LocalServiceAddress, to)
	if err != nil {
		return fmt.Errorf("failed dialling local app: %s", err)
	}

	p.cfg.logger.Printf("[DEBUG] accepted connection from %s", conn.RemoteAddr())

	// Hand conn and dst over to Conn to manage the byte copying.
	c := NewConn(conn, dst)
	return c.CopyBytes()
}
