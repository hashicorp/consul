package connect

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
)

// parseLeafX509Cert will parse an X509 certificate
// from the TLS certificate and store the parsed
// value in the TLS certificate as the Leaf field.
func parseLeafX509Cert(leaf *tls.Certificate) error {
	if leaf == nil {
		// nothing to parse for nil cert
		return nil
	}

	if leaf.Leaf != nil {
		// leaf cert was already parsed
		return nil
	}

	cert, err := x509.ParseCertificate(leaf.Certificate[0])

	if err != nil {
		return err
	}

	leaf.Leaf = cert
	return nil
}

// verifierFunc is a function that can accept rawCertificate bytes from a peer
// and verify them against a given tls.Config. It's called from the
// tls.Config.VerifyPeerCertificate hook.
//
// We don't pass verifiedChains since that is always nil in our usage.
// Implementations can use the roots provided in the cfg to verify the certs.
//
// The passed *tls.Config may have a nil VerifyPeerCertificates function but
// will have correct roots, leaf and other fields.
type verifierFunc func(cfg *tls.Config, rawCerts [][]byte) error

// defaultTLSConfig returns the standard config with no peer verifier. It is
// insecure to use it as-is.
func defaultTLSConfig() *tls.Config {
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
	return cfg
}

// devTLSConfigFromFiles returns a default TLS Config but with certs and CAs
// based on local files for dev. No verification is setup.
func devTLSConfigFromFiles(caFile, certFile,
	keyFile string) (*tls.Config, error) {

	roots := x509.NewCertPool()

	bs, err := os.ReadFile(caFile)
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

// CertURIFromConn is a helper to extract the service identifier URI from a
// net.Conn. If the net.Conn is not a *tls.Conn then an error is always
// returned. If the *tls.Conn didn't present a valid connect certificate, or is
// not yet past the handshake, an error is returned.
func CertURIFromConn(conn net.Conn) (connect.CertURI, error) {
	tc, ok := conn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("invalid non-TLS connect client")
	}
	gotURI, err := extractCertURI(tc.ConnectionState().PeerCertificates)
	if err != nil {
		return nil, err
	}
	return connect.ParseCertURI(gotURI)
}

// extractCertURI returns the first URI SAN from the leaf certificate presented
// in the slice. The slice is expected to be the passed from
// tls.Conn.ConnectionState().PeerCertificates and requires that the leaf has at
// least one URI and the first URI is the correct one to use.
func extractCertURI(certs []*x509.Certificate) (*url.URL, error) {
	if len(certs) < 1 {
		return nil, errors.New("no peer certificate presented")
	}

	// Only check the first cert assuming this is the only leaf. It's not clear if
	// services might ever legitimately present multiple leaf certificates or if
	// the slice is just to allow presenting the whole chain of intermediates.
	cert := certs[0]

	// Our certs will only ever have a single URI for now so only check that
	if len(cert.URIs) < 1 {
		return nil, errors.New("peer certificate invalid")
	}

	return cert.URIs[0], nil
}

// verifyServerCertMatchesURI is used on tls connections dialed to a connect
// server to ensure that the certificate it presented has the correct identity.
func verifyServerCertMatchesURI(certs []*x509.Certificate, expected connect.CertURI) error {
	expectedStr := expected.URI().String()

	gotURI, err := extractCertURI(certs)
	if err != nil {
		return errors.New("peer certificate mismatch")
	}

	// Override the hostname since we rely on x509 constraints to limit ability to
	// spoof the trust domain if needed (i.e. because a root is shared with other
	// PKI or Consul clusters). This allows for seamless migrations between trust
	// domains.
	expectURI := expected.URI()
	expectURI.Host = gotURI.Host
	if strings.EqualFold(gotURI.String(), expectURI.String()) {
		return nil
	}

	return fmt.Errorf("peer certificate mismatch got %s, want %s",
		gotURI.String(), expectedStr)
}

