package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl"
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

func NewSource(name, data string) Source {
	return Source{Name: name, Format: FormatFrom(name), Data: data}
}

func FormatFrom(name string) string {
	if strings.HasSuffix(name, ".hcl") {
		return "hcl"
	}
	return "json"
}

// Parse parses a config fragment in either JSON or HCL format.
func Parse(data string, format string) (c Config, err error) {
	switch format {
	case "json":
		err = json.Unmarshal([]byte(data), &c)
	case "hcl":
		err = hcl.Decode(&c, data)
	default:
		err = fmt.Errorf("invalid format: %s", format)
	}
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
	ACLAgentMasterToken         *string                  `json:"acl_agent_master_token,omitempty" hcl:"acl_agent_master_token"`
	ACLAgentToken               *string                  `json:"acl_agent_token,omitempty" hcl:"acl_agent_token"`
	ACLDatacenter               *string                  `json:"acl_datacenter,omitempty" hcl:"acl_datacenter"`
	ACLDefaultPolicy            *string                  `json:"acl_default_policy,omitempty" hcl:"acl_default_policy"`
	ACLDownPolicy               *string                  `json:"acl_down_policy,omitempty" hcl:"acl_down_policy"`
	ACLEnforceVersion8          *bool                    `json:"acl_enforce_version_8,omitempty" hcl:"acl_enforce_version_8"`
	ACLMasterToken              *string                  `json:"acl_master_token,omitempty" hcl:"acl_master_token"`
	ACLReplicationToken         *string                  `json:"acl_replication_token,omitempty" hcl:"acl_replication_token"`
	ACLTTL                      *string                  `json:"acl_ttl,omitempty" hcl:"acl_ttl"`
	ACLToken                    *string                  `json:"acl_token,omitempty" hcl:"acl_token"`
	Addresses                   Addresses                `json:"addresses,omitempty" hcl:"addresses"`
	AdvertiseAddrLAN            *string                  `json:"advertise_addr,omitempty" hcl:"advertise_addr"`
	AdvertiseAddrWAN            *string                  `json:"advertise_addr_wan,omitempty" hcl:"advertise_addr_wan"`
	AdvertiseAddrs              AdvertiseAddrsConfig     `json:"advertise_addrs,omitempty" hcl:"advertise_addrs"`
	Autopilot                   Autopilot                `json:"autopilot,omitempty" hcl:"autopilot"`
	BindAddr                    *string                  `json:"bind_addr,omitempty" hcl:"bind_addr"`
	Bootstrap                   *bool                    `json:"bootstrap,omitempty" hcl:"bootstrap"`
	BootstrapExpect             *int                     `json:"bootstrap_expect,omitempty" hcl:"bootstrap_expect"`
	CAFile                      *string                  `json:"ca_file,omitempty" hcl:"ca_file"`
	CAPath                      *string                  `json:"ca_path,omitempty" hcl:"ca_path"`
	CertFile                    *string                  `json:"cert_file,omitempty" hcl:"cert_file"`
	Check                       *CheckDefinition         `json:"check,omitempty" hcl:"check"` // needs to be a pointer to avoid partial merges
	CheckUpdateInterval         *string                  `json:"check_update_interval,omitempty" hcl:"check_update_interval"`
	Checks                      []CheckDefinition        `json:"checks,omitempty" hcl:"checks"`
	ClientAddr                  *string                  `json:"client_addr,omitempty" hcl:"client_addr"`
	DNS                         DNS                      `json:"dns_config,omitempty" hcl:"dns_config"`
	DNSDomain                   *string                  `json:"domain,omitempty" hcl:"domain"`
	DNSRecursor                 *string                  `json:"recursor,omitempty" hcl:"recursor"`
	DNSRecursors                []string                 `json:"recursors,omitempty" hcl:"recursors"`
	DataDir                     *string                  `json:"data_dir,omitempty" hcl:"data_dir"`
	Datacenter                  *string                  `json:"datacenter,omitempty" hcl:"datacenter"`
	DisableAnonymousSignature   *bool                    `json:"disable_anonymous_signature,omitempty" hcl:"disable_anonymous_signature"`
	DisableCoordinates          *bool                    `json:"disable_coordinates,omitempty" hcl:"disable_coordinates"`
	DisableHostNodeID           *bool                    `json:"disable_host_node_id,omitempty" hcl:"disable_host_node_id"`
	DisableKeyringFile          *bool                    `json:"disable_keyring_file,omitempty" hcl:"disable_keyring_file"`
	DisableRemoteExec           *bool                    `json:"disable_remote_exec,omitempty" hcl:"disable_remote_exec"`
	DisableUpdateCheck          *bool                    `json:"disable_update_check,omitempty" hcl:"disable_update_check"`
	EnableACLReplication        *bool                    `json:"enable_acl_replication,omitempty" hcl:"enable_acl_replication"`
	EnableDebug                 *bool                    `json:"enable_debug,omitempty" hcl:"enable_debug"`
	EnableScriptChecks          *bool                    `json:"enable_script_checks,omitempty" hcl:"enable_script_checks"`
	EnableSyslog                *bool                    `json:"enable_syslog,omitempty" hcl:"enable_syslog"`
	EnableUI                    *bool                    `json:"enable_ui,omitempty" hcl:"enable_ui"`
	EncryptKey                  *string                  `json:"encrypt,omitempty" hcl:"encrypt"`
	EncryptVerifyIncoming       *bool                    `json:"encrypt_verify_incoming,omitempty" hcl:"encrypt_verify_incoming"`
	EncryptVerifyOutgoing       *bool                    `json:"encrypt_verify_outgoing,omitempty" hcl:"encrypt_verify_outgoing"`
	HTTPConfig                  HTTPConfig               `json:"http_config,omitempty" hcl:"http_config"`
	KeyFile                     *string                  `json:"key_file,omitempty" hcl:"key_file"`
	LeaveOnTerm                 *bool                    `json:"leave_on_terminate,omitempty" hcl:"leave_on_terminate"`
	Limits                      Limits                   `json:"limits,omitempty" hcl:"limits"`
	LogLevel                    *string                  `json:"log_level,omitempty" hcl:"log_level"`
	NodeID                      *string                  `json:"node_id,omitempty" hcl:"node_id"`
	NodeMeta                    map[string]string        `json:"node_meta,omitempty" hcl:"node_meta"`
	NodeName                    *string                  `json:"node_name,omitempty" hcl:"node_name"`
	NonVotingServer             *bool                    `json:"non_voting_server,omitempty" hcl:"non_voting_server"`
	Performance                 Performance              `json:"performance,omitempty" hcl:"performance"`
	PidFile                     *string                  `json:"pid_file,omitempty" hcl:"pid_file"`
	Ports                       Ports                    `json:"ports,omitempty" hcl:"ports"`
	RPCProtocol                 *int                     `json:"protocol,omitempty" hcl:"protocol"`
	RaftProtocol                *int                     `json:"raft_protocol,omitempty" hcl:"raft_protocol"`
	ReconnectTimeoutLAN         *string                  `json:"reconnect_timeout,omitempty" hcl:"reconnect_timeout"`
	ReconnectTimeoutWAN         *string                  `json:"reconnect_timeout_wan,omitempty" hcl:"reconnect_timeout_wan"`
	RejoinAfterLeave            *bool                    `json:"rejoin_after_leave,omitempty" hcl:"rejoin_after_leave"`
	RetryJoinIntervalLAN        *string                  `json:"retry_interval,omitempty" hcl:"retry_interval"`
	RetryJoinIntervalWAN        *string                  `json:"retry_interval_wan,omitempty" hcl:"retry_interval_wan"`
	RetryJoinLAN                []string                 `json:"retry_join,omitempty" hcl:"retry_join"`
	RetryJoinMaxAttemptsLAN     *int                     `json:"retry_max,omitempty" hcl:"retry_max"`
	RetryJoinMaxAttemptsWAN     *int                     `json:"retry_max_wan,omitempty" hcl:"retry_max_wan"`
	RetryJoinWAN                []string                 `json:"retry_join_wan,omitempty" hcl:"retry_join_wan"`
	SegmentName                 *string                  `json:"segment,omitempty" hcl:"segment"`
	Segments                    []Segment                `json:"segments,omitempty" hcl:"segments"`
	SerfBindAddrLAN             *string                  `json:"serf_lan,omitempty" hcl:"serf_lan"`
	SerfBindAddrWAN             *string                  `json:"serf_wan,omitempty" hcl:"serf_wan"`
	ServerMode                  *bool                    `json:"server,omitempty" hcl:"server"`
	ServerName                  *string                  `json:"server_name,omitempty" hcl:"server_name"`
	Service                     *ServiceDefinition       `json:"service,omitempty" hcl:"service"`
	Services                    []ServiceDefinition      `json:"services,omitempty" hcl:"services"`
	SessionTTLMin               *string                  `json:"session_ttl_min,omitempty" hcl:"session_ttl_min"`
	SkipLeaveOnInt              *bool                    `json:"skip_leave_on_interrupt,omitempty" hcl:"skip_leave_on_interrupt"`
	StartJoinAddrsLAN           []string                 `json:"start_join,omitempty" hcl:"start_join"`
	StartJoinAddrsWAN           []string                 `json:"start_join_wan,omitempty" hcl:"start_join_wan"`
	SyslogFacility              *string                  `json:"syslog_facility,omitempty" hcl:"syslog_facility"`
	TLSCipherSuites             *string                  `json:"tls_cipher_suites,omitempty" hcl:"tls_cipher_suites"`
	TLSMinVersion               *string                  `json:"tls_min_version,omitempty" hcl:"tls_min_version"`
	TLSPreferServerCipherSuites *bool                    `json:"tls_prefer_server_cipher_suites,omitempty" hcl:"tls_prefer_server_cipher_suites"`
	TaggedAddresses             map[string]string        `json:"tagged_addresses,omitempty" hcl:"tagged_addresses"`
	Telemetry                   Telemetry                `json:"telemetry,omitempty" hcl:"telemetry"`
	TranslateWANAddrs           *bool                    `json:"translate_wan_addrs,omitempty" hcl:"translate_wan_addrs"`
	UIDir                       *string                  `json:"ui_dir,omitempty" hcl:"ui_dir"`
	UnixSocket                  UnixSocket               `json:"unix_sockets,omitempty" hcl:"unix_sockets"`
	VerifyIncoming              *bool                    `json:"verify_incoming,omitempty" hcl:"verify_incoming"`
	VerifyIncomingHTTPS         *bool                    `json:"verify_incoming_https,omitempty" hcl:"verify_incoming_https"`
	VerifyIncomingRPC           *bool                    `json:"verify_incoming_rpc,omitempty" hcl:"verify_incoming_rpc"`
	VerifyOutgoing              *bool                    `json:"verify_outgoing,omitempty" hcl:"verify_outgoing"`
	VerifyServerHostname        *bool                    `json:"verify_server_hostname,omitempty" hcl:"verify_server_hostname"`
	Watches                     []map[string]interface{} `json:"watches,omitempty" hcl:"watches"`

	// non-user configurable values
	ACLDisabledTTL             *string  `json:"acl_disabled_ttl,omitempty" hcl:"acl_disabled_ttl"`
	AEInterval                 *string  `json:"ae_interval,omitempty" hcl:"ae_interval"`
	CheckDeregisterIntervalMin *string  `json:"check_deregister_interval_min,omitempty" hcl:"check_deregister_interval_min"`
	CheckReapInterval          *string  `json:"check_reap_interval,omitempty" hcl:"check_reap_interval"`
	Consul                     Consul   `json:"consul,omitempty" hcl:"consul"`
	Revision                   *string  `json:"revision,omitempty" hcl:"revision"`
	SyncCoordinateIntervalMin  *string  `json:"sync_coordinate_interval_min,omitempty" hcl:"sync_coordinate_interval_min"`
	SyncCoordinateRateTarget   *float64 `json:"sync_coordinate_rate_target,omitempty" hcl:"sync_coordinate_rate_target"`
	Version                    *string  `json:"version,omitempty" hcl:"version"`
	VersionPrerelease          *string  `json:"version_prerelease,omitempty" hcl:"version_prerelease"`

	// deprecated values
	DeprecatedAtlasACLToken          *string           `json:"atlas_acl_token,omitempty" hcl:"atlas_acl_token"`
	DeprecatedAtlasEndpoint          *string           `json:"atlas_endpoint,omitempty" hcl:"atlas_endpoint"`
	DeprecatedAtlasInfrastructure    *string           `json:"atlas_infrastructure,omitempty" hcl:"atlas_infrastructure"`
	DeprecatedAtlasJoin              *bool             `json:"atlas_join,omitempty" hcl:"atlas_join"`
	DeprecatedAtlasToken             *string           `json:"atlas_token,omitempty" hcl:"atlas_token"`
	DeprecatedDogstatsdAddr          *string           `json:"dogstatsd_addr,omitempty" hcl:"dogstatsd_addr"`
	DeprecatedDogstatsdTags          []string          `json:"dogstatsd_tags,omitempty" hcl:"dogstatsd_tags"`
	DeprecatedHTTPAPIResponseHeaders map[string]string `json:"http_api_response_headers,omitempty" hcl:"http_api_response_headers"`
	DeprecatedRetryJoinAzure         RetryJoinAzure    `json:"retry_join_azure,omitempty" hcl:"retry_join_azure"`
	DeprecatedRetryJoinEC2           RetryJoinEC2      `json:"retry_join_ec2,omitempty" hcl:"retry_join_ec2"`
	DeprecatedRetryJoinGCE           RetryJoinGCE      `json:"retry_join_gce,omitempty" hcl:"retry_join_gce"`
	DeprecatedStatsdAddr             *string           `json:"statsd_addr,omitempty" hcl:"statsd_addr"`
	DeprecatedStatsiteAddr           *string           `json:"statsite_addr,omitempty" hcl:"statsite_addr"`
	DeprecatedStatsitePrefix         *string           `json:"statsite_prefix,omitempty" hcl:"statsite_prefix"`
}

