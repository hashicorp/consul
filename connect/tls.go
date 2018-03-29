package connect

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"sync"

	"github.com/hashicorp/consul/agent/connect"
)

// verifyFunc is the type of tls.Config.VerifyPeerCertificate for convenience.
type verifyFunc func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error

// defaultTLSConfig returns the standard config.
func defaultTLSConfig(verify verifyFunc) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		// We don't have access to go internals that decide if AES hardware
		// acceleration is available in order to prefer CHA CHA if not. So let's
		// just always prefer AES for now. We can look into doing something uglier
		// later like using an external lib for AES checking if it seems important.
		// https://github.com/golang/go/blob/df91b8044dbe790c69c16058330f545be069cc1f/src/crypto/tls/common.go#L919:14
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
		// We have to set this since otherwise Go will attempt to verify DNS names
		// match DNS SAN/CN which we don't want. We hook up VerifyPeerCertificate to
		// do our own path validation as well as Connect AuthZ.
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verify,
		// Include h2 to allow connect http servers to automatically support http2.
		// See: https://github.com/golang/go/blob/917c33fe8672116b04848cf11545296789cafd3b/src/net/http/server.go#L2724-L2731
		NextProtos: []string{"h2"},
	}
}

// ReloadableTLSConfig exposes a tls.Config that can have it's certificates
// reloaded. On a server, this uses GetConfigForClient to pass the current
// tls.Config or client certificate for each acceptted connection. On a client,
// this uses GetClientCertificate to provide the current client certificate.
type ReloadableTLSConfig struct {
	mu sync.Mutex

	// cfg is the current config to use for new connections
	cfg *tls.Config
}

// NewReloadableTLSConfig returns a reloadable config currently set to base.
func NewReloadableTLSConfig(base *tls.Config) *ReloadableTLSConfig {
	c := &ReloadableTLSConfig{}
	c.SetTLSConfig(base)
	return c
}

// TLSConfig returns a *tls.Config that will dynamically load certs. It's
// suitable for use in either a client or server.
func (c *ReloadableTLSConfig) TLSConfig() *tls.Config {
	c.mu.Lock()
	cfgCopy := c.cfg
	c.mu.Unlock()
	return cfgCopy
}

// SetTLSConfig sets the config used for future connections. It is safe to call
// from any goroutine.
func (c *ReloadableTLSConfig) SetTLSConfig(cfg *tls.Config) error {
	copy := cfg.Clone()
	copy.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		current := c.TLSConfig()
		if len(current.Certificates) < 1 {
			return nil, errors.New("tls: no certificates configured")
		}
		return &current.Certificates[0], nil
	}
	copy.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.TLSConfig(), nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = copy
	return nil
}

// devTLSConfigFromFiles returns a default TLS Config but with certs and CAs
// based on local files for dev.
func devTLSConfigFromFiles(caFile, certFile,
	keyFile string) (*tls.Config, error) {

	roots := x509.NewCertPool()

	bs, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	roots.AppendCertsFromPEM(bs)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	// Insecure no verification
	cfg := defaultTLSConfig(nil)

	cfg.Certificates = []tls.Certificate{cert}
	cfg.RootCAs = roots
	cfg.ClientCAs = roots

	return cfg, nil
}

// verifyServerCertMatchesURI is used on tls connections dialled to a connect
// server to ensure that the certificate it presented has the correct identity.
func verifyServerCertMatchesURI(certs []*x509.Certificate,
	expected connect.CertURI) error {
	expectedStr := expected.URI().String()

	if len(certs) < 1 {
		return errors.New("peer certificate mismatch")
	}

	// Only check the first cert assuming this is the only leaf. It's not clear if
	// services might ever legitimately present multiple leaf certificates or if
	// the slice is just to allow presenting the whole chain of intermediates.
	cert := certs[0]

	// Our certs will only ever have a single URI for now so only check that
	if len(cert.URIs) < 1 {
		return errors.New("peer certificate mismatch")
	}
	// We may want to do better than string matching later in some special
	// cases and/or encapsulate the "match" logic inside the CertURI
	// implementation but for now this is all we need.
	if cert.URIs[0].String() == expectedStr {
		return nil
	}
	return errors.New("peer certificate mismatch")
}

// serverVerifyCerts is the verifyFunc for use on Connect servers.
func serverVerifyCerts(rawCerts [][]byte, chains [][]*x509.Certificate) error {
	// TODO(banks): implement me
	return nil
}

// clientVerifyCerts is the verifyFunc for use on Connect clients.
func clientVerifyCerts(rawCerts [][]byte, chains [][]*x509.Certificate) error {
	// TODO(banks): implement me
	return nil
}
