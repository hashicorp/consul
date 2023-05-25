// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/private/pbconfig"
	"github.com/hashicorp/consul/types"
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

// goTLSVersions maps types.TLSVersion to the Go internal value
var goTLSVersions = map[types.TLSVersion]uint16{
	types.TLSVersionAuto: tls.VersionTLS12,
	types.TLSv1_0:        tls.VersionTLS10,
	types.TLSv1_1:        tls.VersionTLS11,
	types.TLSv1_2:        tls.VersionTLS12,
	types.TLSv1_3:        tls.VersionTLS13,
}

// ProtocolConfig contains configuration for a given protocol.
type ProtocolConfig struct {
	// VerifyIncoming is used to verify the authenticity of incoming
	// connections.  This means that TCP requests are forbidden, only
	// allowing for TLS. TLS connections must match a provided certificate
	// authority. This can be used to force client auth.
	VerifyIncoming bool

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

	// TLSMinVersion is the minimum accepted TLS version that can be used.

	TLSMinVersion types.TLSVersion

	// CipherSuites is the list of TLS cipher suites to use.
	//
	// We don't support the raw 0xNNNN values from
	// https://golang.org/pkg/crypto/tls/#pkg-constants
	// even though they are standardized by IANA because it would increase
	// the likelihood of an operator inadvertently setting an insecure configuration
	CipherSuites []types.TLSCipherSuite

	// VerifyOutgoing is used to verify the authenticity of outgoing
	// connections.  This means that TLS requests are used, and TCP
	// requests are not made. TLS connections must match a provided
	// certificate authority. This is used to verify authenticity of server
	// nodes.
	//
	// Note: this setting doesn't apply to the external gRPC configuration, as Consul
	// makes no outgoing connections using this protocol.
	VerifyOutgoing bool

	// VerifyServerHostname is used to enable hostname verification of
	// servers. This ensures that the certificate presented is valid for
	// server.<datacenter>.<domain>.  This prevents a compromised client
	// from being restarted as a server, and then intercepting request
	// traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we
	// cannot break existing clients.
	//
	// Note: this setting only applies to the Internal RPC configuration.
	VerifyServerHostname bool

	// UseAutoCert is used to enable usage of auto_encrypt/auto_config generated
	// certificate & key material on external gRPC listener.
	UseAutoCert bool
}