type Consul struct {
	Coordinate *struct {
		BatchSize    *int    `json:"batch_size,omitempty" hcl:"batch_size"`
		MaxBatches   *int    `json:"max_batches,omitempty" hcl:"max_batches"`
		UpdatePeriod *string `json:"update_period,omitempty" hcl:"update_period"`
	} `json:"coordinate,omitempty" hcl:"coordinate"`

	Raft *struct {
		ElectionTimeout    *string `json:"election_timeout,omitempty" hcl:"election_timeout"`
		HeartbeatTimeout   *string `json:"heartbeat_timeout,omitempty" hcl:"heartbeat_timeout"`
		LeaderLeaseTimeout *string `json:"leader_lease_timeout,omitempty" hcl:"leader_lease_timeout"`
	} `json:"raft,omitempty" hcl:"raft"`

	SerfLAN *struct {
		Memberlist *struct {
			GossipInterval *string `json:"gossip_interval,omitempty" hcl:"gossip_interval"`
			ProbeInterval  *string `json:"probe_interval,omitempty" hcl:"probe_interval"`
			ProbeTimeout   *string `json:"probe_timeout,omitempty" hcl:"probe_timeout"`
			SuspicionMult  *int    `json:"suspicion_mult,omitempty" hcl:"suspicion_mult"`
		} `json:"memberlist,omitempty" hcl:"memberlist"`
	} `json:"serf_lan,omitempty" hcl:"serf_lan"`

	SerfWAN *struct {
		Memberlist *struct {
			GossipInterval *string `json:"gossip_interval,omitempty" hcl:"gossip_interval"`
			ProbeInterval  *string `json:"probe_interval,omitempty" hcl:"probe_interval"`
			ProbeTimeout   *string `json:"probe_timeout,omitempty" hcl:"probe_timeout"`
			SuspicionMult  *int    `json:"suspicion_mult,omitempty" hcl:"suspicion_mult"`
		} `json:"memberlist,omitempty" hcl:"memberlist"`
	} `json:"serf_wan,omitempty" hcl:"serf_wan"`

	Server *struct {
		HealthInterval *string `json:"health_interval,omitempty" hcl:"health_interval"`
	} `json:"server,omitempty" hcl:"server"`
}

