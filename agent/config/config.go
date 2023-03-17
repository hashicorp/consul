// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/lib/decode"
)

// Source parses configuration from some source.
type Source interface {
	// Source returns an identifier for the Source that can be used in error message
	Source() string
	// Parse a configuration and return the result.
	Parse() (Config, Metadata, error)
}

// ErrNoData indicates to Builder.Build that the source contained no data, and
// it can be skipped.
var ErrNoData = fmt.Errorf("config source contained no data")

// FileSource implements Source and parses a config from a file.
type FileSource struct {
	Name   string
	Format string
	Data   string
}

func (f FileSource) Source() string {
	return f.Name
}

// Parse a config file in either JSON or HCL format.
func (f FileSource) Parse() (Config, Metadata, error) {
	m := Metadata{}
	if f.Name == "" || f.Data == "" {
		return Config{}, m, ErrNoData
	}

	var raw map[string]interface{}
	var err error
	var md mapstructure.Metadata
	switch f.Format {
	case "json":
		err = json.Unmarshal([]byte(f.Data), &raw)
	case "hcl":
		err = hcl.Decode(&raw, f.Data)
	default:
		err = fmt.Errorf("invalid format: %s", f.Format)
	}
	if err != nil {
		return Config{}, m, err
	}

	var target decodeTarget
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			// decode.HookWeakDecodeFromSlice is only necessary when reading from
			// an HCL config file. In the future we could omit it when reading from
			// JSON configs. It is left here for now to maintain backwards compat
			// for the unlikely scenario that someone is using malformed JSON configs
			// and expecting this behaviour to correct their config.
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
		),
		Metadata: &md,
		Result:   &target,
	})
	if err != nil {
		return Config{}, m, err
	}
	if err := d.Decode(raw); err != nil {
		return Config{}, m, err
	}

	c, warns := applyDeprecatedConfig(&target)
	m.Unused = md.Unused
	m.Keys = md.Keys
	m.Warnings = warns
	return c, m, nil
}

// Metadata created by Source.Parse
type Metadata struct {
	// Keys used in the config file.
	Keys []string
	// Unused keys that did not match any struct fields.
	Unused []string
	// Warnings caused by deprecated fields
	Warnings []string
}

// LiteralSource implements Source and returns an existing Config struct.
type LiteralSource struct {
	Name   string
	Config Config
}

func (l LiteralSource) Source() string {
	return l.Name
}

func (l LiteralSource) Parse() (Config, Metadata, error) {
	return l.Config, Metadata{}, nil
}

type decodeTarget struct {
	DeprecatedConfig `mapstructure:",squash"`
	Config           `mapstructure:",squash"`
}

