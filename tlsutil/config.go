package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
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
	"":      tls.VersionTLS10, // default in golang
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

	AutoEncryptTLS bool
}

// KeyPair is used to open and parse a certificate and key file
func (c *Config) KeyPair() (*tls.Certificate, error) {
	return loadKeyPair(c.CertFile, c.KeyFile)
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

// Configurator holds a Config and is responsible for generating all the
// *tls.Config necessary for Consul. Except the one in the api package.
type Configurator struct {
	sync.RWMutex
	base    *Config
	cert    *tls.Certificate
	cas     *x509.CertPool
	logger  *log.Logger
	version int
}

// NewConfigurator creates a new Configurator and sets the provided
// configuration.
func NewConfigurator(config Config, logger *log.Logger) (*Configurator, error) {
	c := &Configurator{logger: logger}
	err := c.Update(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Update updates the internal configuration which is used to generate
// *tls.Config.
// This function acquires a write lock because it writes the new config.
func (c *Configurator) Update(config Config) error {
	cert, err := loadKeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return err
	}
	cas, err := loadCAs(config.CAFile, config.CAPath)
	if err != nil {
		return err
	}

	if err = c.check(config, cas, cert); err != nil {
		return err
	}
	c.Lock()
	c.base = &config
	c.cert = cert
	c.cas = cas
	c.version++
	c.Unlock()
	c.log("Update")
	return nil
}

func (c *Configurator) check(config Config, cas *x509.CertPool, cert *tls.Certificate) error {
	// Check if a minimum TLS version was set
	if config.TLSMinVersion != "" {
		if _, ok := TLSLookup[config.TLSMinVersion]; !ok {
			return fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [tls10,tls11,tls12]", config.TLSMinVersion)
		}
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if config.VerifyOutgoing && cas == nil {
		return fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Ensure we have a CA and cert if VerifyIncoming is set
	if config.VerifyIncoming || config.VerifyIncomingRPC || config.VerifyIncomingHTTPS {
		if cas == nil {
			return fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if cert == nil {
			return fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}
	}
	return nil
}

func loadKeyPair(certFile, keyFile string) (*tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}
	return &cert, nil
}

func loadCAs(caFile, caPath string) (*x509.CertPool, error) {
	if caFile != "" {
		return rootcerts.LoadCAFile(caFile)
	} else if caPath != "" {
		pool, err := rootcerts.LoadCAPath(caPath)
		if err != nil {
			return nil, err
		}
		// make sure to not return an empty pool because this is not
		// the users intention when providing a path for CAs.
		if len(pool.Subjects()) == 0 {
			return nil, fmt.Errorf("Error loading CA: path %q has no CAs", caPath)
		}
		return pool, nil
	}
	return nil, nil
}

// commonTLSConfig generates a *tls.Config from the base configuration the
// Configurator has. It accepts an additional flag in case a config is needed
// for incoming TLS connections.
// This function acquires a read lock because it reads from the config.
func (c *Configurator) commonTLSConfig(additionalVerifyIncomingFlag bool) *tls.Config {
	c.RLock()
	defer c.RUnlock()
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !c.base.VerifyServerHostname,
	}

	// Set the cipher suites
	if len(c.base.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.base.CipherSuites
	}

	tlsConfig.PreferServerCipherSuites = c.base.PreferServerCipherSuites

	tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return c.cert, nil
	}
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return c.cert, nil
	}

	tlsConfig.ClientCAs = c.cas
	tlsConfig.RootCAs = c.cas

	// This is possible because TLSLookup also contains "" with golang's
	// default (tls10). And because the initial check makes sure the
	// version correctly matches.
	tlsConfig.MinVersion = TLSLookup[c.base.TLSMinVersion]

	// Set ClientAuth if necessary
	if c.base.VerifyIncoming || additionalVerifyIncomingFlag {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) outgoingRPCTLSDisabled() bool {
	c.RLock()
	defer c.RUnlock()
	return c.cas == nil && !c.base.VerifyOutgoing && !c.base.AutoEncryptTLS
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) someValuesFromConfig() (bool, bool, string) {
	c.RLock()
	defer c.RUnlock()
	return c.base.VerifyServerHostname, c.base.VerifyOutgoing, c.base.Domain
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyIncomingRPC() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.VerifyIncomingRPC
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyIncomingHTTPS() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.VerifyIncomingHTTPS
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) enableAgentTLSForChecks() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.EnableAgentTLSForChecks
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) serverNameOrNodeName() string {
	c.RLock()
	defer c.RUnlock()
	if c.base.ServerName != "" {
		return c.base.ServerName
	}
	return c.base.NodeName
}

// IncomingRPCConfig generates a *tls.Config for incoming RPC connections.
func (c *Configurator) IncomingRPCConfig() *tls.Config {
	c.log("IncomingRPCConfig")
	config := c.commonTLSConfig(c.verifyIncomingRPC())
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingRPCConfig(), nil
	}
	return config
}

