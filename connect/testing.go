package connect

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/freeport"
	testing "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// testVerifier creates a helper verifyFunc that can be set in a tls.Config and
// records calls made, passing back the certificates presented via the returned
// channel. The channel is buffered so up to 128 verification calls can be made
// without reading the chan before verification blocks.
func testVerifier(t testing.T, returnErr error) (verifyFunc, chan [][]byte) {
	ch := make(chan [][]byte, 128)
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		ch <- rawCerts
		return returnErr
	}, ch
}

// TestTLSConfig returns a *tls.Config suitable for use during tests.
func TestTLSConfig(t testing.T, service string, ca *structs.CARoot) *tls.Config {
	t.Helper()

	// Insecure default (nil verifier)
	cfg := defaultTLSConfig(nil)
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
	require.Nil(t, err)
	return cert
}

// TestPeerCertificates returns a []*x509.Certificate as you'd get from
// tls.Conn.ConnectionState().PeerCertificates including the named certificate.
func TestPeerCertificates(t testing.T, service string, ca *structs.CARoot) []*x509.Certificate {
	t.Helper()
	certPEM, _ := connect.TestLeaf(t, service, ca)
	cert, err := connect.ParseCert(certPEM)
	require.Nil(t, err)
	return []*x509.Certificate{cert}
}

// TestService runs a service listener that can be used to test clients. It's
// behaviour can be controlled by the struct members.
type TestService struct {
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

	l        net.Listener
	stopFlag int32
	stopChan chan struct{}
}

// NewTestService returns a TestService. It should be closed when test is
// complete.
func NewTestService(t testing.T, service string, ca *structs.CARoot) *TestService {
	ports := freeport.GetT(t, 1)
	return &TestService{
		Service:  service,
		CA:       ca,
		stopChan: make(chan struct{}),
		TLSCfg:   TestTLSConfig(t, service, ca),
		Addr:     fmt.Sprintf("localhost:%d", ports[0]),
	}
}

// Serve runs a TestService and blocks until it is closed or errors.
func (s *TestService) Serve() error {
	// Just accept TCP conn but so we can control timing of accept/handshake
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.l = l

	for {
		conn, err := s.l.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.stopFlag) == 1 {
				return nil
			}
			return err
		}

		// Ignore the conn if we are not actively ha
		if !s.TimeoutHandshake {
			// Upgrade conn to TLS
			conn = tls.Server(conn, s.TLSCfg)

			// Run an echo service
			go io.Copy(conn, conn)
		}

		// Close this conn when we stop
		go func(c net.Conn) {
			<-s.stopChan
			c.Close()
		}(conn)
	}

	return nil
}

// Close stops a TestService
func (s *TestService) Close() {
	old := atomic.SwapInt32(&s.stopFlag, 1)
	if old == 0 {
		if s.l != nil {
			s.l.Close()
		}
		close(s.stopChan)
	}
}
