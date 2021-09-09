package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/logging"
)

// ALPNWrapper is a function that is used to wrap a non-TLS connection and
// returns an appropriate TLS connection or error. This taks a datacenter and
// node name as argument to configure the desired SNI value and the desired
// next proto for configuring ALPN.
type ALPNWrapper func(dc, nodeName, alpnProto string, conn net.Conn) (net.Conn, error)

// DCWrapper is a function that is used to wrap a non-TLS connection
// and returns an appropriate TLS connection or error. This takes
// a datacenter as an argument.
type DCWrapper func(dc string, conn net.Conn) (net.Conn, error)

// Wrapper is a variant of DCWrapper, where the DC is provided as
// a constant value. This is usually done by currying DCWrapper.
type Wrapper func(conn net.Conn) (net.Conn, error)

// tlsLookup maps the tls_min_version configuration to the internal value
var tlsLookup = map[string]uint16{
	"":      tls.VersionTLS10, // default in golang
	"tls10": tls.VersionTLS10,
	"tls11": tls.VersionTLS11,
	"tls12": tls.VersionTLS12,
	"tls13": tls.VersionTLS13,
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

	// AutoTLS opts the agent into provisioning agent
	// TLS certificates.
	AutoTLS bool
}

func tlsVersions() []string {
	versions := []string{}
	for v := range tlsLookup {
		if v != "" {
			versions = append(versions, v)
		}
	}
	sort.Strings(versions)
	return versions
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

// autoTLS stores configuration that is received from the auto-encrypt or
// auto-config features.
type autoTLS struct {
	extraCAPems          []string
	connectCAPems        []string
	cert                 *tls.Certificate
	verifyServerHostname bool
}

// manual stores the TLS CA and cert received from Configurator.Update which
// generally comes from the agent configuration.
type manual struct {
	caPems []string
	cert   *tls.Certificate
	// caPool containing only the caPems. This CertPool should be used instead of
	// the Configurator.caPool when only the Agent TLS CA is allowed.
	caPool *x509.CertPool
}

// Configurator provides tls.Config and net.Dial wrappers to enable TLS for
// clients and servers, for both HTTPS and RPC requests.
// Configurator receives an initial TLS configuration from agent configuration,
// and receives updates from config reloads, auto-encrypt, and auto-config.
type Configurator struct {
	// version is increased each time the Configurator is updated. Must be accessed
	// using sync/atomic. Also MUST be the first field in this struct to ensure
	// 64-bit alignment. See https://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	version uint64

	// lock synchronizes access to all fields on this struct except for logger and version.
	lock    sync.RWMutex
	base    *Config
	autoTLS autoTLS
	manual  manual
	caPool  *x509.CertPool
	// peerDatacenterUseTLS is a map of DC name to a bool indicating if the DC
	// uses TLS for RPC requests.
	peerDatacenterUseTLS map[string]bool

	// logger is not protected by a lock. It must never be changed after
	// Configurator is created.
	logger hclog.Logger
}

// NewConfigurator creates a new Configurator and sets the provided
// configuration.
func NewConfigurator(config Config, logger hclog.Logger) (*Configurator, error) {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Level: hclog.Debug,
		})
	}

	c := &Configurator{
		logger:               logger.Named(logging.TLSUtil),
		peerDatacenterUseTLS: map[string]bool{},
	}
	err := c.Update(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ManualCAPems returns the currently loaded CAs in PEM format.
func (c *Configurator) ManualCAPems() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.manual.caPems
}

// Update updates the internal configuration which is used to generate
// *tls.Config.
// This function acquires a write lock because it writes the new config.
func (c *Configurator) Update(config Config) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	cert, err := loadKeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return err
	}
	pems, err := LoadCAs(config.CAFile, config.CAPath)
	if err != nil {
		return err
	}
	caPool, err := newX509CertPool(pems, c.autoTLS.extraCAPems, c.autoTLS.connectCAPems)
	if err != nil {
		return err
	}
	if err = validateConfig(config, caPool, cert); err != nil {
		return err
	}
	manualCAPool, err := newX509CertPool(pems)
	if err != nil {
		return err
	}

	c.base = &config
	c.manual.cert = cert
	c.manual.caPems = pems
	c.manual.caPool = manualCAPool
	c.caPool = caPool
	atomic.AddUint64(&c.version, 1)
	c.log("Update")
	return nil
}

