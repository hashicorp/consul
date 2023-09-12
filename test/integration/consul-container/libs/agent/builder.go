package agent

import (
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"

	agentconfig "github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/tlsutil"
)

const (
	remoteCertDirectory = "/consul/config/certs"
)

// BuildContext provides a reusable object meant to share common configuration settings
// between agent configuration builders.
type BuildContext struct {
	datacenter             string
	encryptKey             string
	caCert                 string
	caKey                  string
	index                  int  // keeps track of the certificates issued for naming purposes
	injectAutoEncryption   bool // initialize the built-in CA and set up agents to use auto-encrpt
	injectCerts            bool // initializes the built-in CA and distributes client certificates to agents
	injectGossipEncryption bool // setup the agents to use a gossip encryption key
}

// BuildOptions define the desired automated test setup overrides that are
// applied across agents in the cluster
type BuildOptions struct {
	Datacenter             string // Override datacenter for agents
	InjectCerts            bool   // Provides a CA for all agents and (future) agent certs.
	InjectAutoEncryption   bool   // Configures auto-encrypt for TLS and sets up certs. Overrides InjectCerts.
	InjectGossipEncryption bool   // Provides a gossip encryption key for all agents.
}

func NewBuildContext(opts BuildOptions) (*BuildContext, error) {
	ctx := &BuildContext{
		datacenter:             opts.Datacenter,
		injectAutoEncryption:   opts.InjectAutoEncryption,
		injectCerts:            opts.InjectCerts,
		injectGossipEncryption: opts.InjectGossipEncryption,
	}

	if opts.InjectGossipEncryption {
		serfKey, err := newSerfEncryptionKey()
		if err != nil {
			return nil, errors.Wrap(err, "could not generate serf encryption key")
		}
		ctx.encryptKey = serfKey
	}

	if opts.InjectAutoEncryption || opts.InjectCerts {
		// This is the same call that 'consul tls ca create` will run
		caCert, caKey, err := tlsutil.GenerateCA(tlsutil.CAOpts{Domain: "consul", PermittedDNSDomains: []string{"consul", "localhost"}})
		if err != nil {
			return nil, errors.Wrap(err, "could not generate built-in CA root pair")
		}
		ctx.caCert = caCert
		ctx.caKey = caKey
	}
	return ctx, nil
}

func (c *BuildContext) GetCerts() (cert string, key string) {
	return c.caCert, c.caKey
}

type Builder struct {
	conf    *agentconfig.Config
	certs   map[string]string
	context *BuildContext
}

// NewConfigBuilder instantiates a builder object with sensible defaults for a single consul instance
// This includes the following:
// * default ports with no plaintext options
// * debug logging
// * single server with bootstrap
// * bind to all interfaces, advertise on 'eth0'
// * connect enabled
func NewConfigBuilder(ctx *BuildContext) *Builder {
	b := &Builder{
		certs: map[string]string{},
		conf: &agentconfig.Config{
			AdvertiseAddrLAN: utils.StringToPointer(`{{ GetInterfaceIP "eth0" }}`),
			BindAddr:         utils.StringToPointer("0.0.0.0"),
			Bootstrap:        utils.BoolToPointer(true),
			ClientAddr:       utils.StringToPointer("0.0.0.0"),
			Connect: agentconfig.Connect{
				Enabled: utils.BoolToPointer(true),
			},
			LogLevel:   utils.StringToPointer("DEBUG"),
			ServerMode: utils.BoolToPointer(true),
		},
		context: ctx,
	}

	// These are the default ports, disabling plaintext transport
	b.conf.Ports = agentconfig.Ports{
		DNS:     utils.IntToPointer(8600),
		HTTP:    nil,
		HTTPS:   utils.IntToPointer(8501),
		GRPC:    utils.IntToPointer(8502),
		GRPCTLS: utils.IntToPointer(8503),
		SerfLAN: utils.IntToPointer(8301),
		SerfWAN: utils.IntToPointer(8302),
		Server:  utils.IntToPointer(8300),
	}

	return b
}

func (b *Builder) Bootstrap(servers int) *Builder {
	if servers < 1 {
		b.conf.Bootstrap = nil
		b.conf.BootstrapExpect = nil
	} else if servers == 1 {
		b.conf.Bootstrap = utils.BoolToPointer(true)
		b.conf.BootstrapExpect = nil
	} else {
		b.conf.Bootstrap = nil
		b.conf.BootstrapExpect = utils.IntToPointer(servers)
	}
	return b
}