type Addresses struct {
	DNS   *string `json:"dns,omitempty" hcl:"dns"`
	HTTP  *string `json:"http,omitempty" hcl:"http"`
	HTTPS *string `json:"https,omitempty" hcl:"https"`

	DeprecatedRPC *string `json:"rpc,omitempty" hcl:"rpc"`
}

type AdvertiseAddrsConfig struct {
	RPC     *string `json:"rpc,omitempty" hcl:"rpc"`
	SerfLAN *string `json:"serf_lan,omitempty" hcl:"serf_lan"`
	SerfWAN *string `json:"serf_wan,omitempty" hcl:"serf_wan"`
}

type Autopilot struct {
	CleanupDeadServers      *bool   `json:"cleanup_dead_servers,omitempty" hcl:"cleanup_dead_servers"`
	DisableUpgradeMigration *bool   `json:"disable_upgrade_migration,omitempty" hcl:"disable_upgrade_migration"`
	LastContactThreshold    *string `json:"last_contact_threshold,omitempty" hcl:"last_contact_threshold"`
	// todo(fs): do we need uint64 here? If yes, then I need to write a special parser b/c of JSON limit of 2^53-1 for ints
	MaxTrailingLogs         *int64  `json:"max_trailing_logs,omitempty" hcl:"max_trailing_logs"`
	RedundancyZoneTag       *string `json:"redundancy_zone_tag,omitempty" hcl:"redundancy_zone_tag"`
	ServerStabilizationTime *string `json:"server_stabilization_time,omitempty" hcl:"server_stabilization_time"`
	UpgradeVersionTag       *string `json:"upgrade_version_tag,omitempty" hcl:"upgrade_version_tag"`
}