// UpdateAutoTLSCA updates the autoEncrypt.caPems. This is supposed to be called
// from the server in order to be able to accept TLS connections with TLS
// certificates.
// Or it is being called on the client side when CA changes are detected.
func (c *Configurator) UpdateAutoTLSCA(connectCAPems []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	pool, err := newX509CertPool(c.manual.caPems, c.autoTLS.extraCAPems, connectCAPems)
	if err != nil {
		return err
	}
	if err = validateConfig(*c.base, pool, c.manual.cert); err != nil {
		return err
	}
	c.autoTLS.connectCAPems = connectCAPems
	c.caPool = pool
	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAutoTLSCA")
	return nil
}

// UpdateAutoTLSCert receives the updated Auto-Encrypt certificate.
func (c *Configurator) UpdateAutoTLSCert(pub, priv string) error {
	cert, err := tls.X509KeyPair([]byte(pub), []byte(priv))
	if err != nil {
		return fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.autoTLS.cert = &cert
	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAutoTLSCert")
	return nil
}

// UpdateAutoTLS receives updates from Auto-Config, only expected to be called on
// client agents.
func (c *Configurator) UpdateAutoTLS(manualCAPems, connectCAPems []string, pub, priv string, verifyServerHostname bool) error {
	cert, err := tls.X509KeyPair([]byte(pub), []byte(priv))
	if err != nil {
		return fmt.Errorf("Failed to load cert/key pair: %v", err)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	pool, err := newX509CertPool(c.manual.caPems, manualCAPems, connectCAPems)
	if err != nil {
		return err
	}
	c.autoTLS.extraCAPems = manualCAPems
	c.autoTLS.connectCAPems = connectCAPems
	c.autoTLS.cert = &cert
	c.caPool = pool
	c.autoTLS.verifyServerHostname = verifyServerHostname
	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAutoTLS")
	return nil
}

func (c *Configurator) UpdateAreaPeerDatacenterUseTLS(peerDatacenter string, useTLS bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAreaPeerDatacenterUseTLS")
	c.peerDatacenterUseTLS[peerDatacenter] = useTLS
}

func (c *Configurator) getAreaForPeerDatacenterUseTLS(peerDatacenter string) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if v, ok := c.peerDatacenterUseTLS[peerDatacenter]; ok {
		return v
	}
	return true
}

func (c *Configurator) Base() Config {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return *c.base
}

// newX509CertPool loads all the groups of PEM encoded certificates into a
// single x509.CertPool.
//
// The groups argument is a varargs of slices so that callers do not need to
// append slices together. In some cases append can modify the backing array
// of the first slice passed to append, which will often result in hard to
// find bugs. By accepting a varargs of slices we remove the need for the
// caller to append the groups, which should prevent any such bugs.
func newX509CertPool(groups ...[]string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, group := range groups {
		for _, pem := range group {
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return nil, fmt.Errorf("failed to parse PEM %s", pem)
			}
		}
	}
	if len(pool.Subjects()) == 0 {
		return nil, nil
	}
	return pool, nil
}