// Cache configuration for the agent/cache.
type Cache struct {
	// EntryFetchMaxBurst max burst size of RateLimit for a single cache entry
	EntryFetchMaxBurst *int `mapstructure:"entry_fetch_max_burst"`
	// EntryFetchRate represents the max calls/sec for a single cache entry
	EntryFetchRate *float64 `mapstructure:"entry_fetch_rate"`
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
	ACL                              ACL                 `mapstructure:"acl" json:"-"`
	Addresses                        Addresses           `mapstructure:"addresses" json:"-"`
	AdvertiseAddrLAN                 *string             `mapstructure:"advertise_addr" json:"advertise_addr,omitempty"`
	AdvertiseAddrLANIPv4             *string             `mapstructure:"advertise_addr_ipv4" json:"advertise_addr_ipv4,omitempty"`
	AdvertiseAddrLANIPv6             *string             `mapstructure:"advertise_addr_ipv6" json:"advertise_addr_ipv6,omitempty"`
	AdvertiseAddrWAN                 *string             `mapstructure:"advertise_addr_wan" json:"advertise_addr_wan,omitempty"`
	AdvertiseAddrWANIPv4             *string             `mapstructure:"advertise_addr_wan_ipv4" json:"advertise_addr_wan_ipv4,omitempty"`
	AdvertiseAddrWANIPv6             *string             `mapstructure:"advertise_addr_wan_ipv6" json:"advertise_addr_wan_ipv6,omitempty"`
	AdvertiseReconnectTimeout        *string             `mapstructure:"advertise_reconnect_timeout" json:"-"`
	AutoConfig                       AutoConfigRaw       `mapstructure:"auto_config" json:"-"`
	Autopilot                        Autopilot           `mapstructure:"autopilot" json:"-"`
	BindAddr                         *string             `mapstructure:"bind_addr" json:"bind_addr,omitempty"`
	Bootstrap                        *bool               `mapstructure:"bootstrap" json:"bootstrap,omitempty"`
	BootstrapExpect                  *int                `mapstructure:"bootstrap_expect" json:"bootstrap_expect,omitempty"`
	Cache                            Cache               `mapstructure:"cache" json:"-"`
	Check                            *CheckDefinition    `mapstructure:"check" json:"-"` // needs to be a pointer to avoid partial merges
	CheckOutputMaxSize               *int                `mapstructure:"check_output_max_size" json:"check_output_max_size,omitempty"`
	CheckUpdateInterval              *string             `mapstructure:"check_update_interval" json:"check_update_interval,omitempty"`
	Checks                           []CheckDefinition   `mapstructure:"checks" json:"-"`
	ClientAddr                       *string             `mapstructure:"client_addr" json:"client_addr,omitempty"`
	Cloud                            *CloudConfigRaw     `mapstructure:"cloud" json:"-"`
	ConfigEntries                    ConfigEntries       `mapstructure:"config_entries" json:"-"`
	AutoEncrypt                      AutoEncrypt         `mapstructure:"auto_encrypt" json:"auto_encrypt,omitempty"`
	Connect                          Connect             `mapstructure:"connect" json:"connect,omitempty"`
	DNS                              DNS                 `mapstructure:"dns_config" json:"-"`
	DNSDomain                        *string             `mapstructure:"domain" json:"domain,omitempty"`
	DNSAltDomain                     *string             `mapstructure:"alt_domain" json:"alt_domain,omitempty"`
	DNSRecursors                     []string            `mapstructure:"recursors" json:"recursors,omitempty"`
	DataDir                          *string             `mapstructure:"data_dir" json:"data_dir,omitempty"`
	Datacenter                       *string             `mapstructure:"datacenter" json:"datacenter,omitempty"`
	DefaultQueryTime                 *string             `mapstructure:"default_query_time" json:"default_query_time,omitempty"`
	DisableAnonymousSignature        *bool               `mapstructure:"disable_anonymous_signature" json:"disable_anonymous_signature,omitempty"`
	DisableCoordinates               *bool               `mapstructure:"disable_coordinates" json:"disable_coordinates,omitempty"`
	DisableHostNodeID                *bool               `mapstructure:"disable_host_node_id" json:"disable_host_node_id,omitempty"`
	DisableHTTPUnprintableCharFilter *bool               `mapstructure:"disable_http_unprintable_char_filter" json:"disable_http_unprintable_char_filter,omitempty"`
	DisableKeyringFile               *bool               `mapstructure:"disable_keyring_file" json:"disable_keyring_file,omitempty"`
	DisableRemoteExec                *bool               `mapstructure:"disable_remote_exec" json:"disable_remote_exec,omitempty"`
	DisableUpdateCheck               *bool               `mapstructure:"disable_update_check" json:"disable_update_check,omitempty"`
	DiscardCheckOutput               *bool               `mapstructure:"discard_check_output" json:"discard_check_output,omitempty"`
	DiscoveryMaxStale                *string             `mapstructure:"discovery_max_stale" json:"discovery_max_stale,omitempty"`
	EnableAgentTLSForChecks          *bool               `mapstructure:"enable_agent_tls_for_checks" json:"enable_agent_tls_for_checks,omitempty"`
	EnableCentralServiceConfig       *bool               `mapstructure:"enable_central_service_config" json:"enable_central_service_config,omitempty"`
	EnableDebug                      *bool               `mapstructure:"enable_debug" json:"enable_debug,omitempty"`
	EnableScriptChecks               *bool               `mapstructure:"enable_script_checks" json:"enable_script_checks,omitempty"`
	EnableLocalScriptChecks          *bool               `mapstructure:"enable_local_script_checks" json:"enable_local_script_checks,omitempty"`
	EnableSyslog                     *bool               `mapstructure:"enable_syslog" json:"enable_syslog,omitempty"`
	EncryptKey                       *string             `mapstructure:"encrypt" json:"encrypt,omitempty"`
	EncryptVerifyIncoming            *bool               `mapstructure:"encrypt_verify_incoming" json:"encrypt_verify_incoming,omitempty"`
	EncryptVerifyOutgoing            *bool               `mapstructure:"encrypt_verify_outgoing" json:"encrypt_verify_outgoing,omitempty"`
	GossipLAN                        GossipLANConfig     `mapstructure:"gossip_lan" json:"-"`
	GossipWAN                        GossipWANConfig     `mapstructure:"gossip_wan" json:"-"`
	HTTPConfig                       HTTPConfig          `mapstructure:"http_config" json:"-"`
	LeaveOnTerm                      *bool               `mapstructure:"leave_on_terminate" json:"leave_on_terminate,omitempty"`
	LicensePath                      *string             `mapstructure:"license_path" json:"license_path,omitempty"`
	Limits                           Limits              `mapstructure:"limits" json:"-"`
	Locality                         *Locality           `mapstructure:"locality" json:"-"`
	LogLevel                         *string             `mapstructure:"log_level" json:"log_level,omitempty"`
	LogSublevels                     map[string]string   `mapstructure:"log_sublevels" json:"log_sublevels,omitempty"`
	LogJSON                          *bool               `mapstructure:"log_json" json:"log_json,omitempty"`
	LogFile                          *string             `mapstructure:"log_file" json:"log_file,omitempty"`
	LogRotateDuration                *string             `mapstructure:"log_rotate_duration" json:"log_rotate_duration,omitempty"`
	LogRotateBytes                   *int                `mapstructure:"log_rotate_bytes" json:"log_rotate_bytes,omitempty"`
	LogRotateMaxFiles                *int                `mapstructure:"log_rotate_max_files" json:"log_rotate_max_files,omitempty"`
	MaxQueryTime                     *string             `mapstructure:"max_query_time" json:"max_query_time,omitempty"`
	NodeID                           *string             `mapstructure:"node_id" json:"node_id,omitempty"`
	NodeMeta                         map[string]string   `mapstructure:"node_meta" json:"node_meta,omitempty"`
	NodeName                         *string             `mapstructure:"node_name" json:"node_name,omitempty"`
	Peering                          Peering             `mapstructure:"peering" json:"-"`
	Performance                      Performance         `mapstructure:"performance" json:"-"`
	PidFile                          *string             `mapstructure:"pid_file" json:"pid_file,omitempty"`
	Ports                            Ports               `mapstructure:"ports" json:"ports,omitempty"`
	PrimaryDatacenter                *string             `mapstructure:"primary_datacenter" json:"primary_datacenter,omitempty"`
	PrimaryGateways                  []string            `mapstructure:"primary_gateways" json:"primary_gateways,omitempty"`
	PrimaryGatewaysInterval          *string             `mapstructure:"primary_gateways_interval" json:"primary_gateways_interval,omitempty"`
	RPCProtocol                      *int                `mapstructure:"protocol" json:"protocol,omitempty"`
	RaftProtocol                     *int                `mapstructure:"raft_protocol" json:"raft_protocol,omitempty"`
	RaftSnapshotThreshold            *int                `mapstructure:"raft_snapshot_threshold" json:"raft_snapshot_threshold,omitempty"`
	RaftSnapshotInterval             *string             `mapstructure:"raft_snapshot_interval" json:"raft_snapshot_interval,omitempty"`
	RaftTrailingLogs                 *int                `mapstructure:"raft_trailing_logs" json:"raft_trailing_logs,omitempty"`
	ReconnectTimeoutLAN              *string             `mapstructure:"reconnect_timeout" json:"reconnect_timeout,omitempty"`
	ReconnectTimeoutWAN              *string             `mapstructure:"reconnect_timeout_wan" json:"reconnect_timeout_wan,omitempty"`
	RejoinAfterLeave                 *bool               `mapstructure:"rejoin_after_leave" json:"rejoin_after_leave,omitempty"`
	AutoReloadConfig                 *bool               `mapstructure:"auto_reload_config" json:"auto_reload_config,omitempty"`
	RetryJoinIntervalLAN             *string             `mapstructure:"retry_interval" json:"retry_interval,omitempty"`
	RetryJoinIntervalWAN             *string             `mapstructure:"retry_interval_wan" json:"retry_interval_wan,omitempty"`
	RetryJoinLAN                     []string            `mapstructure:"retry_join" json:"retry_join,omitempty"`
	RetryJoinMaxAttemptsLAN          *int                `mapstructure:"retry_max" json:"retry_max,omitempty"`
	RetryJoinMaxAttemptsWAN          *int                `mapstructure:"retry_max_wan" json:"retry_max_wan,omitempty"`
	RetryJoinWAN                     []string            `mapstructure:"retry_join_wan" json:"retry_join_wan,omitempty"`
	SerfAllowedCIDRsLAN              []string            `mapstructure:"serf_lan_allowed_cidrs" json:"serf_lan_allowed_cidrs,omitempty"`
	SerfAllowedCIDRsWAN              []string            `mapstructure:"serf_wan_allowed_cidrs" json:"serf_wan_allowed_cidrs,omitempty"`
	SerfBindAddrLAN                  *string             `mapstructure:"serf_lan" json:"serf_lan,omitempty"`
	SerfBindAddrWAN                  *string             `mapstructure:"serf_wan" json:"serf_wan,omitempty"`
	ServerMode                       *bool               `mapstructure:"server" json:"server,omitempty"`
	ServerName                       *string             `mapstructure:"server_name" json:"server_name,omitempty"`
	Service                          *ServiceDefinition  `mapstructure:"service" json:"-"`
	Services                         []ServiceDefinition `mapstructure:"services" json:"-"`
	SessionTTLMin                    *string             `mapstructure:"session_ttl_min" json:"session_ttl_min,omitempty"`
	SkipLeaveOnInt                   *bool               `mapstructure:"skip_leave_on_interrupt" json:"skip_leave_on_interrupt,omitempty"`
	SyslogFacility                   *string             `mapstructure:"syslog_facility" json:"syslog_facility,omitempty"`
	TLS                              TLS                 `mapstructure:"tls" json:"tls,omitempty"`
	TaggedAddresses                  map[string]string   `mapstructure:"tagged_addresses" json:"tagged_addresses,omitempty"`
	Telemetry                        Telemetry           `mapstructure:"telemetry" json:"telemetry,omitempty"`
	TranslateWANAddrs                *bool               `mapstructure:"translate_wan_addrs" json:"translate_wan_addrs,omitempty"`
	XDS                              XDS                 `mapstructure:"xds" json:"-"`

	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UI *bool `mapstructure:"ui" json:"-"`
	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UIContentPath *string `mapstructure:"ui_content_path" json:"-"`
	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UIDir    *string     `mapstructure:"ui_dir" json:"-"`
	UIConfig RawUIConfig `mapstructure:"ui_config" json:"-"`

	UnixSocket UnixSocket               `mapstructure:"unix_sockets" json:"-"`
	Watches    []map[string]interface{} `mapstructure:"watches" json:"-"`

	RPC RPC `mapstructure:"rpc" json:"-"`

	RaftLogStore RaftLogStoreRaw `mapstructure:"raft_logstore" json:"raft_logstore,omitempty"`

	// UseStreamingBackend instead of blocking queries for service health and
	// any other endpoints which support streaming.
	UseStreamingBackend *bool `mapstructure:"use_streaming_backend" json:"-"`

	// This isn't used by Consul but we've documented a feature where users
	// can deploy their snapshot agent configs alongside their Consul configs
	// so we have a placeholder here so it can be parsed but this doesn't
	// manifest itself in any way inside the runtime config.
	SnapshotAgent map[string]interface{} `mapstructure:"snapshot_agent" json:"-"`

	// non-user configurable values
	AEInterval                 *string    `mapstructure:"ae_interval" json:"-"`
	CheckDeregisterIntervalMin *string    `mapstructure:"check_deregister_interval_min" json:"-"`
	CheckReapInterval          *string    `mapstructure:"check_reap_interval" json:"-"`
	Consul                     Consul     `mapstructure:"consul" json:"-"`
	Revision                   *string    `mapstructure:"revision" json:"-"`
	SegmentLimit               *int       `mapstructure:"segment_limit" json:"-"`
	SegmentNameLimit           *int       `mapstructure:"segment_name_limit" json:"-"`
	SyncCoordinateIntervalMin  *string    `mapstructure:"sync_coordinate_interval_min" json:"-"`
	SyncCoordinateRateTarget   *float64   `mapstructure:"sync_coordinate_rate_target" json:"-"`
	Version                    *string    `mapstructure:"version" json:"-"`
	VersionPrerelease          *string    `mapstructure:"version_prerelease" json:"-"`
	VersionMetadata            *string    `mapstructure:"version_metadata" json:"-"`
	BuildDate                  *time.Time `mapstructure:"build_date" json:"-"`

	// Enterprise Only
	Audit Audit `mapstructure:"audit" json:"-"`
	// Enterprise Only
	ReadReplica *bool `mapstructure:"read_replica" alias:"non_voting_server" json:"-"`
	// Enterprise Only
	SegmentName *string `mapstructure:"segment" json:"-"`
	// Enterprise Only
	Segments []Segment `mapstructure:"segments" json:"-"`
	// Enterprise Only
	Partition *string `mapstructure:"partition" json:"-"`

	// Enterprise Only - not user configurable
	LicensePollBaseTime   *string `mapstructure:"license_poll_base_time" json:"-"`
	LicensePollMaxTime    *string `mapstructure:"license_poll_max_time" json:"-"`
	LicenseUpdateBaseTime *string `mapstructure:"license_update_base_time" json:"-"`
	LicenseUpdateMaxTime  *string `mapstructure:"license_update_max_time" json:"-"`

	// license reporting
	Reporting Reporting `mapstructure:"reporting" json:"-"`
}