// Config configures the Configurator.
type Config struct {
	// ServerMode indicates whether the configurator is attached to a server
	// or client agent.
	ServerMode bool

	// InternalRPC is used to configure the internal multiplexed RPC protocol.
	InternalRPC ProtocolConfig

	// GRPC is used to configure the external (e.g. xDS) gRPC protocol.
	GRPC ProtocolConfig

	// HTTPS is used to configure the external HTTPS protocol.
	HTTPS ProtocolConfig

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// ServerName is used with the TLS certificate to ensure the name we
	// provide matches the certificate
	ServerName string

	// Domain is the Consul TLD being used. Defaults to "consul."
	Domain string

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

// protocolConfig contains the loaded state (e.g. x509 certificates) for a given
// ProtocolConfig.
type protocolConfig struct {
	// cert is the TLS certificate configured manually by the cert_file/key_file
	// options in the configuration file.
	cert *tls.Certificate

	// manualCAPEMs contains the PEM-encoded CA certificates provided manually by
	// the ca_file/ca_path options in the configuration file.
	manualCAPEMs []string

	// manualCAPool is a pool containing only manualCAPEM, for cases where it is
	// not appropriate to trust the Connect CA (e.g. when verifying server identity
	// in AuthorizeServerConn).
	manualCAPool *x509.CertPool

	// combinedCAPool is a pool containing both manualCAPEMs and the certificates
	// received from auto-config/auto-encrypt.
	combinedCAPool *x509.CertPool

	// useAutoCert indicates wether we should use auto-encrypt/config data
	// for TLS server/listener. NOTE: Only applies to external GRPC Server.
	useAutoCert bool
}

// Configurator provides tls.Config and net.Dial wrappers to enable TLS for
// clients and servers, for internal RPC, and external gRPC and HTTPS connections.
//
// Configurator receives an initial TLS configuration from agent configuration,
// and receives updates from config reloads, auto-encrypt, and auto-config.
type Configurator struct {
	// version is increased each time the Configurator is updated. Must be accessed
	// using sync/atomic. Also MUST be the first field in this struct to ensure
	// 64-bit alignment. See https://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	version uint64

	// lock synchronizes access to all fields on this struct except for logger and version.
	lock sync.RWMutex
	base *Config
	// peerDatacenterUseTLS is a map of DC name to a bool indicating if the DC
	// uses TLS for RPC requests.
	peerDatacenterUseTLS map[string]bool

	grpc        protocolConfig
	https       protocolConfig
	internalRPC protocolConfig

	// autoTLS stores configuration that is received from:
	// - The auto-encrypt or auto-config features for client agents
	// - The servercert.CertManager for server agents.
	autoTLS struct {
		extraCAPems          []string
		connectCAPems        []string
		cert                 *tls.Certificate
		verifyServerHostname bool
		peeringServerName    string
	}

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

// ManualCAPems returns the currently loaded CAs for the internal RPC protocol
// in PEM format. It is used in the auto-config/auto-encrypt endpoints.
func (c *Configurator) ManualCAPems() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.internalRPC.manualCAPEMs
}

// GRPCManualCAPems returns the currently loaded CAs for the gRPC in PEM format.
func (c *Configurator) GRPCManualCAPems() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.grpc.manualCAPEMs
}

// Update updates the internal configuration which is used to generate
// *tls.Config.
// This function acquires a write lock because it writes the new config.
func (c *Configurator) Update(config Config) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	grpc, err := c.loadProtocolConfig(config, config.GRPC)
	if err != nil {
		return err
	}

	https, err := c.loadProtocolConfig(config, config.HTTPS)
	if err != nil {
		return err
	}

	internalRPC, err := c.loadProtocolConfig(config, config.InternalRPC)
	if err != nil {
		return err
	}

	c.base = &config
	c.grpc = *grpc
	c.https = *https
	c.internalRPC = *internalRPC

	atomic.AddUint64(&c.version, 1)
	c.log("Update")
	return nil
}

// loadProtocolConfig loads the certificates etc. for a given ProtocolConfig
// and performs validation.
func (c *Configurator) loadProtocolConfig(base Config, pc ProtocolConfig) (*protocolConfig, error) {
	cert, err := loadKeyPair(pc.CertFile, pc.KeyFile)
	if err != nil {
		return nil, err
	}
	pems, err := LoadCAs(pc.CAFile, pc.CAPath)
	if err != nil {
		return nil, err
	}
	manualPool, err := newX509CertPool(pems)
	if err != nil {
		return nil, err
	}
	combinedPool, err := newX509CertPool(pems, c.autoTLS.connectCAPems, c.autoTLS.extraCAPems)
	if err != nil {
		return nil, err
	}

	if pc.VerifyIncoming {
		// Both auto-config and auto-encrypt require verifying the connection from the
		// client to the server for secure operation. In order to be able to verify the
		// server's certificate we must have some CA certs already provided. Therefore,
		// even though both of those features can push down extra CA certificates which
		// could be used to verify incoming connections, we still must consider it an
		// error if none are provided in the initial configuration as those features
		// cannot be successfully enabled without providing CA certificates to use those
		// features.
		if combinedPool == nil {
			return nil, fmt.Errorf("VerifyIncoming set but no CA certificates were provided")
		}

		// We will use the auto_encrypt/auto_config cert for TLS in the incoming APIs
		// when available. Therefore the check here will ensure that either we enabled
		// one of those two features or a certificate and key were provided manually
		if cert == nil && !base.AutoTLS {
			return nil, fmt.Errorf("VerifyIncoming requires either a Cert and Key pair in the configuration file, or auto_encrypt/auto_config be enabled")
		}
	}

	// Ensure we have a CA if VerifyOutgoing is set.
	if pc.VerifyOutgoing && combinedPool == nil {
		return nil, fmt.Errorf("VerifyOutgoing set but no CA certificates were provided")
	}

	return &protocolConfig{
		cert:           cert,
		manualCAPEMs:   pems,
		manualCAPool:   manualPool,
		combinedCAPool: combinedPool,
		useAutoCert:    pc.UseAutoCert,
	}, nil
}

