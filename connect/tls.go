package connect

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"log"
	"sync"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
)

// verifierFunc is a function that can accept rawCertificate bytes from a peer
// and verify them against a given tls.Config. It's called from the
// tls.Config.VerifyPeerCertificate hook. We don't pass verifiedChains since
// that is always nil in our usage. Implementations can use the roots provided
// in the cfg to verify the certs.
type verifierFunc func(cfg *tls.Config, rawCerts [][]byte) error

// defaultTLSConfig returns the standard config.
func defaultTLSConfig(v verifierFunc) *tls.Config {
	cfg := &tls.Config{
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
		// Include h2 to allow connect http servers to automatically support http2.
		// See: https://github.com/golang/go/blob/917c33fe8672116b04848cf11545296789cafd3b/src/net/http/server.go#L2724-L2731
		NextProtos: []string{"h2"},
	}
	setVerifier(cfg, v)
	return cfg
}

// setVerifier takes a *tls.Config and set's it's VerifyPeerCertificates hook to
// use the passed verifierFunc.
func setVerifier(cfg *tls.Config, v verifierFunc) {
	if v != nil {
		cfg.VerifyPeerCertificate = func(rawCerts [][]byte, chains [][]*x509.Certificate) error {
			return v(cfg, rawCerts)
		}
	}
}

// reloadableTLSConfig exposes a tls.Config that can have it's certificates
// reloaded. On a server, this uses GetConfigForClient to pass the current
// tls.Config or client certificate for each acceptted connection. On a client,
// this uses GetClientCertificate to provide the current client certificate.
type reloadableTLSConfig struct {
	mu sync.Mutex

	// cfg is the current config to use for new connections
	cfg *tls.Config
}

// newReloadableTLSConfig returns a reloadable config currently set to base.
func newReloadableTLSConfig(base *tls.Config) *reloadableTLSConfig {
	c := &reloadableTLSConfig{}
	c.SetTLSConfig(base)
	return c
}

// TLSConfig returns a *tls.Config that will dynamically load certs. It's
// suitable for use in either a client or server.
func (c *reloadableTLSConfig) TLSConfig() *tls.Config {
	c.mu.Lock()
	cfgCopy := c.cfg
	c.mu.Unlock()
	return cfgCopy
}

// SetTLSConfig sets the config used for future connections. It is safe to call
// from any goroutine.
func (c *reloadableTLSConfig) SetTLSConfig(cfg *tls.Config) error {
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

// newServerSideVerifier returns a verifierFunc that wraps the provided
// api.Client to verify the TLS chain and perform AuthZ for the server end of
// the connection. The service name provided is used as the target serviceID
// for the Authorization.
func newServerSideVerifier(client *api.Client, serviceID string) verifierFunc {
	return func(tlsCfg *tls.Config, rawCerts [][]byte) error {
		leaf, err := verifyChain(tlsCfg, rawCerts, false)
		if err != nil {
			return err
		}

		// Check leaf is a cert we understand
		if len(leaf.URIs) < 1 {
			return errors.New("connect: invalid leaf certificate")
		}

		certURI, err := connect.ParseCertURI(leaf.URIs[0])
		if err != nil {
			return errors.New("connect: invalid leaf certificate URI")
		}

		// No AuthZ if there is no client.
		if client == nil {
			return nil
		}

		// Perform AuthZ
		req := &api.AgentAuthorizeParams{
			// TODO(banks): this is jank, we have a serviceID from the Service setup
			// but this needs to be a service name as the target. For now we are
			// relying on them usually being the same but this will break when they
			// are not. We either need to make Authorize endpoint optionally accept
			// IDs somehow or rethink this as it will require fetching the service
			// name sometime ahead of accepting requests (maybe along with TLS certs?)
			// which feels gross and will take extra plumbing to expose it to here.
			Target:           serviceID,
			ClientCertURI:    certURI.URI().String(),
			ClientCertSerial: connect.HexString(leaf.SerialNumber.Bytes()),
		}
		resp, err := client.Agent().ConnectAuthorize(req)
		if err != nil {
			return errors.New("connect: authz call failed: " + err.Error())
		}
		if !resp.Authorized {
			return errors.New("connect: authz denied: " + resp.Reason)
		}
		log.Println("[DEBUG] authz result", resp)
		return nil
	}
}

// clientSideVerifier is a verifierFunc that performs verification of certificates
// on the client end of the connection. For now it is just basic TLS
// verification since the identity check needs additional state and becomes
// clunky to customise the callback for every outgoing request. That is done
// within Service.Dial for now.
func clientSideVerifier(tlsCfg *tls.Config, rawCerts [][]byte) error {
	_, err := verifyChain(tlsCfg, rawCerts, true)
	return err
}

// verifyChain performs standard TLS verification without enforcing remote
// hostname matching.
func verifyChain(tlsCfg *tls.Config, rawCerts [][]byte, client bool) (*x509.Certificate, error) {

	// Fetch leaf and intermediates. This is based on code form tls handshake.
	if len(rawCerts) < 1 {
		return nil, errors.New("tls: no certificates from peer")
	}
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, asn1Data := range rawCerts {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return nil, errors.New("tls: failed to parse certificate from peer: " + err.Error())
		}
		certs[i] = cert
	}

	cas := tlsCfg.RootCAs
	if client {
		cas = tlsCfg.ClientCAs
	}

	opts := x509.VerifyOptions{
		Roots:         cas,
		Intermediates: x509.NewCertPool(),
	}
	if !client {
		// Server side only sets KeyUsages in tls. This defaults to ServerAuth in
		// x509 lib. See
		// https://github.com/golang/go/blob/ee7dd810f9ca4e63ecfc1d3044869591783b8b74/src/crypto/x509/verify.go#L866-L868
		opts.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	// All but the first cert are intermediates
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}
	_, err := certs[0].Verify(opts)
	return certs[0], err
}