// validateConfig checks that config is valid and does not conflict with the pool
// or cert.
func validateConfig(config Config, pool *x509.CertPool, cert *tls.Certificate) error {
	// Check if a minimum TLS version was set
	if config.TLSMinVersion != "" {
		if _, ok := tlsLookup[config.TLSMinVersion]; !ok {
			versions := strings.Join(tlsVersions(), ", ")
			return fmt.Errorf("TLSMinVersion: value %s not supported, please specify one of [%s]", config.TLSMinVersion, versions)
		}
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if config.VerifyOutgoing && pool == nil {
		return fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Ensure we have a CA and cert if VerifyIncoming is set
	if config.anyVerifyIncoming() {
		if pool == nil {
			// both auto-config and auto-encrypt require verifying the connection from the client to the server for secure
			// operation. In order to be able to verify the servers certificate we must have some CA certs already provided.
			// Therefore, even though both of those features can push down extra CA certificates which could be used to
			// verify incoming connections, we still must consider it an error if none are provided in the initial configuration
			// as those features cannot be successfully enabled without providing CA certificates to use those features.
			return fmt.Errorf("VerifyIncoming set but no CA certificates were provided")
		}

		// We will use the auto_encrypt/auto_config cert for TLS in the incoming APIs when available. Therefore the check
		// here will ensure that either we enabled one of those two features or a certificate and key were provided manually
		if cert == nil && !config.AutoTLS {
			return fmt.Errorf("VerifyIncoming requires either a Cert and Key pair in the configuration file, or auto_encrypt/auto_config be enabled")
		}
	}
	return nil
}

func (c Config) anyVerifyIncoming() bool {
	return c.VerifyIncoming || c.VerifyIncomingRPC || c.VerifyIncomingHTTPS
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

func LoadCAs(caFile, caPath string) ([]string, error) {
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

	c.lock.RLock()
	defer c.lock.RUnlock()
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !verifyServerHostname,
	}

	// Set the cipher suites
	if len(c.base.CipherSuites) != 0 {
		tlsConfig.CipherSuites = c.base.CipherSuites
	}

	tlsConfig.PreferServerCipherSuites = c.base.PreferServerCipherSuites

	// GetCertificate is used when acting as a server and responding to
	// client requests. Default to the manually configured cert, but allow
	// autoEncrypt cert too so that a client can encrypt incoming
	// connections without having a manual cert configured.
	tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return c.Cert(), nil
	}

	// GetClientCertificate is used when acting as a client and responding
	// to a server requesting a certificate. Return the autoEncrypt certificate
	// if possible, otherwise default to the manually provisioned one.
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		cert := c.autoTLS.cert
		if cert == nil {
			cert = c.manual.cert
		}

		if cert == nil {
			// the return value MUST not be nil but an empty certificate will be
			// treated the same as having no client certificate
			cert = &tls.Certificate{}
		}

		return cert, nil
	}

	tlsConfig.ClientCAs = c.caPool
	tlsConfig.RootCAs = c.caPool

	// This is possible because tlsLookup also contains "" with golang's
	// default (tls10). And because the initial check makes sure the
	// version correctly matches.
	tlsConfig.MinVersion = tlsLookup[c.base.TLSMinVersion]

	// Set ClientAuth if necessary
	if verifyIncoming {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) Cert() *tls.Certificate {
	c.lock.RLock()
	defer c.lock.RUnlock()
	cert := c.manual.cert
	if cert == nil {
		cert = c.autoTLS.cert
	}
	return cert
}

// VerifyIncomingRPC returns true if the configuration has enabled either
// VerifyIncoming, or VerifyIncomingRPC
func (c *Configurator) VerifyIncomingRPC() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.base.VerifyIncoming || c.base.VerifyIncomingRPC
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) outgoingRPCTLSEnabled() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// use TLS if AutoEncrypt or VerifyOutgoing are enabled.
	return c.base.AutoTLS || c.base.VerifyOutgoing
}

// MutualTLSCapable returns true if Configurator has a CA and a local TLS
// certificate configured.
func (c *Configurator) MutualTLSCapable() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.caPool != nil && (c.autoTLS.cert != nil || c.manual.cert != nil)
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyOutgoing() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// If AutoEncryptTLS is enabled and there is a CA, then verify
	// outgoing.
	if c.base.AutoTLS && c.caPool != nil {
		return true
	}

	return c.base.VerifyOutgoing
}

func (c *Configurator) ServerSNI(dc, nodeName string) string {
	// Strip the trailing '.' from the domain if any
	domain := strings.TrimSuffix(c.domain(), ".")

	if nodeName == "" || nodeName == "*" {
		return "server." + dc + "." + domain
	}

	return nodeName + ".server." + dc + "." + domain
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) domain() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.base.Domain
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) serverNameOrNodeName() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if c.base.ServerName != "" {
		return c.base.ServerName
	}
	return c.base.NodeName
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) VerifyServerHostname() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.base.VerifyServerHostname || c.autoTLS.verifyServerHostname
}

// IncomingXDSConfig generates a *tls.Config for incoming xDS connections.
func (c *Configurator) IncomingXDSConfig() *tls.Config {
	c.log("IncomingXDSConfig")

	// false has the effect that this config doesn't require a client cert
	// verification. This is because there is no verify_incoming_grpc
	// configuration option. And using verify_incoming would be backwards
	// incompatible, because even if it was set before, it didn't have an
	// effect on the grpc server.
	config := c.commonTLSConfig(false)
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingXDSConfig(), nil
	}
	return config
}

// IncomingRPCConfig generates a *tls.Config for incoming RPC connections.
func (c *Configurator) IncomingRPCConfig() *tls.Config {
	c.log("IncomingRPCConfig")
	config := c.commonTLSConfig(c.VerifyIncomingRPC())
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingRPCConfig(), nil
	}
	return config
}