type ServiceDefinition struct {
	ID                *string           `json:"id,omitempty" hcl:"id"`
	Name              *string           `json:"name,omitempty" hcl:"name"`
	Tags              []string          `json:"tags,omitempty" hcl:"tags"`
	Address           *string           `json:"address,omitempty" hcl:"address"`
	Port              *int              `json:"port,omitempty" hcl:"port"`
	Check             *CheckDefinition  `json:"check,omitempty" hcl:"check"`
	Checks            []CheckDefinition `json:"checks,omitempty" hcl:"checks"`
	Token             *string           `json:"token,omitempty" hcl:"token"`
	EnableTagOverride *bool             `json:"enable_tag_override,omitempty" hcl:"enable_tag_override"`
}

type CheckDefinition struct {
	ID                             *string             `json:"id,omitempty" hcl:"id"`
	CheckID                        *string             `json:"check_id,omitempty" hcl:"check_id"`
	Name                           *string             `json:"name,omitempty" hcl:"name"`
	Notes                          *string             `json:"notes,omitempty" hcl:"notes"`
	ServiceID                      *string             `json:"service_id,omitempty" hcl:"service_id"`
	Token                          *string             `json:"token,omitempty" hcl:"token"`
	Status                         *string             `json:"status,omitempty" hcl:"status"`
	Script                         *string             `json:"script,omitempty" hcl:"script"`
	HTTP                           *string             `json:"http,omitempty" hcl:"http"`
	Header                         map[string][]string `json:"header,omitempty" hcl:"header"`
	Method                         *string             `json:"method,omitempty" hcl:"method"`
	TCP                            *string             `json:"tcp,omitempty" hcl:"tcp"`
	Interval                       *string             `json:"interval,omitempty" hcl:"interval"`
	DockerContainerID              *string             `json:"docker_container_id,omitempty" hcl:"docker_container_id"`
	Shell                          *string             `json:"shell,omitempty" hcl:"shell"`
	TLSSkipVerify                  *bool               `json:"tls_skip_verify,omitempty" hcl:"tls_skip_verify"`
	Timeout                        *string             `json:"timeout,omitempty" hcl:"timeout"`
	TTL                            *string             `json:"ttl,omitempty" hcl:"ttl"`
	DeregisterCriticalServiceAfter *string             `json:"deregister_critical_service_after,omitempty" hcl:"deregister_critical_service_after"`

	// alias fields with different names
	AliasDeregisterCriticalServiceAfter *string `json:"deregistercriticalserviceafter,omitempty" hcl:"deregistercriticalserviceafter"`
	AliasDockerContainerID              *string `json:"dockercontainerid,omitempty" hcl:"dockercontainerid"`
	AliasServiceID                      *string `json:"serviceid,omitempty" hcl:"serviceid"`
	AliasTLSSkipVerify                  *bool   `json:"tlsskipverify,omitempty" hcl:"tlsskipverify"`
}