// IncomingNoClientCertRPCConfig generates a *tls.Config for incoming RPC connections.
func (c *Configurator) IncomingAnyCertRPCConfig() *tls.Config {
	c.log("IncomingRPCConfig")
	config := c.commonTLSConfig(false)
	config.ClientAuth = tls.RequireAnyClientCert
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingRPCConfig(), nil
	}
	return config
}

// IncomingHTTPSConfig generates a *tls.Config for incoming HTTPS connections.
func (c *Configurator) IncomingHTTPSConfig() *tls.Config {
	c.log("IncomingHTTPSConfig")
	config := c.commonTLSConfig(c.verifyIncomingHTTPS())
	config.NextProtos = []string{"h2", "http/1.1"}
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingHTTPSConfig(), nil
	}
	return config
}

// IncomingTLSConfig generates a *tls.Config for outgoing TLS connections for
// checks. This function is separated because there is an extra flag to
// consider for checks. EnableAgentTLSForChecks and InsecureSkipVerify has to
// be checked for checks.
func (c *Configurator) OutgoingTLSConfigForCheck(skipVerify bool) *tls.Config {
	c.log("OutgoingTLSConfigForCheck")
	if !c.enableAgentTLSForChecks() {
		return &tls.Config{
			InsecureSkipVerify: skipVerify,
		}
	}

	config := c.commonTLSConfig(false)
	config.InsecureSkipVerify = skipVerify
	config.ServerName = c.serverNameOrNodeName()

	return config
}

// OutgoingRPCConfig generates a *tls.Config for outgoing RPC connections. If
// there is a CA or VerifyOutgoing is set, a *tls.Config will be provided,
// otherwise we assume that no TLS should be used.
func (c *Configurator) OutgoingRPCConfig() *tls.Config {
	c.log("OutgoingRPCConfig")
	if c.outgoingRPCTLSDisabled() {
		return nil
	}
	return c.commonTLSConfig(false)
}

// OutgoingRPCWrapper wraps the result of OutgoingRPCConfig in a DCWrapper. It
// decides if verify server hostname should be used.
func (c *Configurator) OutgoingRPCWrapper() DCWrapper {
	c.log("OutgoingRPCWrapper")
	if c.outgoingRPCTLSDisabled() {
		return nil
	}

	// Generate the wrapper based on dc
	return func(dc string, conn net.Conn) (net.Conn, error) {
		return c.wrapTLSClient(dc, conn)
	}
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) log(name string) {
	if c.logger != nil {
		c.RLock()
		defer c.RUnlock()
		c.logger.Printf("[DEBUG] tlsutil: %s with version %d", name, c.version)
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
func (c *Configurator) wrapTLSClient(dc string, conn net.Conn) (net.Conn, error) {
	var err error
	var tlsConn *tls.Conn

	config := c.OutgoingRPCConfig()
	verifyServerHostname, verifyOutgoing, domain := c.someValuesFromConfig()

	if verifyServerHostname {
		// Strip the trailing '.' from the domain if any
		domain = strings.TrimSuffix(domain, ".")
		config.ServerName = "server." + dc + "." + domain
	}
	tlsConn = tls.Client(conn, config)

	// If crypto/tls is doing verification, there's no need to do
	// our own.
	if !config.InsecureSkipVerify {
		return tlsConn, nil
	}

	// If verification is not turned on, don't do it.
	if !verifyOutgoing {
		return tlsConn, nil
	}

	if err = tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}

	// The following is lightly-modified from the doFullHandshake
	// method in crypto/tls's handshake_client.go.
	opts := x509.VerifyOptions{
		Roots:         config.RootCAs,
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