// IncomingALPNRPCConfig generates a *tls.Config for incoming RPC connections
// directly using TLS with ALPN instead of the older byte-prefixed protocol.
func (c *Configurator) IncomingALPNRPCConfig(alpnProtos []string) *tls.Config {
	c.log("IncomingALPNRPCConfig")
	// Since the ALPN-RPC variation is indirectly exposed to the internet via
	// mesh gateways we force mTLS and full server name verification.
	config := c.commonTLSConfig(true)
	config.InsecureSkipVerify = false

	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingALPNRPCConfig(alpnProtos), nil
	}
	config.NextProtos = alpnProtos
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

	c.lock.RLock()
	verifyIncoming := c.base.VerifyIncoming || c.base.VerifyIncomingHTTPS
	c.lock.RUnlock()

	config := c.commonTLSConfig(verifyIncoming)
	config.NextProtos = []string{"h2", "http/1.1"}
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingHTTPSConfig(), nil
	}
	return config
}

// OutgoingTLSConfigForCheck generates a *tls.Config for outgoing TLS connections
// for checks. This function is separated because there is an extra flag to
// consider for checks. EnableAgentTLSForChecks and InsecureSkipVerify has to
// be checked for checks.
func (c *Configurator) OutgoingTLSConfigForCheck(skipVerify bool, serverName string) *tls.Config {
	c.log("OutgoingTLSConfigForCheck")

	c.lock.RLock()
	useAgentTLS := c.base.EnableAgentTLSForChecks
	c.lock.RUnlock()

	if !useAgentTLS {
		return &tls.Config{
			InsecureSkipVerify: skipVerify,
			ServerName:         serverName,
		}
	}

	if serverName == "" {
		serverName = c.serverNameOrNodeName()
	}
	config := c.commonTLSConfig(false)
	config.InsecureSkipVerify = skipVerify
	config.ServerName = serverName

	return config
}

// OutgoingRPCConfig generates a *tls.Config for outgoing RPC connections. If
// there is a CA or VerifyOutgoing is set, a *tls.Config will be provided,
// otherwise we assume that no TLS should be used.
func (c *Configurator) OutgoingRPCConfig() *tls.Config {
	c.log("OutgoingRPCConfig")
	if !c.outgoingRPCTLSEnabled() {
		return nil
	}
	return c.commonTLSConfig(false)
}

// outgoingALPNRPCConfig generates a *tls.Config for outgoing RPC connections
// directly using TLS with ALPN instead of the older byte-prefixed protocol.
// If there is a CA or VerifyOutgoing is set, a *tls.Config will be provided,
// otherwise we assume that no TLS should be used which completely disables the
// ALPN variation.
func (c *Configurator) outgoingALPNRPCConfig() *tls.Config {
	c.log("outgoingALPNRPCConfig")
	if !c.MutualTLSCapable() {
		return nil // ultimately this will hard-fail as TLS is required
	}

	// Since the ALPN-RPC variation is indirectly exposed to the internet via
	// mesh gateways we force mTLS and full server name verification.
	config := c.commonTLSConfig(true)
	config.InsecureSkipVerify = false
	return config
}

// OutgoingRPCWrapper wraps the result of OutgoingRPCConfig in a DCWrapper. It
// decides if verify server hostname should be used.
func (c *Configurator) OutgoingRPCWrapper() DCWrapper {
	c.log("OutgoingRPCWrapper")

	// Generate the wrapper based on dc
	return func(dc string, conn net.Conn) (net.Conn, error) {
		if c.UseTLS(dc) {
			return c.wrapTLSClient(dc, conn)
		}
		return conn, nil
	}
}

// UseTLS returns true if the outgoing RPC requests have been explicitly configured
// to use TLS (via VerifyOutgoing or AutoTLS, and the target DC supports TLS.
func (c *Configurator) UseTLS(dc string) bool {
	return c.outgoingRPCTLSEnabled() && c.getAreaForPeerDatacenterUseTLS(dc)
}

// OutgoingALPNRPCWrapper wraps the result of outgoingALPNRPCConfig in an
// ALPNWrapper. It configures all of the negotiation plumbing.
func (c *Configurator) OutgoingALPNRPCWrapper() ALPNWrapper {
	c.log("OutgoingALPNRPCWrapper")
	if !c.MutualTLSCapable() {
		return nil
	}

	return c.wrapALPNTLSClient
}