// UpdateAutoTLSCA updates the autoEncrypt.caPems. This is supposed to be called
// from the server in order to be able to accept TLS connections with TLS
// certificates.
// Or it is being called on the client side when CA changes are detected.
func (c *Configurator) UpdateAutoTLSCA(connectCAPems []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	makePool := func(l protocolConfig) (*x509.CertPool, error) {
		return newX509CertPool(l.manualCAPEMs, c.autoTLS.extraCAPems, connectCAPems)
	}

	// Make all of the pools up-front (before assigning anything) so that if any of
	// them fails, we aren't left in a half-applied state.
	internalRPCPool, err := makePool(c.internalRPC)
	if err != nil {
		return err
	}
	grpcPool, err := makePool(c.grpc)
	if err != nil {
		return err
	}
	httpsPool, err := makePool(c.https)
	if err != nil {
		return err
	}

	c.autoTLS.connectCAPems = connectCAPems
	c.internalRPC.combinedCAPool = internalRPCPool
	c.grpc.combinedCAPool = grpcPool
	c.https.combinedCAPool = httpsPool

	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAutoTLSCA")
	return nil
}

// UpdateAutoTLSCert receives the updated automatically-provisioned certificate.
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

// UpdateAutoTLSPeeringServerName receives the updated automatically-provisioned certificate.
func (c *Configurator) UpdateAutoTLSPeeringServerName(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.autoTLS.peeringServerName = name
	atomic.AddUint64(&c.version, 1)
	c.log("UpdateAutoTLSPeeringServerName")
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

	makePool := func(l protocolConfig) (*x509.CertPool, error) {
		return newX509CertPool(l.manualCAPEMs, manualCAPems, connectCAPems)
	}

	// Make all of the pools up-front (before assigning anything) so that if any of
	// them fails, we aren't left in a half-applied state.
	internalRPCPool, err := makePool(c.internalRPC)
	if err != nil {
		return err
	}
	grpcPool, err := makePool(c.grpc)
	if err != nil {
		return err
	}
	httpsPool, err := makePool(c.https)
	if err != nil {
		return err
	}

	c.autoTLS.extraCAPems = manualCAPems
	c.autoTLS.connectCAPems = connectCAPems
	c.autoTLS.cert = &cert
	c.autoTLS.verifyServerHostname = verifyServerHostname
	c.internalRPC.combinedCAPool = internalRPCPool
	c.grpc.combinedCAPool = grpcPool
	c.https.combinedCAPool = httpsPool

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
	var haveCerts bool
	pool := x509.NewCertPool()
	for _, group := range groups {
		for _, pem := range group {
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return nil, fmt.Errorf("failed to parse PEM %s", pem)
			}
			if len(pem) > 0 {
				haveCerts = true
			}
		}
	}
	if !haveCerts {
		return nil, nil
	}
	return pool, nil
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
		pem, err := os.ReadFile(path)
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

// internalRPCTLSConfig generates a *tls.Config for the internal RPC protocol.
//
// This function acquires a read lock because it reads from the config.
func (c *Configurator) internalRPCTLSConfig(verifyIncoming bool) *tls.Config {
	c.lock.RLock()
	defer c.lock.RUnlock()

	config := c.commonTLSConfig(
		c.internalRPC,
		c.base.InternalRPC,
		verifyIncoming,
	)
	config.InsecureSkipVerify = !c.base.InternalRPC.VerifyServerHostname

	return config
}