type GossipLANConfig struct {
	GossipNodes    *int    `mapstructure:"gossip_nodes"`
	GossipInterval *string `mapstructure:"gossip_interval"`
	ProbeInterval  *string `mapstructure:"probe_interval"`
	ProbeTimeout   *string `mapstructure:"probe_timeout"`
	SuspicionMult  *int    `mapstructure:"suspicion_mult"`
	RetransmitMult *int    `mapstructure:"retransmit_mult"`
}

type GossipWANConfig struct {
	GossipNodes    *int    `mapstructure:"gossip_nodes"`
	GossipInterval *string `mapstructure:"gossip_interval"`
	ProbeInterval  *string `mapstructure:"probe_interval"`
	ProbeTimeout   *string `mapstructure:"probe_timeout"`
	SuspicionMult  *int    `mapstructure:"suspicion_mult"`
	RetransmitMult *int    `mapstructure:"retransmit_mult"`
}

// Locality identifies where a given entity is running.
type Locality struct {
	// Region is region the zone belongs to.
	Region *string `mapstructure:"region"`

	// Zone is the zone the entity is running in.
	Zone *string `mapstructure:"zone"`
}

type Consul struct {
	Coordinate struct {
		UpdateBatchSize  *int    `mapstructure:"update_batch_size"`
		UpdateMaxBatches *int    `mapstructure:"update_max_batches"`
		UpdatePeriod     *string `mapstructure:"update_period"`
	} `mapstructure:"coordinate"`

	Raft struct {
		ElectionTimeout    *string `mapstructure:"election_timeout"`
		HeartbeatTimeout   *string `mapstructure:"heartbeat_timeout"`
		LeaderLeaseTimeout *string `mapstructure:"leader_lease_timeout"`
	} `mapstructure:"raft"`

	Server struct {
		HealthInterval *string `mapstructure:"health_interval"`
	} `mapstructure:"server"`
}

