package connect

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"sync"
)

// defaultTLSConfig returns the standard config for connect clients and servers.
func defaultTLSConfig() *tls.Config {
	serverAuther := &ServerAuther{}
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
		InsecureSkipVerify: true,
		// By default auth as if we are a server. Clients need to override this with
		// an Auther that is performs correct validation of the server identity they
		// intended to connect to.
		VerifyPeerCertificate: serverAuther.Auth,
	}
}

// ReloadableTLSConfig exposes a tls.Config that can have it's certificates
// reloaded. This works by
type ReloadableTLSConfig struct {
	mu sync.Mutex

	// cfg is the current config to use for new connections
	cfg *tls.Config
}

// NewReloadableTLSConfig returns a reloadable config currently set to base. The
// Auther used to verify certificates for incoming connections on a Server will
// just be copied from the VerifyPeerCertificate passed. Clients will need to
// pass a specific Auther instance when they call TLSConfig that is configured
// to perform the necessary validation of the server's identity.
func NewReloadableTLSConfig(base *tls.Config) *ReloadableTLSConfig {
	return &ReloadableTLSConfig{cfg: base}
}

// ServerTLSConfig returns a *tls.Config that will dynamically load certs for
// each inbound connection via the GetConfigForClient callback.
func (c *ReloadableTLSConfig) ServerTLSConfig() *tls.Config {
	// Setup the basic one with current params even though we will be using
	// different config for each new conn.
	c.mu.Lock()
	base := c.cfg
	c.mu.Unlock()

	// Dynamically fetch the current config for each new inbound connection
	base.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		return c.TLSConfig(nil), nil
	}

	return base
}

// TLSConfig returns the current value for the config. It is safe to call from
// any goroutine. The passed Auther is inserted into the config's
// VerifyPeerCertificate. Passing a nil Auther will leave the default one in the
// base config
func (c *ReloadableTLSConfig) TLSConfig(auther Auther) *tls.Config {
	c.mu.Lock()
	cfgCopy := c.cfg
	c.mu.Unlock()
	if auther != nil {
		cfgCopy.VerifyPeerCertificate = auther.Auth
	}
	return cfgCopy
}

// SetTLSConfig sets the config used for future connections. It is safe to call
// from any goroutine.
func (c *ReloadableTLSConfig) SetTLSConfig(cfg *tls.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
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

	cfg := defaultTLSConfig()

	cfg.Certificates = []tls.Certificate{cert}
	cfg.RootCAs = roots
	cfg.ClientCAs = roots

	return cfg, nil
}
