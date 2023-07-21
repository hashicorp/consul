package cluster

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TODO: switch from semver to go-version

const (
	remoteCertDirectory = "/consul/config/certs"

	ConsulCACertPEM = "consul-agent-ca.pem"
	ConsulCACertKey = "consul-agent-ca-key.pem"
)

type LogStore string

const (
	LogStore_WAL    LogStore = "wal"
	LogStore_BoltDB LogStore = "boltdb"
)

// BuildContext provides a reusable object meant to share common configuration settings
// between agent configuration builders.
type BuildContext struct {
	datacenter      string
	consulImageName string
	consulVersion   string

	injectGossipEncryption bool // setup the agents to use a gossip encryption key
	encryptKey             string

	injectCerts          bool // initializes the built-in CA and distributes client certificates to agents
	injectAutoEncryption bool // initialize the built-in CA and set up agents to use auto-encrpt
	allowHTTPAnyway      bool
	useAPIWithTLS        bool
	useGRPCWithTLS       bool

	certVolume   string
	caCert       string
	tlsCertIndex int // keeps track of the certificates issued for naming purposes

	aclEnabled bool
	logStore   LogStore
}

func (c *BuildContext) DockerImage() string {
	return utils.DockerImage(c.consulImageName, c.consulVersion)
}

// BuildOptions define the desired automated test setup overrides that are
// applied across agents in the cluster
type BuildOptions struct {
	// Datacenter is the override datacenter for agents.
	Datacenter string

	// ConsulImageName is the default Consul image name for agents in the
	// cluster when none is specified.
	ConsulImageName string

	// ConsulVersion is the default Consul version for agents in the cluster
	// when none is specified.
	ConsulVersion string

	// InjectGossipEncryption provides a gossip encryption key for all agents.
	InjectGossipEncryption bool

	// InjectCerts provides a CA for all agents and (future) agent certs.
	//
	// It also disables the HTTP API unless AllowHTTPAnyway is enabled.
	InjectCerts bool

	// InjectAutoEncryption configures auto-encrypt for TLS and sets up certs.
	// Overrides InjectCerts.
	//
	// It also disables the HTTP API unless AllowHTTPAnyway is enabled.
	InjectAutoEncryption bool

	// AllowHTTPAnyway ensures that the HTTP API is enabled even when
	// InjectCerts or InjectAutoEncryption are enabled.
	AllowHTTPAnyway bool

	// UseAPIWithTLS ensures that any accesses for the JSON API use the https
	// port. By default it will not.
	UseAPIWithTLS bool

	// UseGRPCWithTLS ensures that any accesses for external gRPC use the
	// grpc_tls port. By default it will not.
	UseGRPCWithTLS bool

	// ACLEnabled configures acl in agent configuration
	ACLEnabled bool

	//StoreLog define which LogStore to use
	LogStore LogStore
}

func NewBuildContext(t *testing.T, opts BuildOptions) *BuildContext {
	ctx := &BuildContext{
		datacenter:             opts.Datacenter,
		consulImageName:        opts.ConsulImageName,
		consulVersion:          opts.ConsulVersion,
		injectGossipEncryption: opts.InjectGossipEncryption,
		injectCerts:            opts.InjectCerts,
		injectAutoEncryption:   opts.InjectAutoEncryption,
		allowHTTPAnyway:        opts.AllowHTTPAnyway,
		useAPIWithTLS:          opts.UseAPIWithTLS,
		useGRPCWithTLS:         opts.UseGRPCWithTLS,
		aclEnabled:             opts.ACLEnabled,
		logStore:               opts.LogStore,
	}

	if ctx.consulImageName == "" {
		ctx.consulImageName = utils.TargetImageName
	}
	if ctx.consulVersion == "" {
		ctx.consulVersion = utils.TargetVersion
	}

	if opts.InjectGossipEncryption {
		serfKey, err := newSerfEncryptionKey()
		require.NoError(t, err, "could not generate serf encryption key")
		ctx.encryptKey = serfKey
	}

	if opts.InjectAutoEncryption {
		if opts.UseAPIWithTLS {
			// TODO: we should improve this
			t.Fatalf("Cannot use TLS with the API in conjunction with Auto Encrypt because you would need to use the Connect CA Cert for verification")
		}
		if opts.UseGRPCWithTLS {
			// TODO: we should improve this
			t.Fatalf("Cannot use TLS with gRPC in conjunction with Auto Encrypt because you would need to use the Connect CA Cert for verification")
		}
	}

	if opts.InjectAutoEncryption || opts.InjectCerts {
		ctx.createTLSCAFiles(t)
	} else {
		if opts.UseAPIWithTLS {
			t.Fatalf("UseAPIWithTLS requires one of InjectAutoEncryption or InjectCerts to be set")
		}
		if opts.UseGRPCWithTLS {
			t.Fatalf("UseGRPCWithTLS requires one of InjectAutoEncryption or InjectCerts to be set")
		}
	}
	return ctx
}

