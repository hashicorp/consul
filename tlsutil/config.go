package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"sync"
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
	// VerifyIncoming is used to verify the authenticity of incoming
	// connections.  This means that TCP requests are forbidden, only
	// allowing for TLS. TLS connections must match a provided certificate
	// authority. This can be used to force client auth.
	VerifyIncoming bool

	// VerifyIncomingRPC is used to verify the authenticity of incoming RPC
	// connections.  This means that TCP requests are forbidden, only
	// allowing for TLS. TLS connections must match a provided certificate
	// authority. This can be used to force client auth.
	VerifyIncomingRPC bool

	// VerifyIncomingHTTPS is used to verify the authenticity of incoming
	// HTTPS connections.  This means that TCP requests are forbidden, only
	// allowing for TLS. TLS connections must match a provided certificate
	// authority. This can be used to force client auth.
	VerifyIncomingHTTPS bool

	// VerifyOutgoing is used to verify the authenticity of outgoing
	// connections.  This means that TLS requests are used, and TCP
	// requests are not made. TLS connections must match a provided
	// certificate authority. This is used to verify authenticity of server
	// nodes.
	VerifyOutgoing bool

	// VerifyServerHostname is used to enable hostname verification of
	// servers. This ensures that the certificate presented is valid for
	// server.<datacenter>.<domain>.  This prevents a compromised client
	// from being restarted as a server, and then intercepting request
	// traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we
	// cannot break existing clients.
	VerifyServerHostname bool

	// UseTLS is used to enable outgoing TLS connections to Consul servers.
	UseTLS bool

	// CAFile is a path to a certificate authority file. This is used with
	// VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	CAFile string

	// CAPath is a path to a directory containing certificate authority
	// files. This is used with VerifyIncoming or VerifyOutgoing to verify
	// the TLS connection.
	CAPath string

	// CertFile is used to provide a TLS certificate that is used for
	// serving TLS connections.  Must be provided to serve TLS connections.
	CertFile string

	// KeyFile is used to provide a TLS key that is used for serving TLS
	// connections.  Must be provided to serve TLS connections.
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

	// PreferServerCipherSuites specifies whether to prefer the server's
	// ciphersuite over the client ciphersuites.
	PreferServerCipherSuites bool

	// EnableAgentTLSForChecks is used to apply the agent's TLS settings in
	// order to configure the HTTP client used for health checks. Enabling
	// this allows HTTP checks to present a client certificate and verify
	// the server using the same TLS configuration as the agent (CA, cert,
	// and key).
	EnableAgentTLSForChecks bool
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
func (c *Config) wrapTLSClient(conn net.Conn, tlsConfig *tls.Config) (net.Conn, error) {
	var err error
	var tlsConn *tls.Conn

	tlsConn = tls.Client(conn, tlsConfig)

	// If crypto/tls is doing verification, there's no need to do
	// our own.
	if tlsConfig.InsecureSkipVerify == false {
		return tlsConn, nil
	}

	// If verification is not turned on, don't do it.
	if !c.VerifyOutgoing {
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

// Configurator holds a Config and is responsible for generating all the
// *tls.Config necessary for Consul. Except the one in the api package.
type Configurator struct {
	sync.Mutex
	base   *Config
	checks map[string]bool
}

// NewConfigurator creates a new Configurator and sets the provided
// configuration.
// Todo (Hans): should config be a value instead a pointer to avoid side
// effects?
func NewConfigurator(config *Config) *Configurator {
	return &Configurator{base: config, checks: map[string]bool{}}
}

// Update updates the internal configuration which is used to generate
// *tls.Config.
func (c *Configurator) Update(config *Config) {
	c.Lock()
	defer c.Unlock()
	c.base = config
}

// commonTLSConfig generates a *tls.Config from the base configuration the
// Configurator has. It accepts an additional flag in case a config is needed
// for incoming TLS connections.
func (c *Configurator) commonTLSConfig(additionalVerifyIncomingFlag bool) (*tls.Config, error) {
	if c.base == nil {
		return nil, fmt.Errorf("No base config")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.base.VerifyServerHostname,
	}

	// Set the cipher suites
	if len(c.base.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.base.CipherSuites
	}
	if c.base.PreferServerCipherSuites {
		tlsConfig.PreferServerCipherSuites = true
	}

	// Add cert/key
	cert, err := c.base.KeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	// Check if a minimum TLS version was set
	if c.base.TLSMinVersion != "" {
		tlsvers, ok := TLSLookup[c.base.TLSMinVersion]
		if !ok {
			return nil, fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [tls10,tls11,tls12]", c.base.TLSMinVersion)
		}
		tlsConfig.MinVersion = tlsvers
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if c.base.VerifyOutgoing && c.base.CAFile == "" && c.base.CAPath == "" {
		return nil, fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Parse the CA certs if any
	if c.base.CAFile != "" {
		pool, err := rootcerts.LoadCAFile(c.base.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.RootCAs = pool
	} else if c.base.CAPath != "" {
		pool, err := rootcerts.LoadCAPath(c.base.CAPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.RootCAs = pool
	}

	// Set ClientAuth if necessary
	if c.base.VerifyIncoming || additionalVerifyIncomingFlag {
		if c.base.CAFile == "" && c.base.CAPath == "" {
			return nil, fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if len(tlsConfig.Certificates) == 0 {
			return nil, fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

// IncomingRPCConfig generates a *tls.Config for incoming RPC connections.
func (c *Configurator) IncomingRPCConfig() (*tls.Config, error) {
	return c.commonTLSConfig(c.base.VerifyIncomingRPC)
}

// IncomingHTTPSConfig generates a *tls.Config for incoming HTTPS connections.
func (c *Configurator) IncomingHTTPSConfig() (*tls.Config, error) {
	return c.commonTLSConfig(c.base.VerifyIncomingHTTPS)
}

// IncomingTLSConfig generates a *tls.Config for outgoing TLS connections for
// checks. This function is seperated because there is an extra flag to
// consider for checks. EnableAgentTLSForChecks and InsecureSkipVerify has to
// be checked for checks.
func (c *Configurator) OutgoingTLSConfigForCheck(id string) (*tls.Config, error) {
	if !c.base.EnableAgentTLSForChecks {
		return &tls.Config{
			InsecureSkipVerify: c.getSkipVerifyForCheck(id),
		}, nil
	}

	tlsConfig, err := c.commonTLSConfig(false)
	if err != nil {
		return nil, err
	}
	tlsConfig.InsecureSkipVerify = c.getSkipVerifyForCheck(id)
	tlsConfig.ServerName = c.base.ServerName
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = c.base.NodeName
	}

	return tlsConfig, nil
}

// OutgoingRPCConfig generates a *tls.Config for outgoing RPC connections. If
// there is a CA or VerifyOutgoing is set, a *tls.Config will be provided,
// otherwise we assume that no TLS should be used.
func (c *Configurator) OutgoingRPCConfig() (*tls.Config, error) {
	useTLS := c.base.CAFile != "" || c.base.CAPath != "" || c.base.VerifyOutgoing
	if !useTLS {
		return nil, nil
	}
	return c.commonTLSConfig(false)
}

// OutgoingRPCWrapper wraps the result of OutgoingRPCConfig in a DCWrapper. It
// decides if verify server hostname should be used.
func (c *Configurator) OutgoingRPCWrapper() (DCWrapper, error) {
	// Get the TLS config
	tlsConfig, err := c.OutgoingRPCConfig()
	if err != nil {
		return nil, err
	}

	// Check if TLS is not enabled
	if tlsConfig == nil {
		return nil, nil
	}

	// Generate the wrapper based on hostname verification
	wrapper := func(dc string, conn net.Conn) (net.Conn, error) {
		if c.base.VerifyServerHostname {
			// Strip the trailing '.' from the domain if any
			domain := strings.TrimSuffix(c.base.Domain, ".")
			tlsConfig = tlsConfig.Clone()
			tlsConfig.ServerName = "server." + dc + "." + domain
		}
		return c.base.wrapTLSClient(conn, tlsConfig)
	}

	return wrapper, nil
}

// AddCheck adds a check to the internal check map together with the skipVerify
// value, which is used when generating a *tls.Config for this check.
func (c *Configurator) AddCheck(id string, skipVerify bool) {
	c.Lock()
	defer c.Unlock()
	c.checks[id] = skipVerify
}

// RemoveCheck removes a check from the internal check map.
func (c *Configurator) RemoveCheck(id string) {
	c.Lock()
	defer c.Unlock()
	delete(c.checks, id)
}

func (c *Configurator) getSkipVerifyForCheck(id string) bool {
	c.Lock()
	defer c.Unlock()
	return c.checks[id]
}

// ParseCiphers parse ciphersuites from the comma-separated string into
// recognized slice
func ParseCiphers(cipherStr string) ([]uint16, error) {
	suites := []uint16{}

	cipherStr = strings.TrimSpace(cipherStr)
	if cipherStr == "" {
		return []uint16{}, nil
	}
	ciphers := strings.Split(cipherStr, ",")

	cipherMap := map[string]uint16{
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
		"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
		"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
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