// newServerSideVerifier returns a verifierFunc that wraps the provided
// api.Client to verify the TLS chain and perform AuthZ for the server end of
// the connection. The service name provided is used as the target service name
// for the Authorization.
func newServerSideVerifier(logger hclog.Logger, client *api.Client, serviceName string) verifierFunc {
	return func(tlsCfg *tls.Config, rawCerts [][]byte) error {
		leaf, err := verifyChain(tlsCfg, rawCerts, false)
		if err != nil {
			logger.Error("failed TLS verification", "error", err)
			return err
		}

		// Check leaf is a cert we understand
		if len(leaf.URIs) < 1 {
			logger.Error("invalid leaf certificate: no URIs set")
			return errors.New("connect: invalid leaf certificate")
		}

		certURI, err := connect.ParseCertURI(leaf.URIs[0])
		if err != nil {
			logger.Error("invalid leaf certificate URI", "error", err)
			return errors.New("connect: invalid leaf certificate URI")
		}

		// No AuthZ if there is no client.
		if client == nil {
			logger.Info("nil client provided")
			return nil
		}

		// Perform AuthZ
		req := &api.AgentAuthorizeParams{
			Target:           serviceName,
			ClientCertURI:    certURI.URI().String(),
			ClientCertSerial: connect.EncodeSerialNumber(leaf.SerialNumber),
		}
		resp, err := client.Agent().ConnectAuthorize(req)
		if err != nil {
			logger.Error("authz call failed", "error", err)
			return errors.New("connect: authz call failed: " + err.Error())
		}
		if !resp.Authorized {
			logger.Error("authz call denied", "reason", resp.Reason)
			return errors.New("connect: authz denied: " + resp.Reason)
		}
		return nil
	}
}

// clientSideVerifier is a verifierFunc that performs verification of certificates
// on the client end of the connection. For now it is just basic TLS
// verification since the identity check needs additional state and becomes
// clunky to customize the callback for every outgoing request. That is done
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

// dynamicTLSConfig represents the state for returning a tls.Config that can
// have root and leaf certificates updated dynamically with all existing clients
// and servers automatically picking up the changes. It requires initializing
// with a valid base config from which all the non-certificate and verification
// params are used. The base config passed should not be modified externally as
// it is assumed to be serialized by the embedded mutex.
type dynamicTLSConfig struct {
	base *tls.Config

	sync.RWMutex
	leaf  *tls.Certificate
	roots *x509.CertPool
	// readyCh is closed when the config first gets both leaf and roots set.
	// Watchers can wait on this via ReadyWait.
	readyCh chan struct{}
}

type tlsCfgUpdate struct {
	ch   chan struct{}
	next *tlsCfgUpdate
}

// newDynamicTLSConfig returns a dynamicTLSConfig constructed from base.
// base.Certificates[0] is used as the initial leaf and base.RootCAs is used as
// the initial roots.
func newDynamicTLSConfig(base *tls.Config, logger hclog.Logger) *dynamicTLSConfig {
	cfg := &dynamicTLSConfig{
		base: base,
	}
	if len(base.Certificates) > 0 {
		cfg.leaf = &base.Certificates[0]
		// If this does error then future calls to Ready will fail
		// It is better to handle not-Ready rather than failing
		if err := parseLeafX509Cert(cfg.leaf); err != nil && logger != nil {
			logger.Error("error parsing configured leaf certificate", "error", err)
		}
	}
	if base.RootCAs != nil {
		cfg.roots = base.RootCAs
	}
	if !cfg.Ready() {
		cfg.readyCh = make(chan struct{})
	}
	return cfg
}