// AutoEncryptCert returns the TLS certificate received from auto-encrypt.
func (c *Configurator) AutoEncryptCert() *x509.Certificate {
	c.lock.RLock()
	defer c.lock.RUnlock()
	tlsCert := c.autoTLS.cert
	if tlsCert == nil || tlsCert.Certificate == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil
	}
	return cert
}

func (c *Configurator) log(name string) {
	if c.logger != nil && c.logger.IsTrace() {
		c.logger.Trace(name, "version", atomic.LoadUint64(&c.version))
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

	cs := tlsConn.ConnectionState()
	for _, cert := range cs.PeerCertificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	_, err = cs.PeerCertificates[0].Verify(opts)
	if err != nil {
		tlsConn.Close()
		return nil, err
	}

	return tlsConn, err
}

// Wrap a net.Conn into a client tls connection suitable for secure ALPN-RPC,
// performing any additional verification as needed.
func (c *Configurator) wrapALPNTLSClient(dc, nodeName, alpnProto string, conn net.Conn) (net.Conn, error) {
	if dc == "" {
		return nil, fmt.Errorf("cannot dial using ALPN-RPC without a target datacenter")
	} else if nodeName == "" {
		return nil, fmt.Errorf("cannot dial using ALPN-RPC without a target node")
	} else if alpnProto == "" {
		return nil, fmt.Errorf("cannot dial using ALPN-RPC without a target alpn protocol")
	}

	config := c.outgoingALPNRPCConfig()
	if config == nil {
		return nil, fmt.Errorf("cannot dial via a mesh gateway when outgoing TLS is disabled")
	}

	// Since the ALPN-RPC variation is indirectly exposed to the internet via
	// mesh gateways we force mTLS and full hostname validation (forcing
	// verify_server_hostname and verify_outgoing to be effectively true).

	config.ServerName = c.ServerSNI(dc, nodeName)
	config.NextProtos = []string{alpnProto}

	tlsConn := tls.Client(conn, config)

	// NOTE: For this handshake to succeed the server must have key material
	// for either "<nodename>.server.<datacenter>.<domain>" or
	// "*.server.<datacenter>.<domain>" in addition to the
	// "server.<datacenter>.<domain>" required for standard TLS'd RPC.
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return nil, err
	}

	return tlsConn, nil
}

// AuthorizeServerConn is used to validate that the connection is being established
// by a Consul server in the same datacenter.
//
// The identity of the connection is checked by verifying that the certificate
// presented is signed by the Agent TLS CA, and has a DNSName that matches the
// local ServerSNI name.
//
// Note this check is only performed if VerifyServerHostname is enabled, otherwise
// it does no authorization.
func (c *Configurator) AuthorizeServerConn(dc string, conn *tls.Conn) error {
	if !c.VerifyServerHostname() {
		return nil
	}

	c.lock.RLock()
	caPool := c.manual.caPool
	c.lock.RUnlock()

	expected := c.ServerSNI(dc, "")
	cs := conn.ConnectionState()
	var errs error
	for _, chain := range cs.VerifiedChains {
		if len(chain) == 0 {
			continue
		}
		opts := x509.VerifyOptions{
			DNSName:       expected,
			Intermediates: x509.NewCertPool(),
			Roots:         caPool,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		for _, cert := range cs.PeerCertificates[1:] {
			opts.Intermediates.AddCert(cert)
		}
		_, err := cs.PeerCertificates[0].Verify(opts)
		if err == nil {
			return nil
		}
		multierror.Append(errs, err)
	}
	return fmt.Errorf("AuthorizeServerConn failed certificate validation for certificate with a SAN.DNSName of %v: %w", expected, errs)

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

	// Note: this needs to be kept up to date with the cipherMap in CipherString
	cipherMap := map[string]uint16{
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
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

// CipherString performs the inverse operation of ParseCiphers
func CipherString(ciphers []uint16) (string, error) {
	// Note: this needs to be kept up to date with the cipherMap in ParseCiphers
	cipherMap := map[uint16]string{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:    "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:    "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:      "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:   "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:   "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:      "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:   "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	}

	cipherStrings := make([]string, len(ciphers))
	for i, cipher := range ciphers {
		if v, ok := cipherMap[cipher]; ok {
			cipherStrings[i] = v
		} else {
			return "", fmt.Errorf("unsupported cipher %d", cipher)
		}
	}

	return strings.Join(cipherStrings, ","), nil
}
