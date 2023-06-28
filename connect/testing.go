// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/freeport"
)

// TestService returns a Service instance based on a static TLS Config.
func TestService(t testing.T, service string, ca *structs.CARoot) *Service {
	t.Helper()

	// Don't need to talk to client since we are setting TLSConfig locally
	logger := hclog.New(&hclog.LoggerOptions{})
	svc, err := NewDevServiceWithTLSConfig(service,
		logger, TestTLSConfig(t, service, ca))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// TestTLSConfig returns a *tls.Config suitable for use during tests.
func TestTLSConfig(t testing.T, service string, ca *structs.CARoot) *tls.Config {
	t.Helper()

	cfg := defaultTLSConfig()
	cfg.Certificates = []tls.Certificate{TestSvcKeyPair(t, service, ca)}
	cfg.RootCAs = TestCAPool(t, ca)
	cfg.ClientCAs = TestCAPool(t, ca)
	return cfg
}

// TestCAPool returns an *x509.CertPool containing the passed CA's root(s)
func TestCAPool(t testing.T, cas ...*structs.CARoot) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	for _, ca := range cas {
		pool.AppendCertsFromPEM([]byte(ca.RootCert))
	}
	return pool
}

// TestSvcKeyPair returns an tls.Certificate containing both cert and private
// key for a given service under a given CA from the testdata dir.
func TestSvcKeyPair(t testing.T, service string, ca *structs.CARoot) tls.Certificate {
	t.Helper()
	certPEM, keyPEM := connect.TestLeaf(t, service, ca)
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// TestPeerCertificates returns a []*x509.Certificate as you'd get from
// tls.Conn.ConnectionState().PeerCertificates including the named certificate.
func TestPeerCertificates(t testing.T, service string, ca *structs.CARoot) []*x509.Certificate {
	t.Helper()
	certPEM, _ := connect.TestLeaf(t, service, ca)
	cert, err := connect.ParseCert(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	return []*x509.Certificate{cert}
}

// TestServer runs a service listener that can be used to test clients. It's
// behavior can be controlled by the struct members.
type TestServer struct {
	// The service name to serve.
	Service string
	// The (test) CA to use for generating certs.
	CA *structs.CARoot
	// TimeoutHandshake controls whether the listening server will complete a TLS
	// handshake quickly enough.
	TimeoutHandshake bool
	// TLSCfg is the tls.Config that will be used. By default it's set up from the
	// service and ca set.
	TLSCfg *tls.Config
	// Addr is the listen address. It is set to a random free port on `localhost`
	// by default.
	Addr string
	// Listening is closed when the listener is run.
	Listening chan struct{}

	l        net.Listener
	stopFlag int32
	stopChan chan struct{}
}

// NewTestServer returns a TestServer. It should be closed when test is
// complete.
func NewTestServer(t testing.T, service string, ca *structs.CARoot) *TestServer {
	return &TestServer{
		Service:   service,
		CA:        ca,
		stopChan:  make(chan struct{}),
		TLSCfg:    TestTLSConfig(t, service, ca),
		Addr:      fmt.Sprintf("127.0.0.1:%d", freeport.GetOne(t)),
		Listening: make(chan struct{}),
	}
}

// Serve runs a tcp echo server and blocks until it is closed or errors. If
// TimeoutHandshake is set it won't start TLS handshake on new connections.
func (s *TestServer) Serve() error {
	// Just accept TCP conn but so we can control timing of accept/handshake
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.l = l
	close(s.Listening)
	log.Printf("test connect service listening on %s", s.Addr)

	for {
		conn, err := s.l.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.stopFlag) == 1 {
				return nil
			}
			return err
		}

		// Ignore the conn if we are not actively handshaking
		if !s.TimeoutHandshake {
			// Upgrade conn to TLS
			conn = tls.Server(conn, s.TLSCfg)

			// Run an echo service
			log.Printf("test connect service accepted conn from %s, "+
				" running echo service", conn.RemoteAddr())
			go io.Copy(conn, conn)
		}

		// Close this conn when we stop
		go func(c net.Conn) {
			<-s.stopChan
			c.Close()
		}(conn)
	}
}

// ServeHTTPS runs an HTTPS server with the given config. It invokes the passed
// Handler for all requests.
func (s *TestServer) ServeHTTPS(h http.Handler) error {
	srv := http.Server{
		Addr:      s.Addr,
		TLSConfig: s.TLSCfg,
		Handler:   h,
	}
	log.Printf("starting test connect HTTPS server on %s", s.Addr)

	// Use our own listener so we can signal when it's ready.
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	close(s.Listening)
	s.l = l
	log.Printf("test connect service listening on %s", s.Addr)

	err = srv.ServeTLS(l, "", "")
	if atomic.LoadInt32(&s.stopFlag) == 1 {
		return nil
	}
	return err
}

// Close stops a TestServer
func (s *TestServer) Close() error {
	old := atomic.SwapInt32(&s.stopFlag, 1)
	if old == 0 {
		if s.l != nil {
			s.l.Close()
		}
		close(s.stopChan)
	}
	return nil
}