// Get fetches the lastest tls.Config with all the hooks attached to keep it
// loading the most recent roots and certs even after future changes to cfg.
//
// The verifierFunc passed will be attached to the config returned such that it
// runs with the _latest_ config object returned passed to it. That means that a
// client can use this config for a long time and will still verify against the
// latest roots even though the roots in the struct is has can't change.
func (cfg *dynamicTLSConfig) Get(v verifierFunc) *tls.Config {
	cfg.RLock()
	defer cfg.RUnlock()
	copy := cfg.base.Clone()
	copy.RootCAs = cfg.roots
	copy.ClientCAs = cfg.roots
	if v != nil {
		copy.VerifyPeerCertificate = func(rawCerts [][]byte, chains [][]*x509.Certificate) error {
			return v(cfg.Get(nil), rawCerts)
		}
	}
	copy.GetCertificate = func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
		leaf := cfg.Leaf()
		if leaf == nil {
			return nil, errors.New("tls: no certificates configured")
		}
		return leaf, nil
	}
	copy.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		leaf := cfg.Leaf()
		if leaf == nil {
			return nil, errors.New("tls: no certificates configured")
		}
		return leaf, nil
	}
	copy.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return cfg.Get(v), nil
	}
	return copy
}

// SetRoots sets new roots.
func (cfg *dynamicTLSConfig) SetRoots(roots *x509.CertPool) error {
	cfg.Lock()
	defer cfg.Unlock()
	cfg.roots = roots
	cfg.notify()
	return nil
}

// SetLeaf sets a new leaf.
func (cfg *dynamicTLSConfig) SetLeaf(leaf *tls.Certificate) error {
	cfg.Lock()
	defer cfg.Unlock()
	if err := parseLeafX509Cert(leaf); err != nil {
		return err
	}
	cfg.leaf = leaf

	cfg.notify()
	return nil
}

// notify is called under lock during an update to check if we are now ready.
func (cfg *dynamicTLSConfig) notify() {
	if cfg.readyCh != nil && cfg.leaf != nil && cfg.roots != nil && cfg.leaf.Leaf != nil {
		close(cfg.readyCh)
		cfg.readyCh = nil
	}
}

func (cfg *dynamicTLSConfig) VerifyLeafWithRoots() error {
	cfg.RLock()
	defer cfg.RUnlock()

	if cfg.roots == nil {
		return fmt.Errorf("No roots are set")
	} else if cfg.leaf == nil {
		return fmt.Errorf("No leaf certificate is set")
	} else if cfg.leaf.Leaf == nil {
		return fmt.Errorf("Leaf certificate has not been parsed")
	}

	_, err := cfg.leaf.Leaf.Verify(x509.VerifyOptions{Roots: cfg.roots})
	return err
}

// Roots returns the current CA root CertPool.
func (cfg *dynamicTLSConfig) Roots() *x509.CertPool {
	cfg.RLock()
	defer cfg.RUnlock()
	return cfg.roots
}

// Leaf returns the current Leaf certificate.
func (cfg *dynamicTLSConfig) Leaf() *tls.Certificate {
	cfg.RLock()
	defer cfg.RUnlock()
	return cfg.leaf
}

// Ready returns whether or not both roots and a leaf certificate are
// configured. If both are non-nil, they are assumed to be valid and usable.
func (cfg *dynamicTLSConfig) Ready() bool {
	// not locking because VerifyLeafWithRoots will do that
	return cfg.VerifyLeafWithRoots() == nil
}

// ReadyWait returns a chan that is closed when the the Service becomes ready
// for use for the first time. Note that if the Service is ready when it is
// called it returns a nil chan. Ready means that it has root and leaf
// certificates configured but not that the combination is valid nor that
// the current time is within the validity window of the certificate. The
// service may subsequently stop being "ready" if it's certificates expire
// or are revoked and an error prevents new ones from being loaded but this
// method will not stop returning a nil chan in that case. It is only useful
// for initial startup. For ongoing health Ready() should be used.
func (cfg *dynamicTLSConfig) ReadyWait() <-chan struct{} {
	cfg.RLock()
	defer cfg.RUnlock()
	return cfg.readyCh
}
