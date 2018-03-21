package connect

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// testDataDir is a janky temporary hack to allow use of these methods from
// proxy package. We need to revisit where all this lives since it logically
// overlaps with consul/agent in Mitchell's PR and that one generates certs on
// the fly which will make this unecessary but I want to get things working for
// now with what I've got :). This wonderful heap kinda-sorta gets the path
// relative to _this_ file so it works even if the Test* method is being called
// from a test binary in another package dir.
func testDataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("no caller information")
	}
	return path.Dir(filename) + "/testdata"
}

// TestCAPool returns an *x509.CertPool containing the named CA certs from the
// testdata dir.
func TestCAPool(t testing.T, caNames ...string) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	for _, name := range caNames {
		certs, err := filepath.Glob(testDataDir() + "/" + name + "-ca-*.cert.pem")
		require.Nil(t, err)
		for _, cert := range certs {
			caPem, err := ioutil.ReadFile(cert)
			require.Nil(t, err)
			pool.AppendCertsFromPEM(caPem)
		}
	}
	return pool
}

// TestSvcKeyPair returns an tls.Certificate containing both cert and private
// key for a given service under a given CA from the testdata dir.
func TestSvcKeyPair(t testing.T, ca, name string) tls.Certificate {
	t.Helper()
	prefix := fmt.Sprintf(testDataDir()+"/%s-svc-%s", ca, name)
	cert, err := tls.LoadX509KeyPair(prefix+".cert.pem", prefix+".key.pem")
	require.Nil(t, err)
	return cert
}

// TestTLSConfig returns a *tls.Config suitable for use during tests.
func TestTLSConfig(t testing.T, ca, svc string) *tls.Config {
	t.Helper()
	return &tls.Config{
		Certificates: []tls.Certificate{TestSvcKeyPair(t, ca, svc)},
		MinVersion:   tls.VersionTLS12,
		RootCAs:      TestCAPool(t, ca),
		ClientCAs:    TestCAPool(t, ca),
		ClientAuth:   tls.RequireAndVerifyClientCert,
		// In real life we'll need to do this too since otherwise Go will attempt to
		// verify DNS names match DNS SAN/CN which we don't want, but we'll hook
		// VerifyPeerCertificates and do our own x509 path validation as well as
		// AuthZ upcall. For now we are just testing the basic proxy mechanism so
		// this is fine.
		InsecureSkipVerify: true,
	}
}

// TestAuther is a simple Auther implementation that does nothing but what you
// tell it to!
type TestAuther struct {
	// Return is the value returned from an Auth() call. Set it to nil to have all
	// certificates unconditionally accepted or a value to have them fail.
	Return error
}

// Auth implements Auther
func (a *TestAuther) Auth(rawCerts [][]byte,
	verifiedChains [][]*x509.Certificate) error {
	return a.Return
}
