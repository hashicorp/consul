package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/go-rootcerts"
)

// DCWrapper is a function that is used to wrap a non-TLS connection
// and returns an appropriate TLS connection or error. This takes
// a datacenter as an argument.
type DCWrapper func(dc string, conn net.Conn) (net.Conn, error)

// Wrapper is a variant of DCWrapper, where the DC is provided as
// a constant value. This is usually done by currying DCWrapper.
type Wrapper func(conn net.Conn) (net.Conn, error)

// TLSLookup maps the tls_min_version configuration to the internal value
var TLSLookup = map[string]uint16{
	"tls10": tls.VersionTLS10,
	"tls11": tls.VersionTLS11,
	"tls12": tls.VersionTLS12,
}

// Config used to create tls.Config
type Config struct {
	// VerifyIncoming is used to verify the authenticity of incoming connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncoming bool

	// VerifyOutgoing is used to verify the authenticity of outgoing connections.
	// This means that TLS requests are used, and TCP requests are not made. TLS connections
	// must match a provided certificate authority. This is used to verify authenticity of
	// server nodes.
	VerifyOutgoing bool

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<datacenter>.<domain>.
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool

	// UseTLS is used to enable outgoing TLS connections to Consul servers.
	UseTLS bool

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string

	// CAPath is a path to a directory containing certificate authority files. This is used
	// with VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	CAPath string

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// ServerName is used with the TLS certificate to ensure the name we
	// provide matches the certificate
	ServerName string

	// Domain is the Consul TLD being used. Defaults to "consul."
	Domain string

	// TLSMinVersion is the minimum accepted TLS version that can be used.
	TLSMinVersion string

	// CipherSuites is the list of TLS cipher suites to use.
	CipherSuites []uint16

	// PreferServerCipherSuites specifies whether to prefer the server's ciphersuite
	// over the client ciphersuites.
	PreferServerCipherSuites bool
}

// AppendCA opens and parses the CA file and adds the certificates to
// the provided CertPool.
func (c *Config) AppendCA(pool *x509.CertPool) error {
	if c.CAFile == "" {
		return nil
	}

	// Read the file
	data, err := ioutil.ReadFile(c.CAFile)
	if err != nil {
		return fmt.Errorf("Failed to read CA file: %v", err)
	}

	if !pool.AppendCertsFromPEM(data) {
		return fmt.Errorf("Failed to parse any CA certificates")
	}

	return nil
}

// KeyPair is used to open and parse a certificate and key file
func (c *Config) KeyPair() (*tls.Certificate, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}
	return &cert, err
}

