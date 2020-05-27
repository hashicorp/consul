package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/lib/decode"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"
)

const (
	SerfLANKeyring = "serf/local.keyring"
	SerfWANKeyring = "serf/remote.keyring"
)

type Source struct {
	Name   string
	Format string
	Data   string
}

func FormatFrom(name string) string {
	switch {
	case strings.HasSuffix(name, ".json"):
		return "json"
	case strings.HasSuffix(name, ".hcl"):
		return "hcl"
	default:
		return ""
	}
}

// Parse parses a config fragment in either JSON or HCL format.
func Parse(data string, format string) (c Config, keys []string, err error) {
	var raw map[string]interface{}
	switch format {
	case "json":
		err = json.Unmarshal([]byte(data), &raw)
	case "hcl":
		err = hcl.Decode(&raw, data)
	default:
		err = fmt.Errorf("invalid format: %s", format)
	}
	if err != nil {
		return Config{}, nil, err
	}

	var md mapstructure.Metadata
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			decode.HookWeakDecodeFromSlice, // TODO: only apply when format is hcl
			decode.HookTranslateKeys,
		),
		Metadata: &md,
		Result:   &c,
	})
	if err != nil {
		return Config{}, nil, err
	}
	if err := d.Decode(raw); err != nil {
		return Config{}, nil, err
	}

	for _, k := range md.Unused {
		err = multierror.Append(err, fmt.Errorf("invalid config key %s", k))
	}

	// Don't check these here. The builder can emit warnings for fields it
	// doesn't like
	keys = md.Keys

	return
}