type Addresses struct {
	DNS     *string `mapstructure:"dns"`
	HTTP    *string `mapstructure:"http"`
	HTTPS   *string `mapstructure:"https"`
	GRPC    *string `mapstructure:"grpc"`
	GRPCTLS *string `mapstructure:"grpc_tls"`
}

type AdvertiseAddrsConfig struct {
	RPC     *string `mapstructure:"rpc"`
	SerfLAN *string `mapstructure:"serf_lan"`
	SerfWAN *string `mapstructure:"serf_wan"`
}

type Autopilot struct {
	CleanupDeadServers      *bool   `mapstructure:"cleanup_dead_servers"`
	LastContactThreshold    *string `mapstructure:"last_contact_threshold"`
	MaxTrailingLogs         *int    `mapstructure:"max_trailing_logs"`
	MinQuorum               *uint   `mapstructure:"min_quorum"`
	ServerStabilizationTime *string `mapstructure:"server_stabilization_time"`

	// Enterprise Only
	DisableUpgradeMigration *bool `mapstructure:"disable_upgrade_migration"`
	// Enterprise Only
	RedundancyZoneTag *string `mapstructure:"redundancy_zone_tag"`
	// Enterprise Only
	UpgradeVersionTag *string `mapstructure:"upgrade_version_tag"`
}

// ServiceWeights defines the registration of weights used in DNS for a Service
type ServiceWeights struct {
	Passing *int `mapstructure:"passing"`
	Warning *int `mapstructure:"warning"`
}

type ServiceAddress struct {
	Address *string `mapstructure:"address"`
	Port    *int    `mapstructure:"port"`
}

type ServiceDefinition struct {
	Kind              *string                   `mapstructure:"kind"`
	ID                *string                   `mapstructure:"id"`
	Name              *string                   `mapstructure:"name"`
	Tags              []string                  `mapstructure:"tags"`
	Address           *string                   `mapstructure:"address"`
	TaggedAddresses   map[string]ServiceAddress `mapstructure:"tagged_addresses"`
	Meta              map[string]string         `mapstructure:"meta"`
	Port              *int                      `mapstructure:"port"`
	SocketPath        *string                   `mapstructure:"socket_path"`
	Check             *CheckDefinition          `mapstructure:"check"`
	Checks            []CheckDefinition         `mapstructure:"checks"`
	Token             *string                   `mapstructure:"token"`
	Weights           *ServiceWeights           `mapstructure:"weights"`
	EnableTagOverride *bool                     `mapstructure:"enable_tag_override"`
	Proxy             *ServiceProxy             `mapstructure:"proxy"`
	Connect           *ServiceConnect           `mapstructure:"connect"`

	EnterpriseMeta `mapstructure:",squash"`
}

type CheckDefinition struct {
	ID                             *string             `mapstructure:"id"`
	Name                           *string             `mapstructure:"name"`
	Notes                          *string             `mapstructure:"notes"`
	ServiceID                      *string             `mapstructure:"service_id" alias:"serviceid"`
	Token                          *string             `mapstructure:"token"`
	Status                         *string             `mapstructure:"status"`
	ScriptArgs                     []string            `mapstructure:"args" alias:"scriptargs"`
	HTTP                           *string             `mapstructure:"http"`
	Header                         map[string][]string `mapstructure:"header"`
	Method                         *string             `mapstructure:"method"`
	Body                           *string             `mapstructure:"body"`
	DisableRedirects               *bool               `mapstructure:"disable_redirects"`
	OutputMaxSize                  *int                `mapstructure:"output_max_size"`
	TCP                            *string             `mapstructure:"tcp"`
	UDP                            *string             `mapstructure:"udp"`
	Interval                       *string             `mapstructure:"interval"`
	DockerContainerID              *string             `mapstructure:"docker_container_id" alias:"dockercontainerid"`
	Shell                          *string             `mapstructure:"shell"`
	GRPC                           *string             `mapstructure:"grpc"`
	GRPCUseTLS                     *bool               `mapstructure:"grpc_use_tls"`
	TLSServerName                  *string             `mapstructure:"tls_server_name"`
	TLSSkipVerify                  *bool               `mapstructure:"tls_skip_verify" alias:"tlsskipverify"`
	AliasNode                      *string             `mapstructure:"alias_node"`
	AliasService                   *string             `mapstructure:"alias_service"`
	Timeout                        *string             `mapstructure:"timeout"`
	TTL                            *string             `mapstructure:"ttl"`
	H2PING                         *string             `mapstructure:"h2ping"`
	H2PingUseTLS                   *bool               `mapstructure:"h2ping_use_tls"`
	OSService                      *string             `mapstructure:"os_service"`
	SuccessBeforePassing           *int                `mapstructure:"success_before_passing"`
	FailuresBeforeWarning          *int                `mapstructure:"failures_before_warning"`
	FailuresBeforeCritical         *int                `mapstructure:"failures_before_critical"`
	DeregisterCriticalServiceAfter *string             `mapstructure:"deregister_critical_service_after" alias:"deregistercriticalserviceafter"`

	EnterpriseMeta `mapstructure:",squash"`
}

// ServiceConnect is the connect block within a service registration
type ServiceConnect struct {
	// Native is true when this service can natively understand Connect.
	Native *bool `mapstructure:"native"`

	// SidecarService is a nested Service Definition to register at the same time.
	// It's purely a convenience mechanism to allow specifying a sidecar service
	// along with the application service definition. It's nested nature allows
	// all of the fields to be defaulted which can reduce the amount of
	// boilerplate needed to register a sidecar service separately, but the end
	// result is identical to just making a second service registration via any
	// other means.
	SidecarService *ServiceDefinition `mapstructure:"sidecar_service"`
}