type DNS struct {
	AllowStale         *bool             `json:"allow_stale,omitempty" hcl:"allow_stale"`
	DisableCompression *bool             `json:"disable_compression,omitempty" hcl:"disable_compression"`
	EnableTruncate     *bool             `json:"enable_truncate,omitempty" hcl:"enable_truncate"`
	MaxStale           *string           `json:"max_stale,omitempty" hcl:"max_stale"`
	NodeTTL            *string           `json:"node_ttl,omitempty" hcl:"node_ttl"`
	OnlyPassing        *bool             `json:"only_passing,omitempty" hcl:"only_passing"`
	RecursorTimeout    *string           `json:"recursor_timeout,omitempty" hcl:"recursor_timeout"`
	ServiceTTL         map[string]string `json:"service_ttl,omitempty" hcl:"service_ttl"`
	UDPAnswerLimit     *int              `json:"udp_answer_limit,omitempty" hcl:"udp_answer_limit"`
}

type HTTPConfig struct {
	BlockEndpoints  []string          `json:"block_endpoints,omitempty" hcl:"block_endpoints"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty" hcl:"response_headers"`
}

type Performance struct {
	RaftMultiplier *int `json:"raft_multiplier,omitempty" hcl:"raft_multiplier"` // todo(fs): validate as uint
}

