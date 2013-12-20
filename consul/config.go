package consul

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"io"
	"os"
)

const (
	DefaultDC          = "dc1"
	DefaultRPCAddr     = "0.0.0.0:8300"
	DefaultRPCPort     = 8000
	DefaultLANSerfPort = 8301
	DefaultWANSerfPort = 8302
)

// Config is used to configure the server
type Config struct {
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
	RPCAddr string

	// SerfLANConfig is the configuration for the intra-dc serf
	SerfLANConfig *serf.Config

	// SerfWANConfig is the configuration for the cross-dc serf
	SerfWANConfig *serf.Config

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer
}

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	conf := &Config{
		Datacenter:    DefaultDC,
		NodeName:      hostname,
		RPCAddr:       DefaultRPCAddr,
		RaftConfig:    raft.DefaultConfig(),
		SerfLANConfig: serf.DefaultConfig(),
		SerfWANConfig: serf.DefaultConfig(),
	}

	// WAN Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	conf.SerfWANConfig.MemberlistConfig = memberlist.DefaultWANConfig()

	// Ensure we don't have port conflicts
	conf.SerfLANConfig.MemberlistConfig.Port = DefaultLANSerfPort
	conf.SerfWANConfig.MemberlistConfig.Port = DefaultWANSerfPort

	return conf
}