type Builder struct {
	context *BuildContext // this is non-nil
	conf    *ConfigBuilder
}

// NewConfigBuilder instantiates a builder object with sensible defaults for a single consul instance
// This includes the following:
// * default ports with no plaintext options
// * debug logging
// * single server with bootstrap
// * bind to all interfaces, advertise on 'eth0'
// * connect enabled
func NewConfigBuilder(ctx *BuildContext) *Builder {
	if ctx == nil {
		panic("BuildContext is a required argument")
	}
	b := &Builder{
		conf:    &ConfigBuilder{},
		context: ctx,
	}

	b.conf.Set("advertise_addr", `{{ GetInterfaceIP "eth0" }}`)
	b.conf.Set("bind_addr", "0.0.0.0")
	b.conf.Set("data_dir", "/consul/data")
	b.conf.Set("bootstrap", true)
	b.conf.Set("client_addr", "0.0.0.0")
	b.conf.Set("connect.enabled", true)
	b.conf.Set("log_level", "debug")
	b.conf.Set("server", true)

	// These are the default ports, disabling plaintext transport
	b.conf.Set("ports.dns", 8600)
	//nolint:staticcheck
	if ctx.certVolume == "" {
		b.conf.Set("ports.http", 8500)
		b.conf.Set("ports.https", -1)
	} else {
		b.conf.Set("ports.http", -1)
		b.conf.Set("ports.https", 8501)
	}
	b.conf.Set("ports.grpc", 8502)
	b.conf.Set("ports.serf_lan", 8301)
	b.conf.Set("ports.serf_wan", 8302)
	b.conf.Set("ports.server", 8300)

	if ctx.allowHTTPAnyway {
		b.conf.Set("ports.http", 8500)
	}

	if ctx.consulVersion == "local" || semver.Compare("v"+ctx.consulVersion, "v1.14.0") >= 0 {
		// Enable GRPCTLS for version after v1.14.0
		b.conf.Set("ports.grpc_tls", 8503)
	}

	if ctx.aclEnabled {
		b.conf.Set("acl.enabled", true)
		b.conf.Set("acl.default_policy", "deny")
		b.conf.Set("acl.enable_token_persistence", true)
	}

	ls := string(ctx.logStore)
	if ls != "" && (ctx.consulVersion == "local" ||
		semver.Compare("v"+ctx.consulVersion, "v1.15.0") >= 0) {
		// Enable logstore backend for version after v1.15.0
		if ls != string(LogStore_WAL) && ls != string(LogStore_BoltDB) {
			ls = string(LogStore_BoltDB)
		}
		b.conf.Set("raft_logstore.backend", ls)
	} else {
		b.conf.Unset("raft_logstore.backend")
	}

	return b
}

// Advanced lets you directly manipulate specific config settings.
func (b *Builder) Advanced(fn func(*ConfigBuilder)) *Builder {
	if fn != nil {
		fn(b.conf)
	}
	return b
}

func (b *Builder) Bootstrap(servers int) *Builder {
	if servers < 1 {
		b.conf.Unset("bootstrap")
		b.conf.Unset("bootstrap_expect")
	} else if servers == 1 {
		b.conf.Set("bootstrap", true)
		b.conf.Unset("bootstrap_expect")
	} else {
		b.conf.Unset("bootstrap")
		b.conf.Set("bootstrap_expect", servers)
	}
	return b
}