type Telemetry struct {
	CirconusAPIApp                     *string  `json:"circonus_api_app,omitempty" hcl:"circonus_api_app"`
	CirconusAPIToken                   *string  `json:"circonus_api_token,omitempty" json:"-" hcl:"circonus_api_token" json:"-"`
	CirconusAPIURL                     *string  `json:"circonus_api_url,omitempty" hcl:"circonus_api_url"`
	CirconusBrokerID                   *string  `json:"circonus_broker_id,omitempty" hcl:"circonus_broker_id"`
	CirconusBrokerSelectTag            *string  `json:"circonus_broker_select_tag,omitempty" hcl:"circonus_broker_select_tag"`
	CirconusCheckDisplayName           *string  `json:"circonus_check_display_name,omitempty" hcl:"circonus_check_display_name"`
	CirconusCheckForceMetricActivation *string  `json:"circonus_check_force_metric_activation,omitempty" hcl:"circonus_check_force_metric_activation"`
	CirconusCheckID                    *string  `json:"circonus_check_id,omitempty" hcl:"circonus_check_id"`
	CirconusCheckInstanceID            *string  `json:"circonus_check_instance_id,omitempty" hcl:"circonus_check_instance_id"`
	CirconusCheckSearchTag             *string  `json:"circonus_check_search_tag,omitempty" hcl:"circonus_check_search_tag"`
	CirconusCheckTags                  *string  `json:"circonus_check_tags,omitempty" hcl:"circonus_check_tags"`
	CirconusSubmissionInterval         *string  `json:"circonus_submission_interval,omitempty" hcl:"circonus_submission_interval"`
	CirconusSubmissionURL              *string  `json:"circonus_submission_url,omitempty" hcl:"circonus_submission_url"`
	DisableHostname                    *bool    `json:"disable_hostname,omitempty" hcl:"disable_hostname"`
	DogstatsdAddr                      *string  `json:"dogstatsd_addr,omitempty" hcl:"dogstatsd_addr"`
	DogstatsdTags                      []string `json:"dogstatsd_tags,omitempty" hcl:"dogstatsd_tags"`
	FilterDefault                      *bool    `json:"filter_default,omitempty" hcl:"filter_default"`
	PrefixFilter                       []string `json:"prefix_filter,omitempty" hcl:"prefix_filter"`
	StatsdAddr                         *string  `json:"statsd_address,omitempty" hcl:"statsd_address"`
	StatsiteAddr                       *string  `json:"statsite_address,omitempty" hcl:"statsite_address"`
	StatsitePrefix                     *string  `json:"statsite_prefix,omitempty" hcl:"statsite_prefix"`
}