// commonTLSConfig generates a *tls.Config from the base configuration the
// Configurator has. It accepts an additional flag in case a config is needed
// for incoming TLS connections.
func (c *Configurator) commonTLSConfig(state protocolConfig, cfg ProtocolConfig, verifyIncoming bool) *tls.Config {
	var tlsConfig tls.Config

	// Set the cipher suites
	if len(cfg.CipherSuites) != 0 {
		// TLS cipher suites are validated on input in agent config builder,
		// so it's safe to ignore the error case here.

		cipherSuites, _ := cipherSuiteLookup(cfg.CipherSuites)
		tlsConfig.CipherSuites = cipherSuites
	}

	// GetCertificate is used when acting as a server and responding to
	// client requests. Default to the manually configured cert, but allow
	// autoEncrypt cert too so that a client can encrypt incoming
	// connections without having a manual cert configured.
	tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		if state.cert != nil {
			return state.cert, nil
		}
		return c.autoTLS.cert, nil
	}

	// GetClientCertificate is used when acting as a client and responding
	// to a server requesting a certificate. Return the autoEncrypt certificate
	// if possible, otherwise default to the manually provisioned one.
	tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		cert := state.cert

		// In the general case we only prefer to dial out with the autoTLS cert if we are a client.
		// The server's autoTLS cert is exclusively for peering control plane traffic.
		if !c.base.ServerMode && c.autoTLS.cert != nil {
			cert = c.autoTLS.cert
		}

		if cert == nil {
			// the return value MUST not be nil but an empty certificate will be
			// treated the same as having no client certificate
			cert = &tls.Certificate{}
		}

		return cert, nil
	}

	tlsConfig.ClientCAs = state.combinedCAPool
	tlsConfig.RootCAs = state.combinedCAPool

	// Error handling is not needed here because agent config builder handles ""
	// or a nil value as TLSVersionAuto with goTLSVersions mapping TLSVersionAuto
	// to TLS 1.2 and because the initial check makes sure a specified version is
	// not invalid.
	tlsConfig.MinVersion = goTLSVersions[cfg.TLSMinVersion]

	// Set ClientAuth if necessary
	if verifyIncoming {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return &tlsConfig
}

// Cert returns the certificate used for connections on the internal RPC protocol.
//
// This function acquires a read lock because it reads from the config.
func (c *Configurator) Cert() *tls.Certificate {
	c.lock.RLock()
	defer c.lock.RUnlock()
	cert := c.internalRPC.cert
	if cert == nil {
		cert = c.autoTLS.cert
	}
	return cert
}

// GRPCServerUseTLS returns whether there's a TLS certificate configured for
// (external) gRPC (either manually or by auto-config/auto-encrypt), and use
// of TLS for gRPC has not been explicitly disabled at auto-encrypt.
//
// This function acquires a read lock because it reads from the config.
func (c *Configurator) GRPCServerUseTLS() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.grpc.cert != nil || (c.grpc.useAutoCert && c.autoTLS.cert != nil)
}

// VerifyIncomingRPC returns true if we should verify incoming connnections to
// the internal RPC protocol.
func (c *Configurator) VerifyIncomingRPC() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.base.InternalRPC.VerifyIncoming
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) outgoingRPCTLSEnabled() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// use TLS if AutoEncrypt or VerifyOutgoing are enabled.
	return c.base.AutoTLS || c.base.InternalRPC.VerifyOutgoing
}

// MutualTLSCapable returns true if Configurator has a CA and a local TL
// certificate configured on the internal RPC protocol.
func (c *Configurator) MutualTLSCapable() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.internalRPC.combinedCAPool != nil && (c.autoTLS.cert != nil || c.internalRPC.cert != nil)
}

