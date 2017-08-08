package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// NewTLSConfigFromArgs returns a TLS config based upon the passed
// in list of arguments. Typically these come straight from the
// Corefile.
// no args
//  - creates a Config with no cert and using system CAs
//  - use for a client that talks to a server with a public signed cert (CA installed in system)
//  - the client will not be authenticated by the server since there is no cert
// one arg: the path to CA PEM file
//  - creates a Config with no cert using a specific CA
//  - use for a client that talks to a server with a private signed cert (CA not installed in system)
//  - the client will not be authenticated by the server since there is no cert
// two args: path to cert PEM file, the path to private key PEM file
//  - creates a Config with a cert, using system CAs to validate the other end
//  - use for:
//    - a server; or,
//    - a client that talks to a server with a public cert and needs certificate-based authentication
//  - the other end will authenticate this end via the provided cert
//  - the cert of the other end will be verified via system CAs
// three args: path to cert PEM file, path to client private key PEM file, path to CA PEM file
//  - creates a Config with the cert, using specified CA to validate the other end
//  - use for:
//    - a server; or,
//    - a client that talks to a server with a privately signed cert and needs certificate-based
//      authentication
//  - the other end will authenticate this end via the provided cert
//  - this end will verify the other end's cert using the specified CA
func NewTLSConfigFromArgs(args ...string) (*tls.Config, error) {
	var err error
	var c *tls.Config
	switch len(args) {
	case 0:
		// No client cert, use system CA
		c, err = NewTLSClientConfig("")
	case 1:
		// No client cert, use specified CA
		c, err = NewTLSClientConfig(args[0])
	case 2:
		// Client cert, use system CA
		c, err = NewTLSConfig(args[0], args[1], "")
	case 3:
		// Client cert, use specified CA
		c, err = NewTLSConfig(args[0], args[1], args[2])
	default:
		err = fmt.Errorf("maximum of three arguments allowed for TLS config, found %d", len(args))
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewTLSConfig returns a TLS config that includes a certificate
// Use for server TLS config or when using a client certificate
// If caPath is empty, system CAs will be used
func NewTLSConfig(certPath, keyPath, caPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load TLS cert: %s", err)
	}

	roots, err := loadRoots(caPath)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: roots}, nil
}

// NewTLSClientConfig returns a TLS config for a client connection
// If caPath is empty, system CAs will be used
func NewTLSClientConfig(caPath string) (*tls.Config, error) {
	roots, err := loadRoots(caPath)
	if err != nil {
		return nil, err
	}

	return &tls.Config{RootCAs: roots}, nil
}

func loadRoots(caPath string) (*x509.CertPool, error) {
	if caPath == "" {
		return nil, nil
	}

	roots := x509.NewCertPool()
	pem, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", caPath, err)
	}
	ok := roots.AppendCertsFromPEM(pem)
	if !ok {
		return nil, fmt.Errorf("could not read root certs: %s", err)
	}
	return roots, nil
}

// NewHTTPSTransport returns an HTTP transport configured using tls.Config
func NewHTTPSTransport(cc *tls.Config) *http.Transport {
	// this seems like a bad idea but was here in the previous version
	if cc != nil {
		cc.InsecureSkipVerify = true
	}

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cc,
		MaxIdleConnsPerHost: 25,
	}

	return tr
}