// Config defines the format of a configuration file in either JSON or
// HCL format.
//
// It must contain only pointer values, slices and maps to support
// standardized merging of multiple Config structs into one.
//
// Since this is the format which users use to specify their
// configuration it should be treated as an external API which cannot be
// changed and refactored at will since this will break existing setups.
type Config struct {
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentMasterToken *string `json:"acl_agent_master_token,omitempty" hcl:"acl_agent_master_token" mapstructure:"acl_agent_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLAgentToken *string `json:"acl_agent_token,omitempty" hcl:"acl_agent_token" mapstructure:"acl_agent_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved to "primary_datacenter"
	ACLDatacenter *string `json:"acl_datacenter,omitempty" hcl:"acl_datacenter" mapstructure:"acl_datacenter"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLDefaultPolicy *string `json:"acl_default_policy,omitempty" hcl:"acl_default_policy" mapstructure:"acl_default_policy"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLDownPolicy *string `json:"acl_down_policy,omitempty" hcl:"acl_down_policy" mapstructure:"acl_down_policy"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLEnableKeyListPolicy *bool `json:"acl_enable_key_list_policy,omitempty" hcl:"acl_enable_key_list_policy" mapstructure:"acl_enable_key_list_policy"`
	// DEPRECATED (ACL-Legacy-Compat) -  pre-version8 enforcement is deprecated.
	ACLEnforceVersion8 *bool `json:"acl_enforce_version_8,omitempty" hcl:"acl_enforce_version_8" mapstructure:"acl_enforce_version_8"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLMasterToken *string `json:"acl_master_token,omitempty" hcl:"acl_master_token" mapstructure:"acl_master_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLReplicationToken *string `json:"acl_replication_token,omitempty" hcl:"acl_replication_token" mapstructure:"acl_replication_token"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLTTL *string `json:"acl_ttl,omitempty" hcl:"acl_ttl" mapstructure:"acl_ttl"`
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl.tokens" stanza
	ACLToken                         *string                  `json:"acl_token,omitempty" hcl:"acl_token" mapstructure:"acl_token"`
	ACL                              ACL                      `json:"acl,omitempty" hcl:"acl" mapstructure:"acl"`
	Addresses                        Addresses                `json:"addresses,omitempty" hcl:"addresses" mapstructure:"addresses"`
	AdvertiseAddrLAN                 *string                  `json:"advertise_addr,omitempty" hcl:"advertise_addr" mapstructure:"advertise_addr"`
	AdvertiseAddrLANIPv4             *string                  `json:"advertise_addr_ipv4,omitempty" hcl:"advertise_addr_ipv4" mapstructure:"advertise_addr_ipv4"`
	AdvertiseAddrLANIPv6             *string                  `json:"advertise_addr_ipv6,omitempty" hcl:"advertise_addr_ipv6" mapstructure:"advertise_addr_ipv6"`
	AdvertiseAddrWAN                 *string                  `json:"advertise_addr_wan,omitempty" hcl:"advertise_addr_wan" mapstructure:"advertise_addr_wan"`
	AdvertiseAddrWANIPv4             *string                  `json:"advertise_addr_wan_ipv4,omitempty" hcl:"advertise_addr_wan_ipv4" mapstructure:"advertise_addr_wan_ipv4"`
	AdvertiseAddrWANIPv6             *string                  `json:"advertise_addr_wan_ipv6,omitempty" hcl:"advertise_addr_wan_ipv6" mapstructure:"advertise_addr_ipv6"`
	Autopilot                        Autopilot                `json:"autopilot,omitempty" hcl:"autopilot" mapstructure:"autopilot"`
	BindAddr                         *string                  `json:"bind_addr,omitempty" hcl:"bind_addr" mapstructure:"bind_addr"`
	Bootstrap                        *bool                    `json:"bootstrap,omitempty" hcl:"bootstrap" mapstructure:"bootstrap"`
	BootstrapExpect                  *int                     `json:"bootstrap_expect,omitempty" hcl:"bootstrap_expect" mapstructure:"bootstrap_expect"`
	CAFile                           *string                  `json:"ca_file,omitempty" hcl:"ca_file" mapstructure:"ca_file"`
	CAPath                           *string                  `json:"ca_path,omitempty" hcl:"ca_path" mapstructure:"ca_path"`
	CertFile                         *string                  `json:"cert_file,omitempty" hcl:"cert_file" mapstructure:"cert_file"`
	Check                            *CheckDefinition         `json:"check,omitempty" hcl:"check" mapstructure:"check"` // needs to be a pointer to avoid partial merges
	CheckOutputMaxSize               *int                     `json:"check_output_max_size,omitempty" hcl:"check_output_max_size" mapstructure:"check_output_max_size"`
	CheckUpdateInterval              *string                  `json:"check_update_interval,omitempty" hcl:"check_update_interval" mapstructure:"check_update_interval"`
	Checks                           []CheckDefinition        `json:"checks,omitempty" hcl:"checks" mapstructure:"checks"`
	ClientAddr                       *string                  `json:"client_addr,omitempty" hcl:"client_addr" mapstructure:"client_addr"`
	ConfigEntries                    ConfigEntries            `json:"config_entries,omitempty" hcl:"config_entries" mapstructure:"config_entries"`
	AutoEncrypt                      AutoEncrypt              `json:"auto_encrypt,omitempty" hcl:"auto_encrypt" mapstructure:"auto_encrypt"`
	Connect                          Connect                  `json:"connect,omitempty" hcl:"connect" mapstructure:"connect"`
	DNS                              DNS                      `json:"dns_config,omitempty" hcl:"dns_config" mapstructure:"dns_config"`
	DNSDomain                        *string                  `json:"domain,omitempty" hcl:"domain" mapstructure:"domain"`
	DNSAltDomain                     *string                  `json:"alt_domain,omitempty" hcl:"alt_domain" mapstructure:"alt_domain"`
	DNSRecursors                     []string                 `json:"recursors,omitempty" hcl:"recursors" mapstructure:"recursors"`
	DataDir                          *string                  `json:"data_dir,omitempty" hcl:"data_dir" mapstructure:"data_dir"`
	Datacenter                       *string                  `json:"datacenter,omitempty" hcl:"datacenter" mapstructure:"datacenter"`
	DefaultQueryTime                 *string                  `json:"default_query_time,omitempty" hcl:"default_query_time" mapstructure:"default_query_time"`
	DisableAnonymousSignature        *bool                    `json:"disable_anonymous_signature,omitempty" hcl:"disable_anonymous_signature" mapstructure:"disable_anonymous_signature"`
	DisableCoordinates               *bool                    `json:"disable_coordinates,omitempty" hcl:"disable_coordinates" mapstructure:"disable_coordinates"`
	DisableHostNodeID                *bool                    `json:"disable_host_node_id,omitempty" hcl:"disable_host_node_id" mapstructure:"disable_host_node_id"`
	DisableHTTPUnprintableCharFilter *bool                    `json:"disable_http_unprintable_char_filter,omitempty" hcl:"disable_http_unprintable_char_filter" mapstructure:"disable_http_unprintable_char_filter"`
	DisableKeyringFile               *bool                    `json:"disable_keyring_file,omitempty" hcl:"disable_keyring_file" mapstructure:"disable_keyring_file"`
	DisableRemoteExec                *bool                    `json:"disable_remote_exec,omitempty" hcl:"disable_remote_exec" mapstructure:"disable_remote_exec"`
	DisableUpdateCheck               *bool                    `json:"disable_update_check,omitempty" hcl:"disable_update_check" mapstructure:"disable_update_check"`
	DiscardCheckOutput               *bool                    `json:"discard_check_output" hcl:"discard_check_output" mapstructure:"discard_check_output"`
	DiscoveryMaxStale                *string                  `json:"discovery_max_stale" hcl:"discovery_max_stale" mapstructure:"discovery_max_stale"`
	EnableACLReplication             *bool                    `json:"enable_acl_replication,omitempty" hcl:"enable_acl_replication" mapstructure:"enable_acl_replication"`
	EnableAgentTLSForChecks          *bool                    `json:"enable_agent_tls_for_checks,omitempty" hcl:"enable_agent_tls_for_checks" mapstructure:"enable_agent_tls_for_checks"`
	EnableCentralServiceConfig       *bool                    `json:"enable_central_service_config,omitempty" hcl:"enable_central_service_config" mapstructure:"enable_central_service_config"`
	EnableDebug                      *bool                    `json:"enable_debug,omitempty" hcl:"enable_debug" mapstructure:"enable_debug"`
	EnableScriptChecks               *bool                    `json:"enable_script_checks,omitempty" hcl:"enable_script_checks" mapstructure:"enable_script_checks"`
	EnableLocalScriptChecks          *bool                    `json:"enable_local_script_checks,omitempty" hcl:"enable_local_script_checks" mapstructure:"enable_local_script_checks"`
	EnableSyslog                     *bool                    `json:"enable_syslog,omitempty" hcl:"enable_syslog" mapstructure:"enable_syslog"`
	EncryptKey                       *string                  `json:"encrypt,omitempty" hcl:"encrypt" mapstructure:"encrypt"`
	EncryptVerifyIncoming            *bool                    `json:"encrypt_verify_incoming,omitempty" hcl:"encrypt_verify_incoming" mapstructure:"encrypt_verify_incoming"`
	EncryptVerifyOutgoing            *bool                    `json:"encrypt_verify_outgoing,omitempty" hcl:"encrypt_verify_outgoing" mapstructure:"encrypt_verify_outgoing"`
	GossipLAN                        GossipLANConfig          `json:"gossip_lan,omitempty" hcl:"gossip_lan" mapstructure:"gossip_lan"`
	GossipWAN                        GossipWANConfig          `json:"gossip_wan,omitempty" hcl:"gossip_wan" mapstructure:"gossip_wan"`
	HTTPConfig                       HTTPConfig               `json:"http_config,omitempty" hcl:"http_config" mapstructure:"http_config"`
	KeyFile                          *string                  `json:"key_file,omitempty" hcl:"key_file" mapstructure:"key_file"`
	LeaveOnTerm                      *bool                    `json:"leave_on_terminate,omitempty" hcl:"leave_on_terminate" mapstructure:"leave_on_terminate"`
	Limits                           Limits                   `json:"limits,omitempty" hcl:"limits" mapstructure:"limits"`
	LogLevel                         *string                  `json:"log_level,omitempty" hcl:"log_level" mapstructure:"log_level"`
	LogJSON                          *bool                    `json:"log_json,omitempty" hcl:"log_json" mapstructure:"log_json"`
	LogFile                          *string                  `json:"log_file,omitempty" hcl:"log_file" mapstructure:"log_file"`
	LogRotateDuration                *string                  `json:"log_rotate_duration,omitempty" hcl:"log_rotate_duration" mapstructure:"log_rotate_duration"`
	LogRotateBytes                   *int                     `json:"log_rotate_bytes,omitempty" hcl:"log_rotate_bytes" mapstructure:"log_rotate_bytes"`
	LogRotateMaxFiles                *int                     `json:"log_rotate_max_files,omitempty" hcl:"log_rotate_max_files" mapstructure:"log_rotate_max_files"`
	MaxQueryTime                     *string                  `json:"max_query_time,omitempty" hcl:"max_query_time" mapstructure:"max_query_time"`
	NodeID                           *string                  `json:"node_id,omitempty" hcl:"node_id" mapstructure:"node_id"`
	NodeMeta                         map[string]string        `json:"node_meta,omitempty" hcl:"node_meta" mapstructure:"node_meta"`
	NodeName                         *string                  `json:"node_name,omitempty" hcl:"node_name" mapstructure:"node_name"`
	Performance                      Performance              `json:"performance,omitempty" hcl:"performance" mapstructure:"performance"`
	PidFile                          *string                  `json:"pid_file,omitempty" hcl:"pid_file" mapstructure:"pid_file"`
	Ports                            Ports                    `json:"ports,omitempty" hcl:"ports" mapstructure:"ports"`
	PrimaryDatacenter                *string                  `json:"primary_datacenter,omitempty" hcl:"primary_datacenter" mapstructure:"primary_datacenter"`
	PrimaryGateways                  []string                 `json:"primary_gateways" hcl:"primary_gateways" mapstructure:"primary_gateways"`
	PrimaryGatewaysInterval          *string                  `json:"primary_gateways_interval,omitempty" hcl:"primary_gateways_interval" mapstructure:"primary_gateways_interval"`
	RPCProtocol                      *int                     `json:"protocol,omitempty" hcl:"protocol" mapstructure:"protocol"`
	RaftProtocol                     *int                     `json:"raft_protocol,omitempty" hcl:"raft_protocol" mapstructure:"raft_protocol"`
	RaftSnapshotThreshold            *int                     `json:"raft_snapshot_threshold,omitempty" hcl:"raft_snapshot_threshold" mapstructure:"raft_snapshot_threshold"`
	RaftSnapshotInterval             *string                  `json:"raft_snapshot_interval,omitempty" hcl:"raft_snapshot_interval" mapstructure:"raft_snapshot_interval"`
	RaftTrailingLogs                 *int                     `json:"raft_trailing_logs,omitempty" hcl:"raft_trailing_logs" mapstructure:"raft_trailing_logs"`
	ReconnectTimeoutLAN              *string                  `json:"reconnect_timeout,omitempty" hcl:"reconnect_timeout" mapstructure:"reconnect_timeout"`
	ReconnectTimeoutWAN              *string                  `json:"reconnect_timeout_wan,omitempty" hcl:"reconnect_timeout_wan" mapstructure:"reconnect_timeout_wan"`
	RejoinAfterLeave                 *bool                    `json:"rejoin_after_leave,omitempty" hcl:"rejoin_after_leave" mapstructure:"rejoin_after_leave"`
	RetryJoinIntervalLAN             *string                  `json:"retry_interval,omitempty" hcl:"retry_interval" mapstructure:"retry_interval"`
	RetryJoinIntervalWAN             *string                  `json:"retry_interval_wan,omitempty" hcl:"retry_interval_wan" mapstructure:"retry_interval_wan"`
	RetryJoinLAN                     []string                 `json:"retry_join,omitempty" hcl:"retry_join" mapstructure:"retry_join"`
	RetryJoinMaxAttemptsLAN          *int                     `json:"retry_max,omitempty" hcl:"retry_max" mapstructure:"retry_max"`
	RetryJoinMaxAttemptsWAN          *int                     `json:"retry_max_wan,omitempty" hcl:"retry_max_wan" mapstructure:"retry_max_wan"`
	RetryJoinWAN                     []string                 `json:"retry_join_wan,omitempty" hcl:"retry_join_wan" mapstructure:"retry_join_wan"`
	SerfAllowedCIDRsLAN              []string                 `json:"serf_lan_allowed_cidrs,omitempty" hcl:"serf_lan_allowed_cidrs" mapstructure:"serf_lan_allowed_cidrs"`
	SerfAllowedCIDRsWAN              []string                 `json:"serf_wan_allowed_cidrs,omitempty" hcl:"serf_wan_allowed_cidrs" mapstructure:"serf_wan_allowed_cidrs"`
	SerfBindAddrLAN                  *string                  `json:"serf_lan,omitempty" hcl:"serf_lan" mapstructure:"serf_lan"`
	SerfBindAddrWAN                  *string                  `json:"serf_wan,omitempty" hcl:"serf_wan" mapstructure:"serf_wan"`
	ServerMode                       *bool                    `json:"server,omitempty" hcl:"server" mapstructure:"server"`
	ServerName                       *string                  `json:"server_name,omitempty" hcl:"server_name" mapstructure:"server_name"`
	Service                          *ServiceDefinition       `json:"service,omitempty" hcl:"service" mapstructure:"service"`
	Services                         []ServiceDefinition      `json:"services,omitempty" hcl:"services" mapstructure:"services"`
	SessionTTLMin                    *string                  `json:"session_ttl_min,omitempty" hcl:"session_ttl_min" mapstructure:"session_ttl_min"`
	SkipLeaveOnInt                   *bool                    `json:"skip_leave_on_interrupt,omitempty" hcl:"skip_leave_on_interrupt" mapstructure:"skip_leave_on_interrupt"`
	StartJoinAddrsLAN                []string                 `json:"start_join,omitempty" hcl:"start_join" mapstructure:"start_join"`
	StartJoinAddrsWAN                []string                 `json:"start_join_wan,omitempty" hcl:"start_join_wan" mapstructure:"start_join_wan"`
	SyslogFacility                   *string                  `json:"syslog_facility,omitempty" hcl:"syslog_facility" mapstructure:"syslog_facility"`
	TLSCipherSuites                  *string                  `json:"tls_cipher_suites,omitempty" hcl:"tls_cipher_suites" mapstructure:"tls_cipher_suites"`
	TLSMinVersion                    *string                  `json:"tls_min_version,omitempty" hcl:"tls_min_version" mapstructure:"tls_min_version"`
	TLSPreferServerCipherSuites      *bool                    `json:"tls_prefer_server_cipher_suites,omitempty" hcl:"tls_prefer_server_cipher_suites" mapstructure:"tls_prefer_server_cipher_suites"`
	TaggedAddresses                  map[string]string        `json:"tagged_addresses,omitempty" hcl:"tagged_addresses" mapstructure:"tagged_addresses"`
	Telemetry                        Telemetry                `json:"telemetry,omitempty" hcl:"telemetry" mapstructure:"telemetry"`
	TranslateWANAddrs                *bool                    `json:"translate_wan_addrs,omitempty" hcl:"translate_wan_addrs" mapstructure:"translate_wan_addrs"`
	UI                               *bool                    `json:"ui,omitempty" hcl:"ui" mapstructure:"ui"`
	UIContentPath                    *string                  `json:"ui_content_path,omitempty" hcl:"ui_content_path" mapstructure:"ui_content_path"`
	UIDir                            *string                  `json:"ui_dir,omitempty" hcl:"ui_dir" mapstructure:"ui_dir"`
	UnixSocket                       UnixSocket               `json:"unix_sockets,omitempty" hcl:"unix_sockets" mapstructure:"unix_sockets"`
	VerifyIncoming                   *bool                    `json:"verify_incoming,omitempty" hcl:"verify_incoming" mapstructure:"verify_incoming"`
	VerifyIncomingHTTPS              *bool                    `json:"verify_incoming_https,omitempty" hcl:"verify_incoming_https" mapstructure:"verify_incoming_https"`
	VerifyIncomingRPC                *bool                    `json:"verify_incoming_rpc,omitempty" hcl:"verify_incoming_rpc" mapstructure:"verify_incoming_rpc"`
	VerifyOutgoing                   *bool                    `json:"verify_outgoing,omitempty" hcl:"verify_outgoing" mapstructure:"verify_outgoing"`
	VerifyServerHostname             *bool                    `json:"verify_server_hostname,omitempty" hcl:"verify_server_hostname" mapstructure:"verify_server_hostname"`
	Watches                          []map[string]interface{} `json:"watches,omitempty" hcl:"watches" mapstructure:"watches"`

	// This isn't used by Consul but we've documented a feature where users
	// can deploy their snapshot agent configs alongside their Consul configs
	// so we have a placeholder here so it can be parsed but this doesn't
	// manifest itself in any way inside the runtime config.
	SnapshotAgent map[string]interface{} `json:"snapshot_agent,omitempty" hcl:"snapshot_agent" mapstructure:"snapshot_agent"`

	// non-user configurable values
	// DEPRECATED (ACL-Legacy-Compat) - moved into the "acl" stanza
	ACLDisabledTTL             *string  `json:"acl_disabled_ttl,omitempty" hcl:"acl_disabled_ttl" mapstructure:"acl_disabled_ttl"`
	AEInterval                 *string  `json:"ae_interval,omitempty" hcl:"ae_interval" mapstructure:"ae_interval"`
	CheckDeregisterIntervalMin *string  `json:"check_deregister_interval_min,omitempty" hcl:"check_deregister_interval_min" mapstructure:"check_deregister_interval_min"`
	CheckReapInterval          *string  `json:"check_reap_interval,omitempty" hcl:"check_reap_interval" mapstructure:"check_reap_interval"`
	Consul                     Consul   `json:"consul,omitempty" hcl:"consul" mapstructure:"consul"`
	Revision                   *string  `json:"revision,omitempty" hcl:"revision" mapstructure:"revision"`
	SegmentLimit               *int     `json:"segment_limit,omitempty" hcl:"segment_limit" mapstructure:"segment_limit"`
	SegmentNameLimit           *int     `json:"segment_name_limit,omitempty" hcl:"segment_name_limit" mapstructure:"segment_name_limit"`
	SyncCoordinateIntervalMin  *string  `json:"sync_coordinate_interval_min,omitempty" hcl:"sync_coordinate_interval_min" mapstructure:"sync_coordinate_interval_min"`
	SyncCoordinateRateTarget   *float64 `json:"sync_coordinate_rate_target,omitempty" hcl:"sync_coordinate_rate_target" mapstructure:"sync_coordinate_rate_target"`
	Version                    *string  `json:"version,omitempty" hcl:"version" mapstructure:"version"`
	VersionPrerelease          *string  `json:"version_prerelease,omitempty" hcl:"version_prerelease" mapstructure:"version_prerelease"`

	// Enterprise Only
	Audit *Audit `json:"audit,omitempty" hcl:"audit" mapstructure:"audit"`
	// Enterprise Only
	NonVotingServer *bool `json:"non_voting_server,omitempty" hcl:"non_voting_server" mapstructure:"non_voting_server"`
	// Enterprise Only
	SegmentName *string `json:"segment,omitempty" hcl:"segment" mapstructure:"segment"`
	// Enterprise Only
	Segments []Segment `json:"segments,omitempty" hcl:"segments" mapstructure:"segments"`
}

