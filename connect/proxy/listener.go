package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/connect"
)

const (
	publicListenerMetricPrefix = "inbound"
	upstreamMetricPrefix       = "upstream"
)

// Listener is the implementation of a specific proxy listener. It has pluggable
// Listen and Dial methods to suit public mTLS vs upstream semantics. It handles
// the lifecycle of the listener and all connections opened through it
type Listener struct {
	// Service is the connect service instance to use.
	Service *connect.Service

	// listenFunc, dialFunc and bindAddr are set by type-specific constructors
	listenFunc func() (net.Listener, error)
	dialFunc   func() (net.Conn, error)
	bindAddr   string

	stopFlag int32
	stopChan chan struct{}

	// listeningChan is closed when listener is opened successfully. It's really
	// only for use in tests where we need to coordinate wait for the Serve
	// goroutine to be running before we proceed trying to connect. On my laptop
	// this always works out anyway but on constrained VMs and especially docker
	// containers (e.g. in CI) we often see the Dial routine win the race and get
	// `connection refused`. Retry loops and sleeps are unpleasant workarounds and
	// this is cheap and correct.
	listeningChan chan struct{}

	logger *log.Logger

	// Gauge to track current open connections
	activeConns  int64
	connWG       sync.WaitGroup
	metricPrefix string
	metricLabels []metrics.Label
}

// NewPublicListener returns a Listener setup to listen for public mTLS
// connections and proxy them to the configured local application over TCP.
func NewPublicListener(svc *connect.Service, cfg PublicListenerConfig,
	logger *log.Logger) *Listener {
	bindAddr := fmt.Sprintf("%s:%d", cfg.BindAddress, cfg.BindPort)
	return &Listener{
		Service: svc,
		listenFunc: func() (net.Listener, error) {
			return tls.Listen("tcp", bindAddr, svc.ServerTLSConfig())
		},
		dialFunc: func() (net.Conn, error) {
			return net.DialTimeout("tcp", cfg.LocalServiceAddress,
				time.Duration(cfg.LocalConnectTimeoutMs)*time.Millisecond)
		},
		bindAddr:      bindAddr,
		stopChan:      make(chan struct{}),
		listeningChan: make(chan struct{}),
		logger:        logger,
		metricPrefix:  publicListenerMetricPrefix,
		// For now we only label ourselves as source - we could fetch the src
		// service from cert on each connection and label metrics differently but it
		// significaly complicates the active connection tracking here and it's not
		// clear that it's very valuable - on aggregate looking at all _outbound_
		// connections across all proxies gets you a full picture of src->dst
		// traffic. We might expand this later for better debugging of which clients
		// are abusing a particular service instance but we'll see how valuable that
		// seems for the extra complication of tracking many gauges here.
		metricLabels: []metrics.Label{{Name: "dst", Value: svc.Name()}},
	}
}

// NewUpstreamListener returns a Listener setup to listen locally for TCP
// connections that are proxied to a discovered Connect service instance.
func NewUpstreamListener(svc *connect.Service, cfg UpstreamConfig,
	logger *log.Logger) *Listener {
	bindAddr := fmt.Sprintf("%s:%d", cfg.LocalBindAddress, cfg.LocalBindPort)
	return &Listener{
		Service: svc,
		listenFunc: func() (net.Listener, error) {
			return net.Listen("tcp", bindAddr)
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
		bindAddr:      bindAddr,
		stopChan:      make(chan struct{}),
		listeningChan: make(chan struct{}),
		logger:        logger,
		metricPrefix:  upstreamMetricPrefix,
		metricLabels: []metrics.Label{
			{Name: "src", Value: svc.Name()},
			// TODO(banks): namespace support
			{Name: "dst_type", Value: cfg.DestinationType},
			{Name: "dst", Value: cfg.DestinationName},
		},
	}
}

// Serve runs the listener until it is stopped. It is an error to call Serve
// more than once for any given Listener instance.
func (l *Listener) Serve() error {
	// Ensure we mark state closed if we fail before Close is called externally.
	defer l.Close()

	if atomic.LoadInt32(&l.stopFlag) != 0 {
		return errors.New("serve called on a closed listener")
	}

	listen, err := l.listenFunc()
	if err != nil {
		return err
	}
	close(l.listeningChan)

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

	// Track active conn now (first function call) and defer un-counting it when
	// it closes.
	defer l.trackConn()()

	// Make sure Close() waits for this conn to be cleaned up. Note defer is
	// before conn.Close() so runs after defer conn.Close().
	l.connWG.Add(1)
	defer l.connWG.Done()

	// Note no need to defer dst.Close() since conn handles that for us.
	conn := NewConn(src, dst)
	defer conn.Close()

	connStop := make(chan struct{})

	// Run another goroutine to copy the bytes.
	go func() {
		err = conn.CopyBytes()
		if err != nil {
			l.logger.Printf("[ERR] connection failed: %s", err)
		}
		close(connStop)
	}()

	// Periodically copy stats from conn to metrics (to keep metrics calls out of
	// the path of every single packet copy). 5 seconds is probably good enough
	// resolution - statsd and most others tend to summarize with lower resolution
	// anyway and this amortizes the cost more.
	var tx, rx uint64
	statsT := time.NewTicker(5 * time.Second)
	defer statsT.Stop()

	reportStats := func() {
		newTx, newRx := conn.Stats()
		if delta := newTx - tx; delta > 0 {
			metrics.IncrCounterWithLabels([]string{l.metricPrefix, "tx_bytes"},
				float32(newTx-tx), l.metricLabels)
		}
		if delta := newRx - rx; delta > 0 {
			metrics.IncrCounterWithLabels([]string{l.metricPrefix, "rx_bytes"},
				float32(newRx-rx), l.metricLabels)
		}
		tx, rx = newTx, newRx
	}
	// Always report final stats for the conn.
	defer reportStats()

	// Wait for conn to close
	for {
		select {
		case <-connStop:
			return
		case <-l.stopChan:
			return
		case <-statsT.C:
			reportStats()
		}
	}
}

// trackConn increments the count of active conns and returns a func() that can
// be deferred on to decrement the counter again on connection close.
func (l *Listener) trackConn() func() {
	c := atomic.AddInt64(&l.activeConns, 1)
	metrics.SetGaugeWithLabels([]string{l.metricPrefix, "conns"}, float32(c),
		l.metricLabels)

	return func() {
		c := atomic.AddInt64(&l.activeConns, -1)
		metrics.SetGaugeWithLabels([]string{l.metricPrefix, "conns"}, float32(c),
			l.metricLabels)
	}
}

// Close terminates the listener and all active connections.
func (l *Listener) Close() error {
	oldFlag := atomic.SwapInt32(&l.stopFlag, 1)
	if oldFlag == 0 {
		close(l.stopChan)
		// Wait for all conns to close
		l.connWG.Wait()
	}
	return nil
}

// Wait for the listener to be ready to accept connections.
func (l *Listener) Wait() {
	<-l.listeningChan
}

// BindAddr returns the address the listen is bound to.
func (l *Listener) BindAddr() string {
	return l.bindAddr
}