// This function acquires a read lock because it reads from the config.
func (c *Configurator) verifyOutgoing() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	// If AutoEncryptTLS is enabled and there is a CA, then verify
	// outgoing.
	if c.base.AutoTLS && c.internalRPC.combinedCAPool != nil {
		return true
	}

	return c.base.InternalRPC.VerifyOutgoing
}

// This function acquires a read lock because it reads from the config.
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
	return c.base.InternalRPC.VerifyServerHostname || c.autoTLS.verifyServerHostname
}

// AutoConfigTLSSettings constructs the pbconfig.TLS that will be returned by
// servers in the auto-config endpoint.
func (c *Configurator) AutoConfigTLSSettings() (*pbconfig.TLS, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	cfg := c.base.InternalRPC

	cipherString, err := CipherString(cfg.CipherSuites)
	if err != nil {
		return nil, err
	}

	return &pbconfig.TLS{
		VerifyOutgoing:       cfg.VerifyOutgoing,
		VerifyServerHostname: cfg.VerifyServerHostname || c.autoTLS.verifyServerHostname,
		MinVersion:           types.ConsulAutoConfigTLSVersionStrings[cfg.TLSMinVersion],
		CipherSuites:         cipherString,
	}, nil
}

// IncomingGRPCConfig generates a *tls.Config for incoming external (e.g. xDS)
// GRPC connections.
//
// This function acquires a read lock because it reads from the config.
func (c *Configurator) IncomingGRPCConfig() *tls.Config {
	c.log("IncomingGRPConfig")

	c.lock.RLock()
	defer c.lock.RUnlock()

	config := c.commonTLSConfig(
		c.grpc,
		c.base.GRPC,
		c.base.GRPC.VerifyIncoming,
	)
	config.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		conf := c.IncomingGRPCConfig()
		// Do not enforce mutualTLS for peering SNI entries. This is necessary, because
		// there is no way to specify an mTLS cert when establishing a peering connection.
		// This bypass is only safe because the `grpc-middleware.AuthInterceptor` explicitly
		// restricts the list of endpoints that can be called when peering SNI is present.
		if c.autoTLS.peeringServerName != "" && info.ServerName == c.autoTLS.peeringServerName {
			conf.ClientAuth = tls.NoClientCert
		}
		return conf, nil
	}
	config.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if c.autoTLS.peeringServerName != "" && info.ServerName == c.autoTLS.peeringServerName {
			// For peering control plane traffic we exclusively use the internally managed certificate.
			// For all other traffic it is only a fallback if no manual certificate is provisioned.
			return c.autoTLS.cert, nil
		}

		if c.grpc.cert != nil {
			return c.grpc.cert, nil
		}
		return c.autoTLS.cert, nil
	}
	return config
}

// IncomingRPCConfig generates a *tls.Config for incoming RPC connections.
func (c *Configurator) IncomingRPCConfig() *tls.Config {
	c.log("IncomingRPCConfig")
	config := c.internalRPCTLSConfig(c.base.InternalRPC.VerifyIncoming)
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
	config := c.internalRPCTLSConfig(true)
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
	config := c.internalRPCTLSConfig(false)
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingInsecureRPCConfig(), nil
	}
	return config
}

// IncomingHTTPSConfig generates a *tls.Config for incoming HTTPS connections.
func (c *Configurator) IncomingHTTPSConfig() *tls.Config {
	c.log("IncomingHTTPSConfig")

	c.lock.RLock()
	defer c.lock.RUnlock()

	config := c.commonTLSConfig(
		c.https,
		c.base.HTTPS,
		c.base.HTTPS.VerifyIncoming,
	)
	config.NextProtos = []string{"h2", "http/1.1"}
	config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return c.IncomingHTTPSConfig(), nil
	}
	return config
}