// ServiceProxy is the additional config needed for a Kind = connect-proxy
// registration.
type ServiceProxy struct {
	// DestinationServiceName is required and is the name of the service to accept
	// traffic for.
	DestinationServiceName *string `mapstructure:"destination_service_name"`

	// DestinationServiceID is optional and should only be specified for
	// "side-car" style proxies where the proxy is in front of just a single
	// instance of the service. It should be set to the service ID of the instance
	// being represented which must be registered to the same agent. It's valid to
	// provide a service ID that does not yet exist to avoid timing issues when
	// bootstrapping a service with a proxy.
	DestinationServiceID *string `mapstructure:"destination_service_id"`

	// LocalServiceAddress is the address of the local service instance. It is
	// optional and should only be specified for "side-car" style proxies. It will
	// default to 127.0.0.1 if the proxy is a "side-car" (DestinationServiceID is
	// set) but otherwise will be ignored.
	LocalServiceAddress *string `mapstructure:"local_service_address"`

	// LocalServicePort is the port of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies. It will default
	// to the registered port for the instance if the proxy is a "side-car"
	// (DestinationServiceID is set) but otherwise will be ignored.
	LocalServicePort *int `mapstructure:"local_service_port"`

	// LocalServiceSocketPath is the socket of the local service instance. It is optional
	// and should only be specified for "side-car" style proxies.
	LocalServiceSocketPath string `mapstructure:"local_service_socket_path"`

	// TransparentProxy configuration.
	TransparentProxy *TransparentProxyConfig `mapstructure:"transparent_proxy"`

	// Mode represents how the proxy's inbound and upstream listeners are dialed.
	Mode *string `mapstructure:"mode"`

	// Config is the arbitrary configuration data provided with the proxy
	// registration.
	Config map[string]interface{} `mapstructure:"config"`

	// Upstreams describes any upstream dependencies the proxy instance should
	// setup.
	Upstreams []Upstream `mapstructure:"upstreams"`

	// Mesh Gateway Configuration
	MeshGateway *MeshGatewayConfig `mapstructure:"mesh_gateway"`

	// Expose defines whether checks or paths are exposed through the proxy
	Expose *ExposeConfig `mapstructure:"expose"`
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
	DestinationType      *string `mapstructure:"destination_type"`
	DestinationNamespace *string `mapstructure:"destination_namespace"`
	DestinationPartition *string `mapstructure:"destination_partition"`
	DestinationPeer      *string `mapstructure:"destination_peer"`
	DestinationName      *string `mapstructure:"destination_name"`

	// Datacenter that the service discovery request should be run against. Note
	// for prepared queries, the actual results might be from a different
	// datacenter.
	Datacenter *string `mapstructure:"datacenter"`

	// It would be worth thinking about a separate structure for these four items,
	// unifying under address as something like "unix:/tmp/foo", "tcp:localhost:80" could make sense
	// LocalBindAddress is the ip address a side-car proxy should listen on for
	// traffic destined for this upstream service. Default if empty and local bind socket
	// is not present is 127.0.0.1.
	LocalBindAddress *string `mapstructure:"local_bind_address"`

	// LocalBindPort is the ip address a side-car proxy should listen on for traffic
	// destined for this upstream service. Required.
	LocalBindPort *int `mapstructure:"local_bind_port"`

	// These are exclusive with LocalBindAddress/LocalBindPort. These are created under our control.
	LocalBindSocketPath *string `mapstructure:"local_bind_socket_path"`
	LocalBindSocketMode *string `mapstructure:"local_bind_socket_mode"`

	// Config is an opaque config that is specific to the proxy process being run.
	// It can be used to pass arbitrary configuration for this specific upstream
	// to the proxy.
	Config map[string]interface{} `mapstructure:"config"`

	// Mesh Gateway Configuration
	MeshGateway *MeshGatewayConfig `mapstructure:"mesh_gateway"`
}

type MeshGatewayConfig struct {
	// Mesh Gateway Mode
	Mode *string `mapstructure:"mode"`
}

type TransparentProxyConfig struct {
	// The port of the listener where outbound application traffic is being redirected to.
	OutboundListenerPort *int `mapstructure:"outbound_listener_port"`

	// DialedDirectly indicates whether transparent proxies can dial this proxy instance directly.
	// The discovery chain is not considered when dialing a service instance directly.
	// This setting is useful when addressing stateful services, such as a database cluster with a leader node.
	DialedDirectly *bool `mapstructure:"dialed_directly"`
}

// ExposeConfig describes HTTP paths to expose through Envoy outside of Connect.
// Users can expose individual paths and/or all HTTP/GRPC paths for checks.
type ExposeConfig struct {
	// Checks defines whether paths associated with Consul checks will be exposed.
	// This flag triggers exposing all HTTP and GRPC check paths registered for the service.
	Checks *bool `mapstructure:"checks"`

	// Port defines the port of the proxy's listener for exposed paths.
	Port *int `mapstructure:"port"`

	// Paths is the list of paths exposed through the proxy.
	Paths []ExposePath `mapstructure:"paths"`
}

type ExposePath struct {
	// ListenerPort defines the port of the proxy's listener for exposed paths.
	ListenerPort *int `mapstructure:"listener_port"`

	// Path is the path to expose through the proxy, ie. "/metrics."
	Path *string `mapstructure:"path"`

	// Protocol describes the upstream's service protocol.
	Protocol *string `mapstructure:"protocol"`

	// LocalPathPort is the port that the service is listening on for the given path.
	LocalPathPort *int `mapstructure:"local_path_port"`
}

// AutoEncrypt is the agent-global auto_encrypt configuration.
type AutoEncrypt struct {
	// TLS enables receiving certificates for clients from servers
	TLS *bool `mapstructure:"tls" json:"tls,omitempty"`

	// Additional DNS SAN entries that clients request for their certificates.
	DNSSAN []string `mapstructure:"dns_san" json:"dns_san,omitempty"`

	// Additional IP SAN entries that clients request for their certificates.
	IPSAN []string `mapstructure:"ip_san" json:"ip_san,omitempty"`

	// AllowTLS enables the RPC endpoint on the server to answer
	// AutoEncrypt.Sign requests.
	AllowTLS *bool `mapstructure:"allow_tls" json:"allow_tls,omitempty"`
}