type GossipLANConfig struct {
	GossipNodes    *int    `json:"gossip_nodes,omitempty" hcl:"gossip_nodes" mapstructure:"gossip_nodes"`
	GossipInterval *string `json:"gossip_interval,omitempty" hcl:"gossip_interval" mapstructure:"gossip_interval"`
	ProbeInterval  *string `json:"probe_interval,omitempty" hcl:"probe_interval" mapstructure:"probe_interval"`
	ProbeTimeout   *string `json:"probe_timeout,omitempty" hcl:"probe_timeout" mapstructure:"probe_timeout"`
	SuspicionMult  *int    `json:"suspicion_mult,omitempty" hcl:"suspicion_mult" mapstructure:"suspicion_mult"`
	RetransmitMult *int    `json:"retransmit_mult,omitempty" hcl:"retransmit_mult" mapstructure:"retransmit_mult"`
}

type GossipWANConfig struct {
	GossipNodes    *int    `json:"gossip_nodes,omitempty" hcl:"gossip_nodes" mapstructure:"gossip_nodes"`
	GossipInterval *string `json:"gossip_interval,omitempty" hcl:"gossip_interval" mapstructure:"gossip_interval"`
	ProbeInterval  *string `json:"probe_interval,omitempty" hcl:"probe_interval" mapstructure:"probe_interval"`
	ProbeTimeout   *string `json:"probe_timeout,omitempty" hcl:"probe_timeout" mapstructure:"probe_timeout"`
	SuspicionMult  *int    `json:"suspicion_mult,omitempty" hcl:"suspicion_mult" mapstructure:"suspicion_mult"`
	RetransmitMult *int    `json:"retransmit_mult,omitempty" hcl:"retransmit_mult" mapstructure:"retransmit_mult"`
}