func (b *Builder) Client() *Builder {
	b.conf.Ports.Server = nil
	b.conf.ServerMode = nil
	b.conf.Bootstrap = nil
	b.conf.BootstrapExpect = nil
	return b
}

func (b *Builder) Datacenter(name string) *Builder {
	b.conf.Datacenter = utils.StringToPointer(name)
	return b
}

func (b *Builder) Peering(enable bool) *Builder {
	b.conf.Peering = agentconfig.Peering{
		Enabled: utils.BoolToPointer(enable),
	}
	return b
}

func (b *Builder) RetryJoin(names ...string) *Builder {
	b.conf.RetryJoinLAN = names
	return b
}

func (b *Builder) Telemetry(statSite string) *Builder {
	b.conf.Telemetry = agentconfig.Telemetry{
		StatsiteAddr: utils.StringToPointer(statSite),
	}
	return b
}

// ToAgentConfig renders the builders configuration into a string
// representation of the json config file for agents.
// DANGER! Some fields may not have json tags in the Agent Config.
// You may need to add these yourself.
func (b *Builder) ToAgentConfig() (*Config, error) {
	b.injectContextOptions()

	out, err := json.MarshalIndent(b.conf, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "could not marshall builder")
	}

	conf := &Config{
		Certs:   b.certs,
		Cmd:     []string{"agent"},
		Image:   utils.TargetImage,
		JSON:    string(out),
		Version: utils.TargetVersion,
	}

	return conf, nil
}

func (b *Builder) injectContextOptions() {
	if b.context == nil {
		return
	}

	var dc string
	if b.context.datacenter != "" {
		b.conf.Datacenter = utils.StringToPointer(b.context.datacenter)
		dc = b.context.datacenter
	}
	if b.conf.Datacenter == nil || *b.conf.Datacenter == "" {
		dc = "dc1"
	}

	server := b.conf.ServerMode != nil && *b.conf.ServerMode

	if b.context.encryptKey != "" {
		b.conf.EncryptKey = utils.StringToPointer(b.context.encryptKey)
	}

	// For any TLS setup, we add the CA to agent conf
	if b.context.caCert != "" {
		// Add the ca file to the list of certs that will be mounted to consul
		filename := filepath.Join(remoteCertDirectory, "consul-agent-ca.pem")
		b.certs[filename] = b.context.caCert

		b.conf.TLS = agentconfig.TLS{
			Defaults: agentconfig.TLSProtocolConfig{
				CAFile:         utils.StringToPointer(filename),
				VerifyOutgoing: utils.BoolToPointer(true), // Secure settings
			},
			InternalRPC: agentconfig.TLSProtocolConfig{
				VerifyServerHostname: utils.BoolToPointer(true),
			},
		}
	}

	// Also for any TLS setup, generate server key pairs from the CA
	if b.context.caCert != "" && server {
		keyFileName, priv, certFileName, pub := newServerTLSKeyPair(dc, b.context)

		// Add the key pair to the list that will be mounted to consul
		certFileName = filepath.Join(remoteCertDirectory, certFileName)
		keyFileName = filepath.Join(remoteCertDirectory, keyFileName)

		b.certs[certFileName] = pub
		b.certs[keyFileName] = priv

		b.conf.TLS.Defaults.CertFile = utils.StringToPointer(certFileName)
		b.conf.TLS.Defaults.KeyFile = utils.StringToPointer(keyFileName)
		b.conf.TLS.Defaults.VerifyIncoming = utils.BoolToPointer(true) // Only applies to servers for auto-encrypt
	}

	// This assumes we've already gone through the CA/Cert setup in the previous conditional
	if b.context.injectAutoEncryption && server {
		b.conf.AutoEncrypt = agentconfig.AutoEncrypt{
			AllowTLS: utils.BoolToPointer(true), // This setting is different between client and servers
		}

		b.conf.TLS.GRPC = agentconfig.TLSProtocolConfig{
			UseAutoCert: utils.BoolToPointer(true), // This is required for peering to work over the non-GRPC_TLS port
		}

		// VerifyIncoming does not apply to client agents for auto-encrypt
	}

	if b.context.injectAutoEncryption && !server {
		b.conf.AutoEncrypt = agentconfig.AutoEncrypt{
			TLS: utils.BoolToPointer(true), // This setting is different between client and servers
		}

		b.conf.TLS.GRPC = agentconfig.TLSProtocolConfig{
			UseAutoCert: utils.BoolToPointer(true), // This is required for peering to work over the non-GRPC_TLS port
		}
	}

	if b.context.injectCerts && !b.context.injectAutoEncryption {
		panic("client certificate distribution not implemented")
	}
	b.context.index++
}
