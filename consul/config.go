package consul

import (
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"io"
	"os"
)

const (
	DefaultRPCAddr     = "0.0.0.0:8300"
	DefaultRaftAddr    = "0.0.0.0:8301"
	DefaultLANSerfPort = 8302
	DefaultWANSerfPort = 8303
)

// Config is used to configure the server
type Config struct {
	// Datacenter is the datacenter this Consul server represents
	Datacenter string

	// DataDir is the directory to store our state in
	DataDir string

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// Bind address for Raft (TCP)
	RaftBindAddr string

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// RPCAddr is the RPC address used by Consul. This should be reachable
	// by the WAN and LAN
	RPCAddr string

	// SerfLocalConfig is the configuration for the local serf
	SerfLocalConfig *serf.Config

	// SerfRemoteConfig is the configuration for the remtoe serf
	SerfRemoteConfig *serf.Config

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
		Datacenter:       "dc1",
		NodeName:         hostname,
		RaftBindAddr:     DefaultRaftAddr,
		RPCAddr:          DefaultRPCAddr,
		RaftConfig:       raft.DefaultConfig(),
		SerfLocalConfig:  serf.DefaultConfig(),
		SerfRemoteConfig: serf.DefaultConfig(),
	}

	// Remote Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	conf.SerfRemoteConfig.MemberlistConfig = memberlist.DefaultWANConfig()

	// Ensure we don't have port conflicts
	conf.SerfLocalConfig.MemberlistConfig.Port = DefaultLANSerfPort
	conf.SerfRemoteConfig.MemberlistConfig.Port = DefaultWANSerfPort

	return conf
}
