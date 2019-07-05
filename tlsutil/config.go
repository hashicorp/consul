package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

	// AutoEncryptTLS opts the agent into provisioning agent
	// TLS certificates.
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

type autoEncrypt struct {
	manualCAPems         []string
	connectCAPems        []string
	cert                 *tls.Certificate
	verifyServerHostname bool
}

func (a *autoEncrypt) caPems() []string {
	return append(a.manualCAPems, a.connectCAPems...)
}

type manual struct {
	caPems []string
	cert   *tls.Certificate
}

// Configurator holds a Config and is responsible for generating all the
// *tls.Config necessary for Consul. Except the one in the api package.
type Configurator struct {
	sync.RWMutex
	base        *Config
	autoEncrypt *autoEncrypt
	manual      *manual

	caPool  *x509.CertPool
	logger  *log.Logger
	version int
}

// NewConfigurator creates a new Configurator and sets the provided
// configuration.
func NewConfigurator(config Config, logger *log.Logger) (*Configurator, error) {
	c := &Configurator{logger: logger, manual: &manual{}, autoEncrypt: &autoEncrypt{}}
	err := c.Update(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// CAPems returns the currently loaded CAs in PEM format.
func (c *Configurator) CAPems() []string {
	c.RLock()
	defer c.RUnlock()
	return append(c.manual.caPems, c.autoEncrypt.caPems()...)
}

// ManualCAPems returns the currently loaded CAs in PEM format.
func (c *Configurator) ManualCAPems() []string {
	c.RLock()
	defer c.RUnlock()
	return c.manual.caPems
}

// Update updates the internal configuration which is used to generate
// *tls.Config.
// This function acquires a write lock because it writes the new config.
func (c *Configurator) Update(config Config) error {
	c.Lock()
	// order of defers matters because log acquires a RLock()
	defer c.log("Update")
	defer c.Unlock()

	cert, err := loadKeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return err
	}
	pems, err := loadCAs(config.CAFile, config.CAPath)
	if err != nil {
		return err
	}
	pool, err := pool(append(pems, c.autoEncrypt.caPems()...))
	if err != nil {
		return err
	}
	if err = c.check(config, pool, cert); err != nil {
		return err
	}
	c.base = &config
	c.manual.cert = cert
	c.manual.caPems = pems
	c.caPool = pool
	c.version++
	return nil
}

// UpdateAutoEncryptCA updates the autoEncrypt.caPems. This is supposed to be called
// from the server in order to be able to accept TLS connections with TLS
// certificates.
// Or it is being called on the client side when CA changes are detected.
func (c *Configurator) UpdateAutoEncryptCA(connectCAPems []string) error {
	c.Lock()
	// order of defers matters because log acquires a RLock()
	defer c.log("UpdateAutoEncryptCA")
	defer c.Unlock()

	pool, err := pool(append(c.manual.caPems, append(c.autoEncrypt.manualCAPems, connectCAPems...)...))
	if err != nil {
		c.RUnlock()
		return err
	}
	if err = c.check(*c.base, pool, c.manual.cert); err != nil {
		c.RUnlock()
		return err
	}
	c.autoEncrypt.connectCAPems = connectCAPems
	c.caPool = pool
	c.version++
	return nil
}

// UpdateAutoEncryptCert
func (c *Configurator) UpdateAutoEncryptCert(pub, priv string) error {
	// order of defers matters because log acquires a RLock()
	defer c.log("UpdateAutoEncryptCert")
	cert, err := tls.X509KeyPair([]byte(pub), []byte(priv))
	if err != nil {
		return fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	c.Lock()
	defer c.Unlock()

	c.autoEncrypt.cert = &cert
	c.version++
	return nil
}

// UpdateAutoEncrypt sets everything under autoEncrypt. This is being called on the
// client when it received its cert from AutoEncrypt endpoint.
func (c *Configurator) UpdateAutoEncrypt(manualCAPems, connectCAPems []string, pub, priv string, verifyServerHostname bool) error {
	// order of defers matters because log acquires a RLock()
	defer c.log("UpdateAutoEncrypt")
	cert, err := tls.X509KeyPair([]byte(pub), []byte(priv))
	if err != nil {
		return fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	c.Lock()
	defer c.Unlock()

	pool, err := pool(append(c.manual.caPems, append(manualCAPems, connectCAPems...)...))
	if err != nil {
		return err
	}
	c.autoEncrypt.manualCAPems = manualCAPems
	c.autoEncrypt.connectCAPems = connectCAPems
	c.autoEncrypt.cert = &cert
	c.caPool = pool
	c.autoEncrypt.verifyServerHostname = verifyServerHostname
	c.version++
	return nil
}

func (c *Configurator) Base() Config {
	c.RLock()
	defer c.RUnlock()
	return *c.base
}

func pool(pems []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, pem := range pems {
		if !pool.AppendCertsFromPEM([]byte(pem)) {
			return nil, fmt.Errorf("Couldn't parse PEM %s", pem)
		}
	}
	if len(pool.Subjects()) == 0 {
		return nil, nil
	}
	return pool, nil
}

func (c *Configurator) check(config Config, pool *x509.CertPool, cert *tls.Certificate) error {
	// Check if a minimum TLS version was set
	if config.TLSMinVersion != "" {
		if _, ok := TLSLookup[config.TLSMinVersion]; !ok {
			return fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [tls10,tls11,tls12]", config.TLSMinVersion)
		}
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if config.VerifyOutgoing && pool == nil {
		return fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Ensure we have a CA and cert if VerifyIncoming is set
	if config.anyVerifyIncoming() {
		if pool == nil {
			return fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if cert == nil || cert.Certificate == nil {
			return fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}
	}
	return nil
}

func (c Config) anyVerifyIncoming() bool {
	return c.baseVerifyIncoming() || c.VerifyIncomingRPC || c.VerifyIncomingHTTPS
}

func (c Config) verifyIncomingRPC() bool {
	return c.baseVerifyIncoming() || c.VerifyIncomingRPC
}

func (c Config) verifyIncomingHTTPS() bool {
	return c.baseVerifyIncoming() || c.VerifyIncomingHTTPS
}

func (c *Config) baseVerifyIncoming() bool {
	return c.VerifyIncoming
}

func loadKeyPair(certFile, keyFile string) (*tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return &tls.Certificate{}, nil
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to load cert/key pair: %v", err)
	}
	return &cert, nil
}

func loadCAs(caFile, caPath string) ([]string, error) {
	if caFile == "" && caPath == "" {
		return nil, nil
	}

	pems := []string{}

	readFn := func(path string) error {
		pem, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Error loading from %s: %s", path, err)
		}
		pems = append(pems, string(pem))
		return nil
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if err := readFn(path); err != nil {
				return err
			}
		}
		return nil
	}

	if caFile != "" {
		err := readFn(caFile)
		if err != nil {
			return pems, err
		}
	} else if caPath != "" {
		err := filepath.Walk(caPath, walkFn)
		if err != nil {
			return pems, err
		}
		if len(pems) == 0 {
			return pems, fmt.Errorf("Error loading from CAPath: no CAs found")
		}
	}
	return pems, nil
}

// commonTLSConfig generates a *tls.Config from the base configuration the
// Configurator has. It accepts an additional flag in case a config is needed
// for incoming TLS connections.
// This function acquires a read lock because it reads from the config.
func (c *Configurator) commonTLSConfig(verifyIncoming bool) *tls.Config {
	// this needs to be outside of RLock because it acquires an RLock itself
	verifyServerHostname := c.VerifyServerHostname()

	c.RLock()
	defer c.RUnlock()
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !verifyServerHostname,
	}

	// Set the cipher suites
	if len(c.base.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.base.CipherSuites
	}

	tlsConfig.PreferServerCipherSuites = c.base.PreferServerCipherSuites

	// GetCertificate is used when acting as a server and responding to
	// client requests. Always return the manually configured cert, because
	// on the server this is all we have. And on the client, this is the
	// only sensitive option.
	tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return c.manual.cert, nil
	}

	// GetClientCertificate is used when acting as a client and responding
	// to a server requesting a certificate. Return the autoEncrypt certificate
	// if possible, otherwise default to the manually provisioned one.
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		cert := c.autoEncrypt.cert
		if cert == nil {
			cert = c.manual.cert
		}

		return cert, nil
	}

	tlsConfig.ClientCAs = c.caPool
	tlsConfig.RootCAs = c.caPool

	// This is possible because TLSLookup also contains "" with golang's
	// default (tls10). And because the initial check makes sure the
	// version correctly matches.
	tlsConfig.MinVersion = TLSLookup[c.base.TLSMinVersion]

	// Set ClientAuth if necessary
	if verifyIncoming {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) VerifyIncomingRPC() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.verifyIncomingRPC()
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) outgoingRPCTLSDisabled() bool {
	c.RLock()
	defer c.RUnlock()

	// if AutoEncrypt enabled, always use TLS
	if c.base.AutoEncryptTLS {
		return false
	}

	// if CAs are provided or VerifyOutgoing is set, use TLS
	if c.caPool != nil || c.base.VerifyOutgoing {
		return false
	}

	return true
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyOutgoing() bool {
	c.RLock()
	defer c.RUnlock()

	// If AutoEncryptTLS is enabled and there is a CA, then verify
	// outgoing.
	if c.base.AutoEncryptTLS && c.caPool != nil {
		return true
	}

	return c.base.VerifyOutgoing
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) domain() string {
	c.RLock()
	defer c.RUnlock()
	return c.base.Domain
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyIncomingRPC() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.verifyIncomingRPC()
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyIncomingHTTPS() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.verifyIncomingHTTPS()
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

// This function acquires a read lock because it reads from the config.
func (c *Configurator) VerifyServerHostname() bool {
	c.RLock()
	defer c.RUnlock()
	return c.base.VerifyServerHostname || c.autoEncrypt.verifyServerHostname
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

// IncomingInsecureRPCConfig means that it doesn't verify incoming even thought
// it might have been configured. This is only supposed to be used by the
// servers for the insecure RPC server. At the time of writing only the
// AutoEncrypt.Sign call is supported on that server. And it might be the only
// usecase ever.
func (c *Configurator) IncomingInsecureRPCConfig() *tls.Config {
	c.log("IncomingInsecureRPCConfig")
	config := c.commonTLSConfig(false)
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingInsecureRPCConfig(), nil
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

// AutoEncryptCertNotAfter returns NotAfter from the auto_encrypt cert. In case
// there is no cert, it will return a time in the past.
func (c *Configurator) AutoEncryptCertNotAfter() time.Time {
	c.RLock()
	tlsCert := c.autoEncrypt.cert
	c.RUnlock()
	if tlsCert == nil {
		return time.Now().AddDate(0, 0, -1)
	}
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return time.Now().AddDate(0, 0, -1)
	}
	return cert.NotAfter
}

// AutoEncryptCertExpired returns if the auto_encrypt cert is expired.
func (c *Configurator) AutoEncryptCertExpired() bool {
	return c.AutoEncryptCertNotAfter().Before(time.Now())
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
	config := c.OutgoingRPCConfig()
	verifyServerHostname := c.VerifyServerHostname()
	verifyOutgoing := c.verifyOutgoing()
	domain := c.domain()

	if verifyServerHostname {
		// Strip the trailing '.' from the domain if any
		domain = strings.TrimSuffix(domain, ".")
		config.ServerName = "server." + dc + "." + domain
	}
	tlsConn := tls.Client(conn, config)

	// If crypto/tls is doing verification, there's no need to do
	// our own.
	if !config.InsecureSkipVerify {
		return tlsConn, nil
	}

	// If verification is not turned on, don't do it.
	if !verifyOutgoing {
		return tlsConn, nil
	}

	err := tlsConn.Handshake()
	if err != nil {
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