type Consul struct {
	Coordinate struct {
		UpdateBatchSize  *int    `json:"update_batch_size,omitempty" hcl:"update_batch_size" mapstructure:"update_batch_size"`
		UpdateMaxBatches *int    `json:"update_max_batches,omitempty" hcl:"update_max_batches" mapstructure:"update_max_batches"`
		UpdatePeriod     *string `json:"update_period,omitempty" hcl:"update_period" mapstructure:"update_period"`
	} `json:"coordinate,omitempty" hcl:"coordinate" mapstructure:"coordinate"`

	Raft struct {
		ElectionTimeout    *string `json:"election_timeout,omitempty" hcl:"election_timeout" mapstructure:"election_timeout"`
		HeartbeatTimeout   *string `json:"heartbeat_timeout,omitempty" hcl:"heartbeat_timeout" mapstructure:"heartbeat_timeout"`
		LeaderLeaseTimeout *string `json:"leader_lease_timeout,omitempty" hcl:"leader_lease_timeout" mapstructure:"leader_lease_timeout"`
	} `json:"raft,omitempty" hcl:"raft" mapstructure:"raft"`

	Server struct {
		HealthInterval *string `json:"health_interval,omitempty" hcl:"health_interval" mapstructure:"health_interval"`
	} `json:"server,omitempty" hcl:"server" mapstructure:"server"`
}

type Addresses struct {
	DNS   *string `json:"dns,omitempty" hcl:"dns" mapstructure:"dns"`
	HTTP  *string `json:"http,omitempty" hcl:"http" mapstructure:"http"`
	HTTPS *string `json:"https,omitempty" hcl:"https" mapstructure:"https"`
	GRPC  *string `json:"grpc,omitempty" hcl:"grpc" mapstructure:"grpc"`
}

type AdvertiseAddrsConfig struct {
	RPC     *string `json:"rpc,omitempty" hcl:"rpc" mapstructure:"rpc"`
	SerfLAN *string `json:"serf_lan,omitempty" hcl:"serf_lan" mapstructure:"serf_lan"`
	SerfWAN *string `json:"serf_wan,omitempty" hcl:"serf_wan" mapstructure:"serf_wan"`
}

type Autopilot struct {
	CleanupDeadServers      *bool   `json:"cleanup_dead_servers,omitempty" hcl:"cleanup_dead_servers" mapstructure:"cleanup_dead_servers"`
	LastContactThreshold    *string `json:"last_contact_threshold,omitempty" hcl:"last_contact_threshold" mapstructure:"last_contact_threshold"`
	MaxTrailingLogs         *int    `json:"max_trailing_logs,omitempty" hcl:"max_trailing_logs" mapstructure:"max_trailing_logs"`
	MinQuorum               *uint   `json:"min_quorum,omitempty" hcl:"min_quorum" mapstructure:"min_quorum"`
	ServerStabilizationTime *string `json:"server_stabilization_time,omitempty" hcl:"server_stabilization_time" mapstructure:"server_stabilization_time"`

	// Enterprise Only
	DisableUpgradeMigration *bool `json:"disable_upgrade_migration,omitempty" hcl:"disable_upgrade_migration" mapstructure:"disable_upgrade_migration"`
	// Enterprise Only
	RedundancyZoneTag *string `json:"redundancy_zone_tag,omitempty" hcl:"redundancy_zone_tag" mapstructure:"redundancy_zone_tag"`
	// Enterprise Only
	UpgradeVersionTag *string `json:"upgrade_version_tag,omitempty" hcl:"upgrade_version_tag" mapstructure:"upgrade_version_tag"`
}

// ServiceWeights defines the registration of weights used in DNS for a Service
type ServiceWeights struct {
	Passing *int `json:"passing,omitempty" hcl:"passing" mapstructure:"passing"`
	Warning *int `json:"warning,omitempty" hcl:"warning" mapstructure:"warning"`
}

