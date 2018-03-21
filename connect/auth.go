package connect

import "crypto/x509"

// Auther is the interface that provides both Authentication and Authorization
// for an mTLS connection. It's only method is compatible with
// tls.Config.VerifyPeerCertificate.
type Auther interface {
	// Auth is called during tls Connection establishment to Authenticate and
	// Authorize the presented peer. Note that verifiedChains must not be relied
	// upon as we typically have to skip Go's internal verification so the
	// implementation takes full responsibility to validating the certificate
	// against known roots. It is also up to the user of the interface to ensure
	// appropriate validation is performed for client or server end by arranging
	// for an appropriate implementation to be hooked into the tls.Config used.
	Auth(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
}

// ClientAuther is used to auth Clients connecting to a Server.
type ClientAuther struct{}

// Auth implements Auther
func (a *ClientAuther) Auth(rawCerts [][]byte,
	verifiedChains [][]*x509.Certificate) error {

	// TODO(banks): implement path validation and AuthZ
	return nil
}

// ServerAuther is used to auth the Server identify from a connecting Client.
type ServerAuther struct {
	// TODO(banks): We'll need a way to pass the expected service identity (name,
	// namespace, dc, cluster) here based on discovery result.
}

// Auth implements Auther
func (a *ServerAuther) Auth(rawCerts [][]byte,
	verifiedChains [][]*x509.Certificate) error {

	// TODO(banks): implement path validation and verify URI matches the target
	// service we intended to connect to.
	return nil
}
