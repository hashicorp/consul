package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/connect"
)

// Listener is the implementation of a specific proxy listener. It has pluggable
// Listen and Dial methods to suit public mTLS vs upstream semantics. It handles
// the lifecycle of the listener and all connections opened through it
type Listener struct {
	// Service is the connect service instance to use.
	Service *connect.Service

	listenFunc func() (net.Listener, error)
	dialFunc   func() (net.Conn, error)

	stopFlag int32
	stopChan chan struct{}

	logger *log.Logger
}

// NewPublicListener returns a Listener setup to listen for public mTLS
// connections and proxy them to the configured local application over TCP.
func NewPublicListener(svc *connect.Service, cfg PublicListenerConfig,
	logger *log.Logger) *Listener {
	return &Listener{
		Service: svc,
		listenFunc: func() (net.Listener, error) {
			return tls.Listen("tcp", cfg.BindAddress, svc.ServerTLSConfig())
		},
		dialFunc: func() (net.Conn, error) {
			return net.DialTimeout("tcp", cfg.LocalServiceAddress,
				time.Duration(cfg.LocalConnectTimeoutMs)*time.Millisecond)
		},
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

// NewUpstreamListener returns a Listener setup to listen locally for TCP
// connections that are proxied to a discovered Connect service instance.
func NewUpstreamListener(svc *connect.Service, cfg UpstreamConfig,
	logger *log.Logger) *Listener {
	return &Listener{
		Service: svc,
		listenFunc: func() (net.Listener, error) {
			return net.Listen("tcp", cfg.LocalBindAddress)
		},
		dialFunc: func() (net.Conn, error) {
			if cfg.resolver == nil {
				return nil, errors.New("no resolver provided")
			}
			ctx, cancel := context.WithTimeout(context.Background(),
				time.Duration(cfg.ConnectTimeoutMs)*time.Millisecond)
			defer cancel()
			return svc.Dial(ctx, cfg.resolver)
		},
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

// Serve runs the listener until it is stopped.
func (l *Listener) Serve() error {
	listen, err := l.listenFunc()
	if err != nil {
		return err
	}

	for {
		conn, err := listen.Accept()
		if err != nil {
			if atomic.LoadInt32(&l.stopFlag) == 1 {
				return nil
			}
			return err
		}

		go l.handleConn(conn)
	}
}

// handleConn is the internal connection handler goroutine.
func (l *Listener) handleConn(src net.Conn) {
	defer src.Close()

	dst, err := l.dialFunc()
	if err != nil {
		l.logger.Printf("[ERR] failed to dial: %s", err)
		return
	}
	// Note no need to defer dst.Close() since conn handles that for us.
	conn := NewConn(src, dst)
	defer conn.Close()

	err = conn.CopyBytes()
	if err != nil {
		l.logger.Printf("[ERR] connection failed: %s", err)
		return
	}
}

// Close terminates the listener and all active connections.
func (l *Listener) Close() error {
	return nil
}