func (b *Builder) Client() *Builder {
	b.conf.Unset("ports.server")
	b.conf.Unset("server")
	b.conf.Unset("bootstrap")
	b.conf.Unset("bootstrap_expect")
	return b
}

func (b *Builder) Datacenter(name string) *Builder {
	b.conf.Set("datacenter", name)
	return b
}

func (b *Builder) Peering(enable bool) *Builder {
	b.conf.Set("peering.enabled", enable)
	return b
}

func (b *Builder) NodeID(nodeID string) *Builder {
	b.conf.Set("node_id", nodeID)
	return b
}

func (b *Builder) Partition(name string) *Builder {
	b.conf.Set("partition", name)
	return b
}

func (b *Builder) RetryJoin(names ...string) *Builder {
	b.conf.Set("retry_join", names)
	return b
}

func (b *Builder) EnableACL() *Builder {
	b.conf.Set("acl.enabled", true)
	b.conf.Set("acl.default_policy", "deny")
	b.conf.Set("acl.enable_token_persistence", true)
	return b
}

func (b *Builder) Telemetry(statSite string) *Builder {
	b.conf.Set("telemetry.statsite_address", statSite)
	return b
}

// ToAgentConfig renders the builders configuration into a string
// representation of the json config file for agents.
func (b *Builder) ToAgentConfig(t *testing.T) *Config {
	b.injectContextOptions(t)

	out, err := json.MarshalIndent(b.conf, "", "  ")
	require.NoError(t, err, "could not generate json config")

	confCopy, err := b.conf.Clone()
	require.NoError(t, err)

	return &Config{
		JSON:          string(out),
		ConfigBuilder: confCopy,

		Cmd: []string{"agent"},

		Image:   b.context.consulImageName,
		Version: b.context.consulVersion,

		CertVolume: b.context.certVolume,
		CACert:     b.context.caCert,

		UseAPIWithTLS:  b.context.useAPIWithTLS,
		UseGRPCWithTLS: b.context.useGRPCWithTLS,

		ACLEnabled: b.context.aclEnabled,
	}
}

func (b *Builder) injectContextOptions(t *testing.T) {
	var dc string
	if b.context.datacenter != "" {
		b.conf.Set("datacenter", b.context.datacenter)
		dc = b.context.datacenter
	}
	if val, _ := b.conf.GetString("datacenter"); val == "" {
		dc = "dc1"
	}
	b.conf.Set("datacenter", dc)

	server, _ := b.conf.GetBool("server")

	if b.context.encryptKey != "" {
		b.conf.Set("encrypt", b.context.encryptKey)
	}

	// For any TLS setup, we add the CA to agent conf
	if b.context.certVolume != "" {
		b.conf.Set("tls.defaults.ca_file", filepath.Join(remoteCertDirectory, ConsulCACertPEM))
		b.conf.Set("tls.defaults.verify_outgoing", true) // Secure settings
		b.conf.Set("tls.internal_rpc.verify_server_hostname", true)
	}

	// Also for any TLS setup, generate server key pairs from the CA
	if b.context.certVolume != "" && server {
		keyFileName, certFileName := b.context.createTLSCertFiles(t, dc)
		b.context.tlsCertIndex++

		b.conf.Set("tls.defaults.cert_file", filepath.Join(remoteCertDirectory, certFileName))
		b.conf.Set("tls.defaults.key_file", filepath.Join(remoteCertDirectory, keyFileName))
		b.conf.Set("tls.internal_rpc.verify_incoming", true) // Only applies to servers for auto-encrypt
	}

	// This assumes we've already gone through the CA/Cert setup in the previous conditional
	if b.context.injectAutoEncryption {
		if server {
			b.conf.Set("auto_encrypt.allow_tls", true) // This setting is different between client and servers
			b.conf.Set("tls.grpc.use_auto_cert", true) // This is required for peering to work over the non-GRPC_TLS port
			// VerifyIncoming does not apply to client agents for auto-encrypt
		} else {
			b.conf.Set("auto_encrypt.tls", true)       // This setting is different between client and servers
			b.conf.Set("tls.grpc.use_auto_cert", true) // This is required for peering to work over the non-GRPC_TLS port
		}
	}

	if b.context.injectCerts && !b.context.injectAutoEncryption {
		panic("client certificate distribution not implemented")
	}
}
