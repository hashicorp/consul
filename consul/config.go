package consul

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
)

const (
	DefaultDC          = "dc1"
	DefaultLANSerfPort = 8301
	DefaultWANSerfPort = 8302
)

var (
	DefaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 8300}
)

// ProtocolVersionMap is the mapping of Consul protocol versions
// to Serf protocol versions. We mask the Serf protocols using
// our own protocol version.
var protocolVersionMap map[uint8]uint8

func init() {
	protocolVersionMap = map[uint8]uint8{
		1: 4,
	}
}

// Config is used to configure the server
type Config struct {
	// Bootstrap mode is used to bring up the first Consul server.
	// It is required so that it can elect a leader without any
	// other nodes being present
	Bootstrap bool

	// Datacenter is the datacenter this Consul server represents
	Datacenter string

	// DataDir is the directory to store our state in
	DataDir string

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// RPCAddr is the RPC address used by Consul. This should be reachable
	// by the WAN and LAN
	RPCAddr *net.TCPAddr

	// RPCAdvertise is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:8300", but this address must be
	// reachable
	RPCAdvertise *net.TCPAddr

	// SerfLANConfig is the configuration for the intra-dc serf
	SerfLANConfig *serf.Config

	// SerfWANConfig is the configuration for the cross-dc serf
	SerfWANConfig *serf.Config

	// ReconcileInterval controls how often we reconcile the strongly
	// consistent store with the Serf info. This is used to handle nodes
	// that are force removed, as well as intermittent unavailability during
	// leader election.
	ReconcileInterval time.Duration

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion uint8

	// VerifyIncoming is used to verify the authenticity of incoming connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncoming bool

	// VerifyOutgoing is used to verify the authenticity of outgoing connections.
	// This means that TLS requests are used, and TCP requests are not made. TLS connections
	// must match a provided certificate authority. This is used to verify authenticity of
	// server nodes.
	VerifyOutgoing bool

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string

	// ServerUp callback can be used to trigger a notification that
	// a Consul server is now up and known about.
	ServerUp func()
}

// CheckVersion is used to check if the ProtocolVersion is valid
func (c *Config) CheckVersion() error {
	if c.ProtocolVersion < ProtocolVersionMin {
		return fmt.Errorf("Protocol version '%d' too low. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	} else if c.ProtocolVersion > ProtocolVersionMax {
		return fmt.Errorf("Protocol version '%d' too high. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}
	return nil
}

// CACertificate is used to open and parse a CA file
func (c *Config) CACertificate() (*x509.Certificate, error) {
	if c.CAFile == "" {
		return nil, nil
	}

	// Read the file
	data, err := ioutil.ReadFile(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read CA file: %v", err)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse CA file: %v", err)
	}
	return cert, nil
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

// OutgoingTLSConfig generates a TLS configuration for outgoing requests
func (c *Config) OutgoingTLSConfig() (*tls.Config, error) {
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		RootCAs:            x509.NewCertPool(),
		InsecureSkipVerify: !c.VerifyOutgoing,
	}

	// Parse the CA cert if any
	ca, err := c.CACertificate()
	if err != nil {
		return nil, err
	} else if ca != nil {
		tlsConfig.RootCAs.AddCert(ca)
	}

	// Ensure we have a CA if VerifyOutgoing is set
	if c.VerifyOutgoing && ca == nil {
		return nil, fmt.Errorf("VerifyOutgoing set, and no CA certificate provided!")
	}

	// Add cert/key
	cert, err := c.KeyPair()
	if err != nil {
		return nil, err
	} else if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	return tlsConfig, nil
}

// IncomingTLSConfig generates a TLS configuration for incoming requests
func (c *Config) IncomingTLSConfig() (*tls.Config, error) {
	// Create the tlsConfig
	tlsConfig := &tls.Config{
		ClientCAs:  x509.NewCertPool(),
		ClientAuth: tls.NoClientCert,
	}

	// Parse the CA cert if any
	ca, err := c.CACertificate()
	if err != nil {
		return nil, err
	} else if ca != nil {
		tlsConfig.ClientCAs.AddCert(ca)
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
		if ca == nil {
			return nil, fmt.Errorf("VerifyIncoming set, and no CA certificate provided!")
		}
		if cert == nil {
			return nil, fmt.Errorf("VerifyIncoming set, and no Cert/Key pair provided!")
		}
	}
	return nil, nil
}

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	conf := &Config{
		Datacenter:        DefaultDC,
		NodeName:          hostname,
		RPCAddr:           DefaultRPCAddr,
		RaftConfig:        raft.DefaultConfig(),
		SerfLANConfig:     serf.DefaultConfig(),
		SerfWANConfig:     serf.DefaultConfig(),
		ReconcileInterval: 60 * time.Second,
		ProtocolVersion:   ProtocolVersionMax,
	}

	// Increase our reap interval to 3 days instead of 24h.
	conf.SerfLANConfig.ReconnectTimeout = 3 * 24 * time.Hour
	conf.SerfWANConfig.ReconnectTimeout = 3 * 24 * time.Hour

	// WAN Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	conf.SerfWANConfig.MemberlistConfig = memberlist.DefaultWANConfig()

	// Ensure we don't have port conflicts
	conf.SerfLANConfig.MemberlistConfig.BindPort = DefaultLANSerfPort
	conf.SerfWANConfig.MemberlistConfig.BindPort = DefaultWANSerfPort

	// Disable shutdown on removal
	conf.RaftConfig.ShutdownOnRemove = false

	return conf
}