// Connect is the agent-global connect configuration.
type Connect struct {
	// Enabled opts the agent into connect. It should be set on all clients and
	// servers in a cluster for correct connect operation.
	Enabled                         *bool                  `mapstructure:"enabled" json:"enabled,omitempty"`
	CAProvider                      *string                `mapstructure:"ca_provider" json:"ca_provider,omitempty"`
	CAConfig                        map[string]interface{} `mapstructure:"ca_config" json:"ca_config,omitempty"`
	MeshGatewayWANFederationEnabled *bool                  `mapstructure:"enable_mesh_gateway_wan_federation" json:"enable_mesh_gateway_wan_federation,omitempty"`

	// TestCALeafRootChangeSpread controls how long after a CA roots change before new leaf certs will be generated.
	// This is only tuned in tests, generally set to 1ns to make tests deterministic with when to expect updated leaf
	// certs by. This configuration is not exposed to users (not documented, and agent/config/default.go will override it)
	TestCALeafRootChangeSpread *string `mapstructure:"test_ca_leaf_root_change_spread" json:"test_ca_leaf_root_change_spread,omitempty"`
}

// SOA is the configuration of SOA for DNS
type SOA struct {
	Refresh *uint32 `mapstructure:"refresh"`
	Retry   *uint32 `mapstructure:"retry"`
	Expire  *uint32 `mapstructure:"expire"`
	Minttl  *uint32 `mapstructure:"min_ttl"`
}

type DNS struct {
	AllowStale         *bool             `mapstructure:"allow_stale"`
	ARecordLimit       *int              `mapstructure:"a_record_limit"`
	DisableCompression *bool             `mapstructure:"disable_compression"`
	EnableTruncate     *bool             `mapstructure:"enable_truncate"`
	MaxStale           *string           `mapstructure:"max_stale"`
	NodeTTL            *string           `mapstructure:"node_ttl"`
	OnlyPassing        *bool             `mapstructure:"only_passing"`
	RecursorStrategy   *string           `mapstructure:"recursor_strategy"`
	RecursorTimeout    *string           `mapstructure:"recursor_timeout"`
	ServiceTTL         map[string]string `mapstructure:"service_ttl"`
	UDPAnswerLimit     *int              `mapstructure:"udp_answer_limit"`
	NodeMetaTXT        *bool             `mapstructure:"enable_additional_node_meta_txt"`
	SOA                *SOA              `mapstructure:"soa"`
	UseCache           *bool             `mapstructure:"use_cache"`
	CacheMaxAge        *string           `mapstructure:"cache_max_age"`

	// Enterprise Only
	PreferNamespace *bool `mapstructure:"prefer_namespace"`
}

type HTTPConfig struct {
	BlockEndpoints     []string          `mapstructure:"block_endpoints"`
	AllowWriteHTTPFrom []string          `mapstructure:"allow_write_http_from"`
	ResponseHeaders    map[string]string `mapstructure:"response_headers"`
	UseCache           *bool             `mapstructure:"use_cache"`
	MaxHeaderBytes     *int              `mapstructure:"max_header_bytes"`
}

type Performance struct {
	LeaveDrainTime *string `mapstructure:"leave_drain_time"`
	RaftMultiplier *int    `mapstructure:"raft_multiplier"` // todo(fs): validate as uint
	RPCHoldTimeout *string `mapstructure:"rpc_hold_timeout"`
}

type Telemetry struct {
	CirconusAPIApp                     *string  `mapstructure:"circonus_api_app" json:"circonus_api_app,omitempty"`
	CirconusAPIToken                   *string  `mapstructure:"circonus_api_token" json:"circonus_api_token,omitempty"`
	CirconusAPIURL                     *string  `mapstructure:"circonus_api_url" json:"circonus_api_url,omitempty"`
	CirconusBrokerID                   *string  `mapstructure:"circonus_broker_id" json:"circonus_broker_id,omitempty"`
	CirconusBrokerSelectTag            *string  `mapstructure:"circonus_broker_select_tag" json:"circonus_broker_select_tag,omitempty"`
	CirconusCheckDisplayName           *string  `mapstructure:"circonus_check_display_name" json:"circonus_check_display_name,omitempty"`
	CirconusCheckForceMetricActivation *string  `mapstructure:"circonus_check_force_metric_activation" json:"circonus_check_force_metric_activation,omitempty"`
	CirconusCheckID                    *string  `mapstructure:"circonus_check_id" json:"circonus_check_id,omitempty"`
	CirconusCheckInstanceID            *string  `mapstructure:"circonus_check_instance_id" json:"circonus_check_instance_id,omitempty"`
	CirconusCheckSearchTag             *string  `mapstructure:"circonus_check_search_tag" json:"circonus_check_search_tag,omitempty"`
	CirconusCheckTags                  *string  `mapstructure:"circonus_check_tags" json:"circonus_check_tags,omitempty"`
	CirconusSubmissionInterval         *string  `mapstructure:"circonus_submission_interval" json:"circonus_submission_interval,omitempty"`
	CirconusSubmissionURL              *string  `mapstructure:"circonus_submission_url" json:"circonus_submission_url,omitempty"`
	DisableHostname                    *bool    `mapstructure:"disable_hostname" json:"disable_hostname,omitempty"`
	DogstatsdAddr                      *string  `mapstructure:"dogstatsd_addr" json:"dogstatsd_addr,omitempty"`
	DogstatsdTags                      []string `mapstructure:"dogstatsd_tags" json:"dogstatsd_tags,omitempty"`
	RetryFailedConfiguration           *bool    `mapstructure:"retry_failed_connection" json:"retry_failed_connection,omitempty"`
	FilterDefault                      *bool    `mapstructure:"filter_default" json:"filter_default,omitempty"`
	PrefixFilter                       []string `mapstructure:"prefix_filter" json:"prefix_filter,omitempty"`
	MetricsPrefix                      *string  `mapstructure:"metrics_prefix" json:"metrics_prefix,omitempty"`
	PrometheusRetentionTime            *string  `mapstructure:"prometheus_retention_time" json:"prometheus_retention_time,omitempty"`
	StatsdAddr                         *string  `mapstructure:"statsd_address" json:"statsd_address,omitempty"`
	StatsiteAddr                       *string  `mapstructure:"statsite_address" json:"statsite_address,omitempty"`
}