// OutgoingTLSConfigForCheck creates a client *tls.Config for executing checks.
// It is RECOMMENDED that the serverName be left unspecified. The crypto/tls
// client will deduce the ServerName (for SNI) from the check address unless
// it's an IP (RFC 6066, Section 3). However, there are two instances where
// supplying a serverName is useful:
//
//  1. When the check address is an IP, a serverName can be supplied for SNI.
//     Note: setting serverName will also override the hostname used to verify
//     the certificate presented by the server being checked.
//
//  2. When the a hostname in the check address won't be present in the SAN
//     (Subject Alternative Name) field of the certificate presented by the
//     server being checked. Note: setting serverName will also override the
//     ServerName used for SNI.
//
// Setting skipVerify will disable verification of the server's certificate
// chain and hostname, which is generally not suitable for production use.
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

	config := c.internalRPCTLSConfig(false)
	config.InsecureSkipVerify = skipVerify
	config.ServerName = serverName
	return config
}

// OutgoingRPCConfig generates a *tls.Config for outgoing internal RPC
// connections. If there is a CA or VerifyOutgoing is set, a *tls.Config
// will be provided, otherwise we assume that no TLS should be used.
func (c *Configurator) OutgoingRPCConfig() *tls.Config {
	c.log("OutgoingRPCConfig")
	if !c.outgoingRPCTLSEnabled() {
		return nil
	}
	return c.internalRPCTLSConfig(false)
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
	config := c.internalRPCTLSConfig(true)
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

func (c *Configurator) PeeringServerName() string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.autoTLS.peeringServerName
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

type TLSConn interface {
	ConnectionState() tls.ConnectionState
}

// AuthorizeServerConn is used to validate that the connection is being established
// by a Consul server in the same datacenter.
//
// The identity of the connection is checked by verifying that the certificate
// presented is signed by the Agent TLS CA, and has a DNSName that matches the
// local ServerSNI name.
//
// Note this check is only performed if VerifyServerHostname and VerifyIncomingRPC
// are both enabled, otherwise it does no authorization.
func (c *Configurator) AuthorizeServerConn(dc string, conn TLSConn) error {
	if !c.VerifyIncomingRPC() || !c.VerifyServerHostname() {
		return nil
	}

	c.lock.RLock()
	caPool := c.internalRPC.manualCAPool
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
		errs = multierror.Append(errs, err)
	}
	if errs == nil {
		errs = fmt.Errorf("no verified chains")
	}
	return fmt.Errorf("AuthorizeServerConn failed certificate validation for certificate with a SAN.DNSName of %v: %w", expected, errs)

}

// NOTE: any new cipher suites will also need to be added in types/tls.go
// TODO: should this be moved into types/tls.go? Would importing Go's tls
// package in there be acceptable?
var goTLSCipherSuites = map[types.TLSCipherSuite]uint16{
	types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256: tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	types.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:          tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	types.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:          tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	types.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,

	types.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256: tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	types.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:          tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:       tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	types.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:          tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	types.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:       tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

func cipherSuiteLookup(ciphers []types.TLSCipherSuite) ([]uint16, error) {
	suites := []uint16{}

	if len(ciphers) == 0 {
		return []uint16{}, nil
	}

	for _, cipher := range ciphers {
		if v, ok := goTLSCipherSuites[cipher]; ok {
			suites = append(suites, v)
		} else {
			return suites, fmt.Errorf("unsupported cipher %q", cipher)
		}
	}

	return suites, nil
}

// CipherString performs the inverse operation of types.ParseCiphers
func CipherString(ciphers []types.TLSCipherSuite) (string, error) {
	err := types.ValidateConsulAgentCipherSuites(ciphers)
	if err != nil {
		return "", err
	}

	cipherStrings := make([]string, len(ciphers))
	for i, cipher := range ciphers {
		cipherStrings[i] = string(cipher)
	}

	return strings.Join(cipherStrings, ","), nil
}
