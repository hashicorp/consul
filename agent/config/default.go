package config

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/version"
)

func DefaultRPCProtocol() (int, error) {
	src := DefaultSource()
	c, err := Parse(src.Data, src.Format)
	if err != nil {
		return 0, fmt.Errorf("Error parsing default config: %s", err)
	}
	if c.RPCProtocol == nil {
		return 0, fmt.Errorf("No default RPC protocol set")
	}
	return *c.RPCProtocol, nil
}

// DefaultSource is the default agent configuration.
// This needs to be merged first in the head.
// todo(fs): The values are sourced from multiple sources.
// todo(fs): IMO, this should be the definitive default for all configurable values
// todo(fs): and whatever is in here should clobber every default value. Hence, no sourcing.
func DefaultSource() Source {
	return Source{
		Name:   "default",
		Format: "hcl",
		Data: `
		acl_default_policy = "allow"
		acl_down_policy = "extend-cache"
		acl_enforce_version_8 = true
		acl_ttl = "30s"
		bind_addr = "0.0.0.0"
		bootstrap = false
		bootstrap_expect = 0
		check_update_interval = "5m"
		client_addr = "127.0.0.1"
		datacenter = "` + consul.DefaultDC + `"
		disable_coordinates = false
		disable_host_node_id = true
		disable_remote_exec = true
		domain = "consul."
		encrypt_verify_incoming = true
		encrypt_verify_outgoing = true
		log_level = "INFO"
		protocol =  2
		retry_interval = "30s"
		retry_interval_wan = "30s"
		server = false
		syslog_facility = "LOCAL0"
		tls_min_version = "tls10"

		dns_config = {
			allow_stale = true
			udp_answer_limit = 3
			max_stale = "87600h"
			recursor_timeout = "2s"
		}
		limits = {
			rpc_rate = -1
			rpc_max_burst = 1000
		}
		performance = {
			leave_drain_time = "5s"
			raft_multiplier = ` + strconv.Itoa(int(consul.DefaultRaftMultiplier)) + `
			rpc_hold_timeout = "7s"
		}
		ports = {
			dns = 8600
			http = 8500
			https = -1
			serf_lan = ` + strconv.Itoa(consul.DefaultLANSerfPort) + `
			serf_wan = ` + strconv.Itoa(consul.DefaultWANSerfPort) + `
			server = ` + strconv.Itoa(consul.DefaultRPCPort) + `
		}
		telemetry = {
			metrics_prefix = "consul"
			filter_default = true
		}
	`,
	}
}

// DevSource is the additional default configuration for dev mode.
// This should be merged in the head after the default configuration.
func DevSource() Source {
	return Source{
		Name:   "dev",
		Format: "hcl",
		Data: `
		bind_addr = "127.0.0.1"
		disable_anonymous_signature = true
		disable_keyring_file = true
		enable_debug = true
		ui = true
		log_level = "DEBUG"
		server = true
		performance = {
			raft_multiplier = 1
		}
	`,
	}
}

// NonUserSource contains the values the user cannot configure.
// This needs to be merged in the tail.
func NonUserSource() Source {
	return Source{
		Name:   "non-user",
		Format: "hcl",
		Data: `
		acl_disabled_ttl = "120s"
		check_deregister_interval_min = "1m"
		check_reap_interval = "30s"
		ae_interval = "1m"
		sync_coordinate_rate_target = 64
		sync_coordinate_interval_min = "15s"

		# segment_limit is the maximum number of network segments that may be declared.
		segment_limit = 64

		# SegmentNameLimit is the maximum segment name length.
		segment_name_limit = 64
	`,
	}
}

// VersionSource creates a config source for the version parameters.
// This should be merged in the tail since these values are not
// user configurable.
func VersionSource(rev, ver, verPre string) Source {
	return Source{
		Name:   "version",
		Format: "hcl",
		Data:   fmt.Sprintf(`revision = %q version = %q version_prerelease = %q`, rev, ver, verPre),
	}
}

// DefaultVersionSource returns the version config source for the embedded
// version numbers.
func DefaultVersionSource() Source {
	return VersionSource(version.GitCommit, version.Version, version.VersionPrerelease)
}

// DefaultConsulSource returns the default configuration for the consul agent.
// This should be merged in the tail since these values are not user configurable.
func DefaultConsulSource() Source {
	cfg := consul.DefaultConfig()
	raft := cfg.RaftConfig
	serfLAN := cfg.SerfLANConfig.MemberlistConfig
	serfWAN := cfg.SerfWANConfig.MemberlistConfig
	return Source{
		Name:   "consul",
		Format: "hcl",
		Data: `
		consul = {
			coordinate = {
				update_batch_size = ` + strconv.Itoa(cfg.CoordinateUpdateBatchSize) + `
				update_max_batches = ` + strconv.Itoa(cfg.CoordinateUpdateMaxBatches) + `
				update_period = "` + cfg.CoordinateUpdatePeriod.String() + `"
			}
			raft = {
				election_timeout = "` + raft.ElectionTimeout.String() + `"
				heartbeat_timeout = "` + raft.HeartbeatTimeout.String() + `"
				leader_lease_timeout = "` + raft.LeaderLeaseTimeout.String() + `"
			}
			serf_lan = {
				memberlist = {
					gossip_interval = "` + serfLAN.GossipInterval.String() + `"
					probe_interval = "` + serfLAN.ProbeInterval.String() + `"
					probe_timeout = "` + serfLAN.ProbeTimeout.String() + `"
					suspicion_mult = ` + strconv.Itoa(serfLAN.SuspicionMult) + `
				}
			}
			serf_wan = {
				memberlist = {
					gossip_interval = "` + serfWAN.GossipInterval.String() + `"
					probe_interval = "` + serfWAN.ProbeInterval.String() + `"
					probe_timeout = "` + serfWAN.ProbeTimeout.String() + `"
					suspicion_mult = ` + strconv.Itoa(serfWAN.SuspicionMult) + `
				}
			}
			server = {
				health_interval = "` + cfg.ServerHealthInterval.String() + `"
			}
		}
	`,
	}
}

// DevConsulSource returns the consul agent configuration for the dev mode.
// This should be merged in the tail after the DefaultConsulSource.
func DevConsulSource() Source {
	return Source{
		Name:   "consul-dev",
		Format: "hcl",
		Data: `
		consul = {
			coordinate = {
				update_period = "100ms"
			}
			raft = {
				election_timeout = "52ms"
				heartbeat_timeout = "35ms"
				leader_lease_timeout = "20ms"
			}
			serf_lan = {
				memberlist = {
					gossip_interval = "100ms"
					probe_interval = "100ms"
					probe_timeout = "100ms"
					suspicion_mult = 3
				}
			}
			serf_wan = {
				memberlist = {
					gossip_interval = "100ms"
					probe_interval = "100ms"
					probe_timeout = "100ms"
					suspicion_mult = 3
				}
			}
			server = {
				health_interval = "10ms"
			}
		}
	`,
	}
}

func DefaultRuntimeConfig(hcl string) *RuntimeConfig {
	b, err := NewBuilder(Flags{HCL: []string{hcl}})
	if err != nil {
		panic(err)
	}
	rt, err := b.BuildAndValidate()
	if err != nil {
		panic(err)
	}
	return &rt
}