type Ports struct {
	DNS            *int `mapstructure:"dns" json:"dns,omitempty"`
	HTTP           *int `mapstructure:"http" json:"http,omitempty"`
	HTTPS          *int `mapstructure:"https" json:"https,omitempty"`
	SerfLAN        *int `mapstructure:"serf_lan" json:"serf_lan,omitempty"`
	SerfWAN        *int `mapstructure:"serf_wan" json:"serf_wan,omitempty"`
	Server         *int `mapstructure:"server" json:"server,omitempty"`
	GRPC           *int `mapstructure:"grpc" json:"grpc,omitempty"`
	GRPCTLS        *int `mapstructure:"grpc_tls" json:"grpc_tls,omitempty"`
	ProxyMinPort   *int `mapstructure:"proxy_min_port" json:"proxy_min_port,omitempty"`
	ProxyMaxPort   *int `mapstructure:"proxy_max_port" json:"proxy_max_port,omitempty"`
	SidecarMinPort *int `mapstructure:"sidecar_min_port" json:"sidecar_min_port,omitempty"`
	SidecarMaxPort *int `mapstructure:"sidecar_max_port" json:"sidecar_max_port,omitempty"`
	ExposeMinPort  *int `mapstructure:"expose_min_port" json:"expose_min_port,omitempty" `
	ExposeMaxPort  *int `mapstructure:"expose_max_port" json:"expose_max_port,omitempty"`
}

type UnixSocket struct {
	Group *string `mapstructure:"group"`
	Mode  *string `mapstructure:"mode"`
	User  *string `mapstructure:"user"`
}

type RequestLimits struct {
	Mode      *string  `mapstructure:"mode"`
	ReadRate  *float64 `mapstructure:"read_rate"`
	WriteRate *float64 `mapstructure:"write_rate"`
}

type Limits struct {
	HTTPMaxConnsPerClient *int          `mapstructure:"http_max_conns_per_client"`
	HTTPSHandshakeTimeout *string       `mapstructure:"https_handshake_timeout"`
	RequestLimits         RequestLimits `mapstructure:"request_limits"`
	RPCClientTimeout      *string       `mapstructure:"rpc_client_timeout"`
	RPCHandshakeTimeout   *string       `mapstructure:"rpc_handshake_timeout"`
	RPCMaxBurst           *int          `mapstructure:"rpc_max_burst"`
	RPCMaxConnsPerClient  *int          `mapstructure:"rpc_max_conns_per_client"`
	RPCRate               *float64      `mapstructure:"rpc_rate"`
	KVMaxValueSize        *uint64       `mapstructure:"kv_max_value_size"`
	TxnMaxReqLen          *uint64       `mapstructure:"txn_max_req_len"`
}

type Segment struct {
	Advertise   *string `mapstructure:"advertise"`
	Bind        *string `mapstructure:"bind"`
	Name        *string `mapstructure:"name"`
	Port        *int    `mapstructure:"port"`
	RPCListener *bool   `mapstructure:"rpc_listener"`
}

type ACL struct {
	Enabled                *bool   `mapstructure:"enabled"`
	TokenReplication       *bool   `mapstructure:"enable_token_replication"`
	PolicyTTL              *string `mapstructure:"policy_ttl"`
	RoleTTL                *string `mapstructure:"role_ttl"`
	TokenTTL               *string `mapstructure:"token_ttl"`
	DownPolicy             *string `mapstructure:"down_policy"`
	DefaultPolicy          *string `mapstructure:"default_policy"`
	EnableKeyListPolicy    *bool   `mapstructure:"enable_key_list_policy"`
	Tokens                 Tokens  `mapstructure:"tokens"`
	EnableTokenPersistence *bool   `mapstructure:"enable_token_persistence"`

	// Enterprise Only
	MSPDisableBootstrap *bool `mapstructure:"msp_disable_bootstrap"`
}

type Tokens struct {
	InitialManagement      *string `mapstructure:"initial_management"`
	Replication            *string `mapstructure:"replication"`
	AgentRecovery          *string `mapstructure:"agent_recovery"`
	Default                *string `mapstructure:"default"`
	Agent                  *string `mapstructure:"agent"`
	ConfigFileRegistration *string `mapstructure:"config_file_service_registration"`

	// Enterprise Only
	ManagedServiceProvider []ServiceProviderToken `mapstructure:"managed_service_provider"`

	DeprecatedTokens `mapstructure:",squash"`
}

type DeprecatedTokens struct {
	// DEPRECATED (ACL) - renamed to "initial_management"
	Master *string `mapstructure:"master"`
	// DEPRECATED (ACL) - renamed to "agent_recovery"
	AgentMaster *string `mapstructure:"agent_master"`
}

// ServiceProviderToken groups an accessor and secret for a service provider token. Enterprise Only
type ServiceProviderToken struct {
	AccessorID *string `mapstructure:"accessor_id"`
	SecretID   *string `mapstructure:"secret_id"`
}

type ConfigEntries struct {
	// Bootstrap is the list of config_entries that should only be persisted to
	// cluster on initial startup of a new leader if no such config exists
	// already. The type is map not structs.ConfigEntry for decoding reasons - we
	// need to figure out the right concrete type before we can decode it
	// unabiguously.
	Bootstrap []map[string]interface{} `mapstructure:"bootstrap"`
}

// Audit allows us to enable and define destinations for auditing
type Audit struct {
	Enabled *bool                `mapstructure:"enabled"`
	Sinks   map[string]AuditSink `mapstructure:"sink"`
}

// AuditSink can be provided multiple times to define pipelines for auditing
type AuditSink struct {
	Type              *string `mapstructure:"type"`
	Format            *string `mapstructure:"format"`
	Path              *string `mapstructure:"path"`
	DeliveryGuarantee *string `mapstructure:"delivery_guarantee"`
	Mode              *string `mapstructure:"mode"`
	RotateBytes       *int    `mapstructure:"rotate_bytes"`
	RotateDuration    *string `mapstructure:"rotate_duration"`
	RotateMaxFiles    *int    `mapstructure:"rotate_max_files"`
}

type AutoConfigRaw struct {
	Enabled         *bool                      `mapstructure:"enabled"`
	IntroToken      *string                    `mapstructure:"intro_token"`
	IntroTokenFile  *string                    `mapstructure:"intro_token_file"`
	ServerAddresses []string                   `mapstructure:"server_addresses"`
	DNSSANs         []string                   `mapstructure:"dns_sans"`
	IPSANs          []string                   `mapstructure:"ip_sans"`
	Authorization   AutoConfigAuthorizationRaw `mapstructure:"authorization"`
}

type AutoConfigAuthorizationRaw struct {
	Enabled *bool                   `mapstructure:"enabled"`
	Static  AutoConfigAuthorizerRaw `mapstructure:"static"`
}

