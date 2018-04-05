package connect

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"sync"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
)

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

// dynamicTLSConfig represents the state for returning a tls.Config that can
// have root and leaf certificates updated dynamically with all existing clients
// and servers automatically picking up the changes. It requires initialising
// with a valid base config from which all the non-certificate and verification
// params are used. The base config passed should not be modified externally as
// it is assumed to be serialised by the embedded mutex.
type dynamicTLSConfig struct {
	base *tls.Config

	sync.Mutex
	leaf  *tls.Certificate
	roots *x509.CertPool
}

// newDynamicTLSConfig returns a dynamicTLSConfig constructed from base.
// base.Certificates[0] is used as the initial leaf and base.RootCAs is used as
// the initial roots.
func newDynamicTLSConfig(base *tls.Config) *dynamicTLSConfig {
	cfg := &dynamicTLSConfig{
		base: base,
	}
	if len(base.Certificates) > 0 {
		cfg.leaf = &base.Certificates[0]
	}
	if base.RootCAs != nil {
		cfg.roots = base.RootCAs
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
	cfg.Lock()
	defer cfg.Unlock()
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
	return nil
}

// SetLeaf sets a new leaf.
func (cfg *dynamicTLSConfig) SetLeaf(leaf *tls.Certificate) error {
	cfg.Lock()
	defer cfg.Unlock()
	cfg.leaf = leaf
	return nil
}

// Roots returns the current CA root CertPool.
func (cfg *dynamicTLSConfig) Roots() *x509.CertPool {
	cfg.Lock()
	defer cfg.Unlock()
	return cfg.roots
}

// Leaf returns the current Leaf certificate.
func (cfg *dynamicTLSConfig) Leaf() *tls.Certificate {
	cfg.Lock()
	defer cfg.Unlock()
	return cfg.leaf
}