type Ports struct {
	DNS     *int `json:"dns,omitempty" hcl:"dns"`
	HTTP    *int `json:"http,omitempty" hcl:"http"`
	HTTPS   *int `json:"https,omitempty" hcl:"https"`
	SerfLAN *int `json:"serf_lan,omitempty" hcl:"serf_lan"`
	SerfWAN *int `json:"serf_wan,omitempty" hcl:"serf_wan"`
	Server  *int `json:"server,omitempty" hcl:"server"`

	DeprecatedRPC *int `json:"rpc,omitempty" hcl:"rpc"`
}

type RetryJoinAzure struct {
	ClientID        *string `json:"client_id,omitempty" hcl:"client_id"`
	SecretAccessKey *string `json:"secret_access_key,omitempty" hcl:"secret_access_key"`
	SubscriptionID  *string `json:"subscription_id,omitempty" hcl:"subscription_id"`
	TagName         *string `json:"tag_name,omitempty" hcl:"tag_name"`
	TagValue        *string `json:"tag_value,omitempty" hcl:"tag_value"`
	TenantID        *string `json:"tenant_id,omitempty" hcl:"tenant_id"`
}

type RetryJoinEC2 struct {
	AccessKeyID     *string `json:"access_key_id,omitempty" hcl:"access_key_id"`
	Region          *string `json:"region,omitempty" hcl:"region"`
	SecretAccessKey *string `json:"secret_access_key,omitempty" hcl:"secret_access_key"`
	TagKey          *string `json:"tag_key,omitempty" hcl:"tag_key"`
	TagValue        *string `json:"tag_value,omitempty" hcl:"tag_value"`
}

type RetryJoinGCE struct {
	CredentialsFile *string `json:"credentials_file,omitempty" hcl:"credentials_file"`
	ProjectName     *string `json:"project_name,omitempty" hcl:"project_name"`
	TagValue        *string `json:"tag_value,omitempty" hcl:"tag_value"`
	ZonePattern     *string `json:"zone_pattern,omitempty" hcl:"zone_pattern"`
}

type UnixSocket struct {
	Group *string `json:"group,omitempty" hcl:"group"`
	Mode  *string `json:"mode,omitempty" hcl:"mode"`
	User  *string `json:"user,omitempty" hcl:"user"`
}

type Limits struct {
	RPCMaxBurst *int     `json:"rpc_max_burst,omitempty" hcl:"rpc_max_burst"`
	RPCRate     *float64 `json:"rpc_rate,omitempty" hcl:"rpc_rate"`
}

type Segment struct {
	Advertise   *string `json:"advertise,omitempty" hcl:"advertise"`
	Bind        *string `json:"bind,omitempty" hcl:"bind"`
	Name        *string `json:"name,omitempty" hcl:"name"`
	Port        *int    `json:"port,omitempty" hcl:"port"`
	RPCListener *bool   `json:"rpc_listener,omitempty" hcl:"rpc_listener"`
}