type AutoConfigAuthorizerRaw struct {
	ClaimAssertions []string `mapstructure:"claim_assertions"`
	AllowReuse      *bool    `mapstructure:"allow_reuse"`

	// Fields to be shared with the JWT Auth Method
	JWTSupportedAlgs     []string          `mapstructure:"jwt_supported_algs"`
	BoundAudiences       []string          `mapstructure:"bound_audiences"`
	ClaimMappings        map[string]string `mapstructure:"claim_mappings"`
	ListClaimMappings    map[string]string `mapstructure:"list_claim_mappings"`
	OIDCDiscoveryURL     *string           `mapstructure:"oidc_discovery_url"`
	OIDCDiscoveryCACert  *string           `mapstructure:"oidc_discovery_ca_cert"`
	JWKSURL              *string           `mapstructure:"jwks_url"`
	JWKSCACert           *string           `mapstructure:"jwks_ca_cert"`
	JWTValidationPubKeys []string          `mapstructure:"jwt_validation_pub_keys"`
	BoundIssuer          *string           `mapstructure:"bound_issuer"`
	ExpirationLeeway     *string           `mapstructure:"expiration_leeway"`
	NotBeforeLeeway      *string           `mapstructure:"not_before_leeway"`
	ClockSkewLeeway      *string           `mapstructure:"clock_skew_leeway"`
}

type RawUIConfig struct {
	Enabled                    *bool             `mapstructure:"enabled"`
	Dir                        *string           `mapstructure:"dir"`
	ContentPath                *string           `mapstructure:"content_path"`
	MetricsProvider            *string           `mapstructure:"metrics_provider"`
	MetricsProviderFiles       []string          `mapstructure:"metrics_provider_files"`
	MetricsProviderOptionsJSON *string           `mapstructure:"metrics_provider_options_json"`
	MetricsProxy               RawUIMetricsProxy `mapstructure:"metrics_proxy"`
	DashboardURLTemplates      map[string]string `mapstructure:"dashboard_url_templates"`
}

type RawUIMetricsProxy struct {
	BaseURL       *string                      `mapstructure:"base_url"`
	AddHeaders    []RawUIMetricsProxyAddHeader `mapstructure:"add_headers"`
	PathAllowlist []string                     `mapstructure:"path_allowlist"`
}

type RawUIMetricsProxyAddHeader struct {
	Name  *string `mapstructure:"name"`
	Value *string `mapstructure:"value"`
}

type RPC struct {
	EnableStreaming *bool `mapstructure:"enable_streaming"`
}

type CloudConfigRaw struct {
	ResourceID   *string `mapstructure:"resource_id"`
	ClientID     *string `mapstructure:"client_id"`
	ClientSecret *string `mapstructure:"client_secret"`
	Hostname     *string `mapstructure:"hostname"`
	AuthURL      *string `mapstructure:"auth_url"`
	ScadaAddress *string `mapstructure:"scada_address"`
}

type TLSProtocolConfig struct {
	CAFile               *string `mapstructure:"ca_file" json:"ca_file,omitempty"`
	CAPath               *string `mapstructure:"ca_path" json:"ca_path,omitempty"`
	CertFile             *string `mapstructure:"cert_file" json:"cert_file,omitempty"`
	KeyFile              *string `mapstructure:"key_file" json:"key_file,omitempty"`
	TLSMinVersion        *string `mapstructure:"tls_min_version" json:"tls_min_version,omitempty"`
	TLSCipherSuites      *string `mapstructure:"tls_cipher_suites" json:"tls_cipher_suites,omitempty"`
	VerifyIncoming       *bool   `mapstructure:"verify_incoming" json:"verify_incoming,omitempty"`
	VerifyOutgoing       *bool   `mapstructure:"verify_outgoing" json:"verify_outgoing,omitempty"`
	VerifyServerHostname *bool   `mapstructure:"verify_server_hostname" json:"verify_server_hostname,omitempty"`
	UseAutoCert          *bool   `mapstructure:"use_auto_cert" json:"use_auto_cert,omitempty"`
}

type TLS struct {
	Defaults    TLSProtocolConfig `mapstructure:"defaults" json:"defaults,omitempty"`
	InternalRPC TLSProtocolConfig `mapstructure:"internal_rpc" json:"internal_rpc,omitempty"`
	HTTPS       TLSProtocolConfig `mapstructure:"https" json:"https,omitempty"`
	GRPC        TLSProtocolConfig `mapstructure:"grpc" json:"grpc,omitempty"`

	// GRPCModifiedByDeprecatedConfig is a flag used to indicate that GRPC was
	// modified by the deprecated field mapping (as apposed to a user-provided
	// a grpc stanza). This prevents us from emitting a warning about an
	// ineffectual grpc stanza when we modify GRPC to honor the legacy behaviour
	// that setting `verify_incoming = true` at the top-level *does not* enable
	// client certificate verification on the gRPC port.
	//
	// See: applyDeprecatedTLSConfig.
	//
	// Note: we use a *struct{} here because a simple bool isn't supported by our
	// config merging logic.
	GRPCModifiedByDeprecatedConfig *struct{} `mapstructure:"-" json:"-"`
}

type Peering struct {
	Enabled *bool `mapstructure:"enabled" json:"enabled,omitempty"`

	// TestAllowPeerRegistrations controls whether CatalogRegister endpoints allow registrations for objects with `PeerName`
	// This always gets overridden in NonUserSource()
	TestAllowPeerRegistrations *bool `mapstructure:"test_allow_peer_registrations" json:"test_allow_peer_registrations,omitempty"`
}

type XDS struct {
	UpdateMaxPerSecond *float64 `mapstructure:"update_max_per_second"`
}

type RaftLogStoreRaw struct {
	Backend         *string `mapstructure:"backend" json:"backend,omitempty"`
	DisableLogCache *bool   `mapstructure:"disable_log_cache" json:"disable_log_cache,omitempty"`

	Verification RaftLogStoreVerificationRaw `mapstructure:"verification" json:"verification,omitempty"`

	BoltDBConfig RaftBoltDBConfigRaw `mapstructure:"boltdb" json:"boltdb,omitempty"`

	WALConfig RaftWALConfigRaw `mapstructure:"wal" json:"wal,omitempty"`
}

type RaftLogStoreVerificationRaw struct {
	Enabled  *bool   `mapstructure:"enabled" json:"enabled,omitempty"`
	Interval *string `mapstructure:"interval" json:"interval,omitempty"`
}

type RaftBoltDBConfigRaw struct {
	NoFreelistSync *bool `mapstructure:"no_freelist_sync" json:"no_freelist_sync,omitempty"`
}

type RaftWALConfigRaw struct {
	SegmentSizeMB *int `mapstructure:"segment_size_mb" json:"segment_size_mb,omitempty"`
}

type License struct {
	Enabled *bool `mapstructure:"enabled"`
}

type Reporting struct {
	License License `mapstructure:"license"`
}