type ServiceAddress struct {
	Address *string `json:"address,omitempty" hcl:"address" mapstructure:"address"`
	Port    *int    `json:"port,omitempty" hcl:"port" mapstructure:"port"`
}

type ServiceDefinition struct {
	Kind              *string                   `json:"kind,omitempty" hcl:"kind" mapstructure:"kind"`
	ID                *string                   `json:"id,omitempty" hcl:"id" mapstructure:"id"`
	Name              *string                   `json:"name,omitempty" hcl:"name" mapstructure:"name"`
	Tags              []string                  `json:"tags,omitempty" hcl:"tags" mapstructure:"tags"`
	Address           *string                   `json:"address,omitempty" hcl:"address" mapstructure:"address"`
	TaggedAddresses   map[string]ServiceAddress `json:"tagged_addresses,omitempty" hcl:"tagged_addresses" mapstructure:"tagged_addresses"`
	Meta              map[string]string         `json:"meta,omitempty" hcl:"meta" mapstructure:"meta"`
	Port              *int                      `json:"port,omitempty" hcl:"port" mapstructure:"port"`
	Check             *CheckDefinition          `json:"check,omitempty" hcl:"check" mapstructure:"check"`
	Checks            []CheckDefinition         `json:"checks,omitempty" hcl:"checks" mapstructure:"checks"`
	Token             *string                   `json:"token,omitempty" hcl:"token" mapstructure:"token"`
	Weights           *ServiceWeights           `json:"weights,omitempty" hcl:"weights" mapstructure:"weights"`
	EnableTagOverride *bool                     `json:"enable_tag_override,omitempty" hcl:"enable_tag_override" mapstructure:"enable_tag_override"`
	Proxy             *ServiceProxy             `json:"proxy,omitempty" hcl:"proxy" mapstructure:"proxy"`
	Connect           *ServiceConnect           `json:"connect,omitempty" hcl:"connect" mapstructure:"connect"`

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

type CheckDefinition struct {
	ID                             *string             `json:"id,omitempty" hcl:"id" mapstructure:"id"`
	Name                           *string             `json:"name,omitempty" hcl:"name" mapstructure:"name"`
	Notes                          *string             `json:"notes,omitempty" hcl:"notes" mapstructure:"notes"`
	ServiceID                      *string             `json:"service_id,omitempty" hcl:"service_id" mapstructure:"service_id" alias:"serviceid"`
	Token                          *string             `json:"token,omitempty" hcl:"token" mapstructure:"token"`
	Status                         *string             `json:"status,omitempty" hcl:"status" mapstructure:"status"`
	ScriptArgs                     []string            `json:"args,omitempty" hcl:"args" mapstructure:"args" alias:"scriptargs"`
	HTTP                           *string             `json:"http,omitempty" hcl:"http" mapstructure:"http"`
	Header                         map[string][]string `json:"header,omitempty" hcl:"header" mapstructure:"header"`
	Method                         *string             `json:"method,omitempty" hcl:"method" mapstructure:"method"`
	Body                           *string             `json:"body,omitempty" hcl:"body" mapstructure:"body"`
	OutputMaxSize                  *int                `json:"output_max_size,omitempty" hcl:"output_max_size" mapstructure:"output_max_size"`
	TCP                            *string             `json:"tcp,omitempty" hcl:"tcp" mapstructure:"tcp"`
	Interval                       *string             `json:"interval,omitempty" hcl:"interval" mapstructure:"interval"`
	DockerContainerID              *string             `json:"docker_container_id,omitempty" hcl:"docker_container_id" mapstructure:"docker_container_id" alias:"dockercontainerid"`
	Shell                          *string             `json:"shell,omitempty" hcl:"shell" mapstructure:"shell"`
	GRPC                           *string             `json:"grpc,omitempty" hcl:"grpc" mapstructure:"grpc"`
	GRPCUseTLS                     *bool               `json:"grpc_use_tls,omitempty" hcl:"grpc_use_tls" mapstructure:"grpc_use_tls"`
	TLSSkipVerify                  *bool               `json:"tls_skip_verify,omitempty" hcl:"tls_skip_verify" mapstructure:"tls_skip_verify" alias:"tlsskipverify"`
	AliasNode                      *string             `json:"alias_node,omitempty" hcl:"alias_node" mapstructure:"alias_node"`
	AliasService                   *string             `json:"alias_service,omitempty" hcl:"alias_service" mapstructure:"alias_service"`
	Timeout                        *string             `json:"timeout,omitempty" hcl:"timeout" mapstructure:"timeout"`
	TTL                            *string             `json:"ttl,omitempty" hcl:"ttl" mapstructure:"ttl"`
	SuccessBeforePassing           *int                `json:"success_before_passing,omitempty" hcl:"success_before_passing" mapstructure:"success_before_passing"`
	FailuresBeforeCritical         *int                `json:"failures_before_critical,omitempty" hcl:"failures_before_critical" mapstructure:"failures_before_critical"`
	DeregisterCriticalServiceAfter *string             `json:"deregister_critical_service_after,omitempty" hcl:"deregister_critical_service_after" mapstructure:"deregister_critical_service_after" alias:"deregistercriticalserviceafter"`

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

// ServiceConnect is the connect block within a service registration
type ServiceConnect struct {
	// Native is true when this service can natively understand Connect.
	Native *bool `json:"native,omitempty" hcl:"native" mapstructure:"native"`

	// SidecarService is a nested Service Definition to register at the same time.
	// It's purely a convenience mechanism to allow specifying a sidecar service
	// along with the application service definition. It's nested nature allows
	// all of the fields to be defaulted which can reduce the amount of
	// boilerplate needed to register a sidecar service separately, but the end
	// result is identical to just making a second service registration via any
	// other means.
	SidecarService *ServiceDefinition `json:"sidecar_service,omitempty" hcl:"sidecar_service" mapstructure:"sidecar_service"`
}

// ServiceProxy is the additional config needed for a Kind = connect-proxy
// registration.
type ServiceProxy struct {
	// DestinationServiceName is required and is the name of the service to accept
	// traffic for.
	DestinationServiceName *string `json:"destination_service_name,omitempty" hcl:"destination_service_name" mapstructure:"destination_service_name"`

	// DestinationServiceID is optional and should only be specified for
	// "side-car" style proxies where the proxy is in front of just a single
	// instance of the service. It should be set to the service ID of the instance
	// being represented which must be registered to the same agent. It's valid to
	// provide a service ID that does not yet exist to avoid timing issues when
	// bootstrapping a service with a proxy.
	DestinationServiceID *string `json:"destination_service_id,omitempty" hcl:"destination_service_id" mapstructure:"destination_service_id"`

	// LocalServiceAddress is the address of the local service instance. It is
	// optional and should only be specified for "side-car" style proxies. It will
	// default to 127.0.0.1 if the proxy is a "side-car" (DestinationServiceID is
	// set) but otherwise will be ignored.
	LocalServiceAddress *string `json:"local_service_address,omitempty" hcl:"local_service_address" mapstructure:"local_service_address"`

	// LocalServicePort is the port of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies. It will default
	// to the registered port for the instance if the proxy is a "side-car"
	// (DestinationServiceID is set) but otherwise will be ignored.
	LocalServicePort *int `json:"local_service_port,omitempty" hcl:"local_service_port" mapstructure:"local_service_port"`

	// Config is the arbitrary configuration data provided with the proxy
	// registration.
	Config map[string]interface{} `json:"config,omitempty" hcl:"config" mapstructure:"config"`

	// Upstreams describes any upstream dependencies the proxy instance should
	// setup.
	Upstreams []Upstream `json:"upstreams,omitempty" hcl:"upstreams" mapstructure:"upstreams"`

	// Mesh Gateway Configuration
	MeshGateway *MeshGatewayConfig `json:"mesh_gateway,omitempty" hcl:"mesh_gateway" mapstructure:"mesh_gateway"`

	// Expose defines whether checks or paths are exposed through the proxy
	Expose *ExposeConfig `json:"expose,omitempty" hcl:"expose" mapstructure:"expose"`
}

// Upstream represents a single upstream dependency for a service or proxy. It
// describes the mechanism used to discover instances to communicate with (the
// Target) as well as any potential client configuration that may be useful such
// as load balancer options, timeouts etc.
type Upstream struct {
	// Destination fields are the required ones for determining what this upstream
	// points to. Depending on DestinationType some other fields below might
	// further restrict the set of instances allowable.
	//
	// DestinationType would be better as an int constant but even with custom
	// JSON marshallers it causes havoc with all the mapstructure mangling we do
	// on service definitions in various places.
	DestinationType      *string `json:"destination_type,omitempty" hcl:"destination_type" mapstructure:"destination_type"`
	DestinationNamespace *string `json:"destination_namespace,omitempty" hcl:"destination_namespace" mapstructure:"destination_namespace"`
	DestinationName      *string `json:"destination_name,omitempty" hcl:"destination_name" mapstructure:"destination_name"`

	// Datacenter that the service discovery request should be run against. Note
	// for prepared queries, the actual results might be from a different
	// datacenter.
	Datacenter *string `json:"datacenter,omitempty" hcl:"datacenter" mapstructure:"datacenter"`

	// LocalBindAddress is the ip address a side-car proxy should listen on for
	// traffic destined for this upstream service. Default if empty is 127.0.0.1.
	LocalBindAddress *string `json:"local_bind_address,omitempty" hcl:"local_bind_address" mapstructure:"local_bind_address"`

	// LocalBindPort is the ip address a side-car proxy should listen on for traffic
	// destined for this upstream service. Required.
	LocalBindPort *int `json:"local_bind_port,omitempty" hcl:"local_bind_port" mapstructure:"local_bind_port"`

	// Config is an opaque config that is specific to the proxy process being run.
	// It can be used to pass arbitrary configuration for this specific upstream
	// to the proxy.
	Config map[string]interface{} `json:"config,omitempty" hcl:"config" mapstructure:"config"`

	// Mesh Gateway Configuration
	MeshGateway *MeshGatewayConfig `json:"mesh_gateway,omitempty" hcl:"mesh_gateway" mapstructure:"mesh_gateway"`
}

type MeshGatewayConfig struct {
	// Mesh Gateway Mode
	Mode *string `json:"mode,omitempty" hcl:"mode" mapstructure:"mode"`
}

// ExposeConfig describes HTTP paths to expose through Envoy outside of Connect.
// Users can expose individual paths and/or all HTTP/GRPC paths for checks.
type ExposeConfig struct {
	// Checks defines whether paths associated with Consul checks will be exposed.
	// This flag triggers exposing all HTTP and GRPC check paths registered for the service.
	Checks *bool `json:"checks,omitempty" hcl:"checks" mapstructure:"checks"`

	// Port defines the port of the proxy's listener for exposed paths.
	Port *int `json:"port,omitempty" hcl:"port" mapstructure:"port"`

	// Paths is the list of paths exposed through the proxy.
	Paths []ExposePath `json:"paths,omitempty" hcl:"paths" mapstructure:"paths"`
}

type ExposePath struct {
	// ListenerPort defines the port of the proxy's listener for exposed paths.
	ListenerPort *int `json:"listener_port,omitempty" hcl:"listener_port" mapstructure:"listener_port"`

	// Path is the path to expose through the proxy, ie. "/metrics."
	Path *string `json:"path,omitempty" hcl:"path" mapstructure:"path"`

	// Protocol describes the upstream's service protocol.
	Protocol *string `json:"protocol,omitempty" hcl:"protocol" mapstructure:"protocol"`

	// LocalPathPort is the port that the service is listening on for the given path.
	LocalPathPort *int `json:"local_path_port,omitempty" hcl:"local_path_port" mapstructure:"local_path_port"`
}

// AutoEncrypt is the agent-global auto_encrypt configuration.
type AutoEncrypt struct {
	// TLS enables receiving certificates for clients from servers
	TLS *bool `json:"tls,omitempty" hcl:"tls" mapstructure:"tls"`

	// Additional DNS SAN entries that clients request for their certificates.
	DNSSAN []string `json:"dns_san,omitempty" hcl:"dns_san" mapstructure:"dns_san"`

	// Additional IP SAN entries that clients request for their certificates.
	IPSAN []string `json:"ip_san,omitempty" hcl:"ip_san" mapstructure:"ip_san"`

	// AllowTLS enables the RPC endpoint on the server to answer
	// AutoEncrypt.Sign requests.
	AllowTLS *bool `json:"allow_tls,omitempty" hcl:"allow_tls" mapstructure:"allow_tls"`
}

// Connect is the agent-global connect configuration.
type Connect struct {
	// Enabled opts the agent into connect. It should be set on all clients and
	// servers in a cluster for correct connect operation.
	Enabled                         *bool                  `json:"enabled,omitempty" hcl:"enabled" mapstructure:"enabled"`
	CAProvider                      *string                `json:"ca_provider,omitempty" hcl:"ca_provider" mapstructure:"ca_provider"`
	CAConfig                        map[string]interface{} `json:"ca_config,omitempty" hcl:"ca_config" mapstructure:"ca_config"`
	MeshGatewayWANFederationEnabled *bool                  `json:"enable_mesh_gateway_wan_federation" hcl:"enable_mesh_gateway_wan_federation" mapstructure:"enable_mesh_gateway_wan_federation"`
}

// SOA is the configuration of SOA for DNS
type SOA struct {
	Refresh *uint32 `json:"refresh,omitempty" hcl:"refresh" mapstructure:"refresh"`
	Retry   *uint32 `json:"retry,omitempty" hcl:"retry" mapstructure:"retry"`
	Expire  *uint32 `json:"expire,omitempty" hcl:"expire" mapstructure:"expire"`
	Minttl  *uint32 `json:"min_ttl,omitempty" hcl:"min_ttl" mapstructure:"min_ttl"`
}

type DNS struct {
	AllowStale         *bool             `json:"allow_stale,omitempty" hcl:"allow_stale" mapstructure:"allow_stale"`
	ARecordLimit       *int              `json:"a_record_limit,omitempty" hcl:"a_record_limit" mapstructure:"a_record_limit"`
	DisableCompression *bool             `json:"disable_compression,omitempty" hcl:"disable_compression" mapstructure:"disable_compression"`
	EnableTruncate     *bool             `json:"enable_truncate,omitempty" hcl:"enable_truncate" mapstructure:"enable_truncate"`
	MaxStale           *string           `json:"max_stale,omitempty" hcl:"max_stale" mapstructure:"max_stale"`
	NodeTTL            *string           `json:"node_ttl,omitempty" hcl:"node_ttl" mapstructure:"node_ttl"`
	OnlyPassing        *bool             `json:"only_passing,omitempty" hcl:"only_passing" mapstructure:"only_passing"`
	RecursorTimeout    *string           `json:"recursor_timeout,omitempty" hcl:"recursor_timeout" mapstructure:"recursor_timeout"`
	ServiceTTL         map[string]string `json:"service_ttl,omitempty" hcl:"service_ttl" mapstructure:"service_ttl"`
	UDPAnswerLimit     *int              `json:"udp_answer_limit,omitempty" hcl:"udp_answer_limit" mapstructure:"udp_answer_limit"`
	NodeMetaTXT        *bool             `json:"enable_additional_node_meta_txt,omitempty" hcl:"enable_additional_node_meta_txt" mapstructure:"enable_additional_node_meta_txt"`
	SOA                *SOA              `json:"soa,omitempty" hcl:"soa" mapstructure:"soa"`
	UseCache           *bool             `json:"use_cache,omitempty" hcl:"use_cache" mapstructure:"use_cache"`
	CacheMaxAge        *string           `json:"cache_max_age,omitempty" hcl:"cache_max_age" mapstructure:"cache_max_age"`

	// Enterprise Only
	PreferNamespace *bool `json:"prefer_namespace,omitempty" hcl:"prefer_namespace" mapstructure:"prefer_namespace"`
}

type HTTPConfig struct {
	BlockEndpoints     []string          `json:"block_endpoints,omitempty" hcl:"block_endpoints" mapstructure:"block_endpoints"`
	AllowWriteHTTPFrom []string          `json:"allow_write_http_from,omitempty" hcl:"allow_write_http_from" mapstructure:"allow_write_http_from"`
	ResponseHeaders    map[string]string `json:"response_headers,omitempty" hcl:"response_headers" mapstructure:"response_headers"`
}

type Performance struct {
	LeaveDrainTime *string `json:"leave_drain_time,omitempty" hcl:"leave_drain_time" mapstructure:"leave_drain_time"`
	RaftMultiplier *int    `json:"raft_multiplier,omitempty" hcl:"raft_multiplier" mapstructure:"raft_multiplier"` // todo(fs): validate as uint
	RPCHoldTimeout *string `json:"rpc_hold_timeout" hcl:"rpc_hold_timeout" mapstructure:"rpc_hold_timeout"`
}

type Telemetry struct {
	CirconusAPIApp                     *string  `json:"circonus_api_app,omitempty" hcl:"circonus_api_app" mapstructure:"circonus_api_app"`
	CirconusAPIToken                   *string  `json:"circonus_api_token,omitempty" hcl:"circonus_api_token" mapstructure:"circonus_api_token"`
	CirconusAPIURL                     *string  `json:"circonus_api_url,omitempty" hcl:"circonus_api_url" mapstructure:"circonus_api_url"`
	CirconusBrokerID                   *string  `json:"circonus_broker_id,omitempty" hcl:"circonus_broker_id" mapstructure:"circonus_broker_id"`
	CirconusBrokerSelectTag            *string  `json:"circonus_broker_select_tag,omitempty" hcl:"circonus_broker_select_tag" mapstructure:"circonus_broker_select_tag"`
	CirconusCheckDisplayName           *string  `json:"circonus_check_display_name,omitempty" hcl:"circonus_check_display_name" mapstructure:"circonus_check_display_name"`
	CirconusCheckForceMetricActivation *string  `json:"circonus_check_force_metric_activation,omitempty" hcl:"circonus_check_force_metric_activation" mapstructure:"circonus_check_force_metric_activation"`
	CirconusCheckID                    *string  `json:"circonus_check_id,omitempty" hcl:"circonus_check_id" mapstructure:"circonus_check_id"`
	CirconusCheckInstanceID            *string  `json:"circonus_check_instance_id,omitempty" hcl:"circonus_check_instance_id" mapstructure:"circonus_check_instance_id"`
	CirconusCheckSearchTag             *string  `json:"circonus_check_search_tag,omitempty" hcl:"circonus_check_search_tag" mapstructure:"circonus_check_search_tag"`
	CirconusCheckTags                  *string  `json:"circonus_check_tags,omitempty" hcl:"circonus_check_tags" mapstructure:"circonus_check_tags"`
	CirconusSubmissionInterval         *string  `json:"circonus_submission_interval,omitempty" hcl:"circonus_submission_interval" mapstructure:"circonus_submission_interval"`
	CirconusSubmissionURL              *string  `json:"circonus_submission_url,omitempty" hcl:"circonus_submission_url" mapstructure:"circonus_submission_url"`
	DisableHostname                    *bool    `json:"disable_hostname,omitempty" hcl:"disable_hostname" mapstructure:"disable_hostname"`
	DogstatsdAddr                      *string  `json:"dogstatsd_addr,omitempty" hcl:"dogstatsd_addr" mapstructure:"dogstatsd_addr"`
	DogstatsdTags                      []string `json:"dogstatsd_tags,omitempty" hcl:"dogstatsd_tags" mapstructure:"dogstatsd_tags"`
	FilterDefault                      *bool    `json:"filter_default,omitempty" hcl:"filter_default" mapstructure:"filter_default"`
	PrefixFilter                       []string `json:"prefix_filter,omitempty" hcl:"prefix_filter" mapstructure:"prefix_filter"`
	MetricsPrefix                      *string  `json:"metrics_prefix,omitempty" hcl:"metrics_prefix" mapstructure:"metrics_prefix"`
	PrometheusRetentionTime            *string  `json:"prometheus_retention_time,omitempty" hcl:"prometheus_retention_time" mapstructure:"prometheus_retention_time"`
	StatsdAddr                         *string  `json:"statsd_address,omitempty" hcl:"statsd_address" mapstructure:"statsd_address"`
	StatsiteAddr                       *string  `json:"statsite_address,omitempty" hcl:"statsite_address" mapstructure:"statsite_address"`
}

type Ports struct {
	DNS            *int `json:"dns,omitempty" hcl:"dns" mapstructure:"dns"`
	HTTP           *int `json:"http,omitempty" hcl:"http" mapstructure:"http"`
	HTTPS          *int `json:"https,omitempty" hcl:"https" mapstructure:"https"`
	SerfLAN        *int `json:"serf_lan,omitempty" hcl:"serf_lan" mapstructure:"serf_lan"`
	SerfWAN        *int `json:"serf_wan,omitempty" hcl:"serf_wan" mapstructure:"serf_wan"`
	Server         *int `json:"server,omitempty" hcl:"server" mapstructure:"server"`
	GRPC           *int `json:"grpc,omitempty" hcl:"grpc" mapstructure:"grpc"`
	ProxyMinPort   *int `json:"proxy_min_port,omitempty" hcl:"proxy_min_port" mapstructure:"proxy_min_port"`
	ProxyMaxPort   *int `json:"proxy_max_port,omitempty" hcl:"proxy_max_port" mapstructure:"proxy_max_port"`
	SidecarMinPort *int `json:"sidecar_min_port,omitempty" hcl:"sidecar_min_port" mapstructure:"sidecar_min_port"`
	SidecarMaxPort *int `json:"sidecar_max_port,omitempty" hcl:"sidecar_max_port" mapstructure:"sidecar_max_port"`
	ExposeMinPort  *int `json:"expose_min_port,omitempty" hcl:"expose_min_port" mapstructure:"expose_min_port"`
	ExposeMaxPort  *int `json:"expose_max_port,omitempty" hcl:"expose_max_port" mapstructure:"expose_max_port"`
}

type UnixSocket struct {
	Group *string `json:"group,omitempty" hcl:"group" mapstructure:"group"`
	Mode  *string `json:"mode,omitempty" hcl:"mode" mapstructure:"mode"`
	User  *string `json:"user,omitempty" hcl:"user" mapstructure:"user"`
}

type Limits struct {
	HTTPMaxConnsPerClient *int     `json:"http_max_conns_per_client,omitempty" hcl:"http_max_conns_per_client" mapstructure:"http_max_conns_per_client"`
	HTTPSHandshakeTimeout *string  `json:"https_handshake_timeout,omitempty" hcl:"https_handshake_timeout" mapstructure:"https_handshake_timeout"`
	RPCHandshakeTimeout   *string  `json:"rpc_handshake_timeout,omitempty" hcl:"rpc_handshake_timeout" mapstructure:"rpc_handshake_timeout"`
	RPCMaxBurst           *int     `json:"rpc_max_burst,omitempty" hcl:"rpc_max_burst" mapstructure:"rpc_max_burst"`
	RPCMaxConnsPerClient  *int     `json:"rpc_max_conns_per_client,omitempty" hcl:"rpc_max_conns_per_client" mapstructure:"rpc_max_conns_per_client"`
	RPCRate               *float64 `json:"rpc_rate,omitempty" hcl:"rpc_rate" mapstructure:"rpc_rate"`
	KVMaxValueSize        *uint64  `json:"kv_max_value_size,omitempty" hcl:"kv_max_value_size" mapstructure:"kv_max_value_size"`
	TxnMaxReqLen          *uint64  `json:"txn_max_req_len,omitempty" hcl:"txn_max_req_len" mapstructure:"txn_max_req_len"`
}

type Segment struct {
	Advertise   *string `json:"advertise,omitempty" hcl:"advertise" mapstructure:"advertise"`
	Bind        *string `json:"bind,omitempty" hcl:"bind" mapstructure:"bind"`
	Name        *string `json:"name,omitempty" hcl:"name" mapstructure:"name"`
	Port        *int    `json:"port,omitempty" hcl:"port" mapstructure:"port"`
	RPCListener *bool   `json:"rpc_listener,omitempty" hcl:"rpc_listener" mapstructure:"rpc_listener"`
}

type ACL struct {
	Enabled                *bool   `json:"enabled,omitempty" hcl:"enabled" mapstructure:"enabled"`
	TokenReplication       *bool   `json:"enable_token_replication,omitempty" hcl:"enable_token_replication" mapstructure:"enable_token_replication"`
	PolicyTTL              *string `json:"policy_ttl,omitempty" hcl:"policy_ttl" mapstructure:"policy_ttl"`
	RoleTTL                *string `json:"role_ttl,omitempty" hcl:"role_ttl" mapstructure:"role_ttl"`
	TokenTTL               *string `json:"token_ttl,omitempty" hcl:"token_ttl" mapstructure:"token_ttl"`
	DownPolicy             *string `json:"down_policy,omitempty" hcl:"down_policy" mapstructure:"down_policy"`
	DefaultPolicy          *string `json:"default_policy,omitempty" hcl:"default_policy" mapstructure:"default_policy"`
	EnableKeyListPolicy    *bool   `json:"enable_key_list_policy,omitempty" hcl:"enable_key_list_policy" mapstructure:"enable_key_list_policy"`
	Tokens                 Tokens  `json:"tokens,omitempty" hcl:"tokens" mapstructure:"tokens"`
	DisabledTTL            *string `json:"disabled_ttl,omitempty" hcl:"disabled_ttl" mapstructure:"disabled_ttl"`
	EnableTokenPersistence *bool   `json:"enable_token_persistence" hcl:"enable_token_persistence" mapstructure:"enable_token_persistence"`

	// Enterprise Only
	MSPDisableBootstrap *bool `json:"msp_disable_bootstrap" hcl:"msp_disable_bootstrap" mapstructure:"msp_disable_bootstrap"`
}

type Tokens struct {
	Master      *string `json:"master,omitempty" hcl:"master" mapstructure:"master"`
	Replication *string `json:"replication,omitempty" hcl:"replication" mapstructure:"replication"`
	AgentMaster *string `json:"agent_master,omitempty" hcl:"agent_master" mapstructure:"agent_master"`
	Default     *string `json:"default,omitempty" hcl:"default" mapstructure:"default"`
	Agent       *string `json:"agent,omitempty" hcl:"agent" mapstructure:"agent"`

	// Enterprise Only
	ManagedServiceProvider []ServiceProviderToken `json:"managed_service_provider,omitempty" hcl:"managed_service_provider" mapstructure:"managed_service_provider"`
}

// ServiceProviderToken groups an accessor and secret for a service provider token. Enterprise Only
type ServiceProviderToken struct {
	AccessorID *string `json:"accessor_id,omitempty" hcl:"accessor_id" mapstructure:"accessor_id"`
	SecretID   *string `json:"secret_id,omitempty" hcl:"secret_id" mapstructure:"secret_id"`
}

type ConfigEntries struct {
	// Bootstrap is the list of config_entries that should only be persisted to
	// cluster on initial startup of a new leader if no such config exists
	// already. The type is map not structs.ConfigEntry for decoding reasons - we
	// need to figure out the right concrete type before we can decode it
	// unabiguously.
	Bootstrap []map[string]interface{} `json:"bootstrap,omitempty" hcl:"bootstrap" mapstructure:"bootstrap"`
}

// Audit allows us to enable and define destinations for auditing
type Audit struct {
	Enabled *bool                `json:"enabled,omitempty" hcl:"enabled" mapstructure:"enabled"`
	Sinks   map[string]AuditSink `json:"sink,omitempty"    hcl:"sink"    mapstructure:"sink"`
}

// AuditSink can be provided multiple times to define pipelines for auditing
type AuditSink struct {
	Name              *string `json:"name,omitempty"               hcl:"name"               mapstructure:"name"`
	Type              *string `json:"type,omitempty"               hcl:"type"               mapstructure:"type"`
	Format            *string `json:"format,omitempty"             hcl:"format"             mapstructure:"format"`
	Path              *string `json:"path,omitempty"               hcl:"path"               mapstructure:"path"`
	DeliveryGuarantee *string `json:"delivery_guarantee,omitempty" hcl:"delivery_guarantee" mapstructure:"delivery_guarantee"`
	RotateBytes       *int    `json:"rotate_bytes,omitempty"       hcl:"rotate_bytes"       mapstructure:"rotate_bytes"`
	RotateDuration    *string `json:"rotate_duration,omitempty"    hcl:"rotate_duration"    mapstructure:"rotate_duration"`
	RotateMaxFiles    *int    `json:"rotate_max_files,omitempty"   hcl:"rotate_max_files"   mapstructure:"rotate_max_files"`
}