// OutgoingTLSConfig generates a TLS configuration for outgoing
// requests. It will return a nil config if this configuration should
// not use TLS for outgoing connections.
func (c *Config) OutgoingTLSConfig() (*tls.Config, error) {
	// If VerifyServerHostname is true, that implies VerifyOutgoing
	if c.VerifyServerHostname {
		c.VerifyOutgoing = true
	}
	if !c.UseTLS && !c.VerifyOutgoing {
		return nil, nil
	}
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		RootCAs:            x509.NewCertPool(),
		InsecureSkipVerify: true,
	}
	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
		tlsConfig.InsecureSkipVerify = false
	}
	if c.VerifyServerHostname {
		// ServerName is filled in dynamically based on the target DC
		tlsConfig.ServerName = "VerifyServerHostname"
		tlsConfig.InsecureSkipVerify = false
	}
	if len(c.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.CipherSuites
	}
	if c.PreferServerCipherSuites {
		tlsConfig.PreferServerCipherSuites = true
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if c.VerifyOutgoing && c.CAFile == "" && c.CAPath == "" {
		return nil, fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Parse the CA certs if any
	rootConfig := &rootcerts.Config{
		CAFile: c.CAFile,
		CAPath: c.CAPath,
	}
	if err := rootcerts.ConfigureTLS(tlsConfig, rootConfig); err != nil {
		return nil, err
	}

	// Add cert/key
	cert, err := c.KeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	// Check if a minimum TLS version was set
	if c.TLSMinVersion != "" {
		tlsvers, ok := TLSLookup[c.TLSMinVersion]
		if !ok {
			return nil, fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [tls10,tls11,tls12]", c.TLSMinVersion)
		}
		tlsConfig.MinVersion = tlsvers
	}

	return tlsConfig, nil
}

// Clone returns a copy of c. Only the exported fields are copied. This
// was copied from https://golang.org/src/crypto/tls/common.go since that
// isn't exported and Go 1.7's vet uncovered an unsafe copy of a mutex in
// here.
//
// TODO (slackpad) - This can be removed once we move to Go 1.8, see
// https://github.com/golang/go/commit/d24f446 for details.
func clone(c *tls.Config) *tls.Config {
	return &tls.Config{
		Rand:                        c.Rand,
		Time:                        c.Time,
		Certificates:                c.Certificates,
		NameToCertificate:           c.NameToCertificate,
		GetCertificate:              c.GetCertificate,
		RootCAs:                     c.RootCAs,
		NextProtos:                  c.NextProtos,
		ServerName:                  c.ServerName,
		ClientAuth:                  c.ClientAuth,
		ClientCAs:                   c.ClientCAs,
		InsecureSkipVerify:          c.InsecureSkipVerify,
		CipherSuites:                c.CipherSuites,
		PreferServerCipherSuites:    c.PreferServerCipherSuites,
		SessionTicketsDisabled:      c.SessionTicketsDisabled,
		SessionTicketKey:            c.SessionTicketKey,
		ClientSessionCache:          c.ClientSessionCache,
		MinVersion:                  c.MinVersion,
		MaxVersion:                  c.MaxVersion,
		CurvePreferences:            c.CurvePreferences,
		DynamicRecordSizingDisabled: c.DynamicRecordSizingDisabled,
		Renegotiation:               c.Renegotiation,
	}
}

// OutgoingTLSWrapper returns a a DCWrapper based on the OutgoingTLS
// configuration. If hostname verification is on, the wrapper
// will properly generate the dynamic server name for verification.
func (c *Config) OutgoingTLSWrapper() (DCWrapper, error) {
	// Get the TLS config
	tlsConfig, err := c.OutgoingTLSConfig()
	if err != nil {
		return nil, err
	}

	// Check if TLS is not enabled
	if tlsConfig == nil {
		return nil, nil
	}

	// Strip the trailing '.' from the domain if any
	domain := strings.TrimSuffix(c.Domain, ".")

	wrapper := func(dc string, c net.Conn) (net.Conn, error) {
		return WrapTLSClient(c, tlsConfig)
	}

	// Generate the wrapper based on hostname verification
	if c.VerifyServerHostname {
		wrapper = func(dc string, conn net.Conn) (net.Conn, error) {
			conf := clone(tlsConfig)
			conf.ServerName = "server." + dc + "." + domain
			return WrapTLSClient(conn, conf)
		}
	}

	return wrapper, nil
}

// SpecificDC is used to invoke a static datacenter
// and turns a DCWrapper into a Wrapper type.
func SpecificDC(dc string, tlsWrap DCWrapper) Wrapper {
	if tlsWrap == nil {
		return nil
	}
	return func(conn net.Conn) (net.Conn, error) {
		return tlsWrap(dc, conn)
	}
}

// Wrap a net.Conn into a client tls connection, performing any
// additional verification as needed.
//
// As of go 1.3, crypto/tls only supports either doing no certificate
// verification, or doing full verification including of the peer's
// DNS name. For consul, we want to validate that the certificate is
// signed by a known CA, but because consul doesn't use DNS names for
// node names, we don't verify the certificate DNS names. Since go 1.3
// no longer supports this mode of operation, we have to do it
// manually.
func WrapTLSClient(conn net.Conn, tlsConfig *tls.Config) (net.Conn, error) {
	var err error
	var tlsConn *tls.Conn

	tlsConn = tls.Client(conn, tlsConfig)

	// If crypto/tls is doing verification, there's no need to do
	// our own.
	if tlsConfig.InsecureSkipVerify == false {
		return tlsConn, nil
	}

	if err = tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}

	// The following is lightly-modified from the doFullHandshake
	// method in crypto/tls's handshake_client.go.
	opts := x509.VerifyOptions{
		Roots:         tlsConfig.RootCAs,
		CurrentTime:   time.Now(),
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	for i, cert := range certs {
		if i == 0 {
			continue
		}
		opts.Intermediates.AddCert(cert)
	}

	_, err = certs[0].Verify(opts)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}

	return tlsConn, err
}

// IncomingTLSConfig generates a TLS configuration for incoming requests
func (c *Config) IncomingTLSConfig() (*tls.Config, error) {
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		ServerName: c.ServerName,
		ClientCAs:  x509.NewCertPool(),
		ClientAuth: tls.NoClientCert,
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = c.NodeName
	}

	// Set the cipher suites
	if len(c.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.CipherSuites
	}
	if c.PreferServerCipherSuites {
		tlsConfig.PreferServerCipherSuites = true
	}

	// Parse the CA certs if any
	if c.CAFile != "" {
		pool, err := rootcerts.LoadCAFile(c.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
	} else if c.CAPath != "" {
		pool, err := rootcerts.LoadCAPath(c.CAPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
	}

	// Add cert/key
	cert, err := c.KeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	// Check if we require verification
	if c.VerifyIncoming {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		if c.CAFile == "" && c.CAPath == "" {
			return nil, fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if cert == nil {
			return nil, fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}
	}

	// Check if a minimum TLS version was set
	if c.TLSMinVersion != "" {
		tlsvers, ok := TLSLookup[c.TLSMinVersion]
		if !ok {
			return nil, fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [tls10,tls11,tls12]", c.TLSMinVersion)
		}
		tlsConfig.MinVersion = tlsvers
	}
	return tlsConfig, nil
}

// ParseCiphers parse ciphersuites from the comma-separated string into recognized slice
func ParseCiphers(cipherStr string) ([]uint16, error) {
	suites := []uint16{}

	cipherStr = strings.TrimSpace(cipherStr)
	if cipherStr == "" {
		return []uint16{}, nil
	}
	ciphers := strings.Split(cipherStr, ",")

	cipherMap := map[string]uint16{
		"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}
	for _, cipher := range ciphers {
		if v, ok := cipherMap[cipher]; ok {
			suites = append(suites, v)
		} else {
			return suites, fmt.Errorf("unsupported cipher %q", cipher)
		}
	}

	return suites, nil
}
