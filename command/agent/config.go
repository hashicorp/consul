package agent

import (
	"github.com/hashicorp/consul/consul"
)

// This is the default port we use for co
const DefaultBindPort int = 8300

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
	// Datacenter is the datacenter this node is in. Defaults to dc1
	Datacenter string

	// DataDir is the directory to store our state in
	DataDir string

	// LogLevel is the level of the logs to putout
	LogLevel string

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// RPCAddr is the address and port to listen on for the
	// agent's RPC interface.
	RPCAddr string

	// BindAddr is the address that Consul's RPC and Serf's will
	// bind to. This address should be routable by all other hosts.
	SerfBindAddr string

	// SerfLanPort is the port we use for the lan-local serf cluster
	// This is used for all nodes.
	SerfLanPort int

	// SerfWanPort is the port we use for the wan serf cluster.
	// This is only for the Consul servers
	SerfWanPort int

	// ServerRPCAddr is the address we use for Consul server communication.
	// Defaults to 0.0.0.0:8300
	ServerRPCAddr string

	// Server controls if this agent acts like a Consul server,
	// or merely as a client. Servers have more state, take part
	// in leader election, etc.
	Server bool

	// ConsulConfig can either be provided or a default one created
	ConsulConfig *consul.Config
}

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		LogLevel: "INFO",
		RPCAddr:  "127.0.0.1:8400",
		Server:   false,
	}
}
