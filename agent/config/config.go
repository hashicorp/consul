package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/consul/agent/consul"

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

	// FromUser indicates whether the the file source was provided by the user.
	// This distinguishes from synthetic file sources that Consul will generate.
	FromUser bool
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

	c, warns := applyDeprecatedConfig(&target, f.FromUser)
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
	ACL                              ACL                 `mapstructure:"acl"`
	Addresses                        Addresses           `mapstructure:"addresses"`
	AdvertiseAddrLAN                 *string             `mapstructure:"advertise_addr"`
	AdvertiseAddrLANIPv4             *string             `mapstructure:"advertise_addr_ipv4"`
	AdvertiseAddrLANIPv6             *string             `mapstructure:"advertise_addr_ipv6"`
	AdvertiseAddrWAN                 *string             `mapstructure:"advertise_addr_wan"`
	AdvertiseAddrWANIPv4             *string             `mapstructure:"advertise_addr_wan_ipv4"`
	AdvertiseAddrWANIPv6             *string             `mapstructure:"advertise_addr_wan_ipv6"`
	AdvertiseReconnectTimeout        *string             `mapstructure:"advertise_reconnect_timeout"`
	AutoConfig                       AutoConfigRaw       `mapstructure:"auto_config"`
	Autopilot                        Autopilot           `mapstructure:"autopilot"`
	BindAddr                         *string             `mapstructure:"bind_addr"`
	Bootstrap                        *bool               `mapstructure:"bootstrap"`
	BootstrapExpect                  *int                `mapstructure:"bootstrap_expect"`
	Cache                            Cache               `mapstructure:"cache"`
	Check                            *CheckDefinition    `mapstructure:"check"` // needs to be a pointer to avoid partial merges
	CheckOutputMaxSize               *int                `mapstructure:"check_output_max_size"`
	CheckUpdateInterval              *string             `mapstructure:"check_update_interval"`
	Checks                           []CheckDefinition   `mapstructure:"checks"`
	ClientAddr                       *string             `mapstructure:"client_addr"`
	ConfigEntries                    ConfigEntries       `mapstructure:"config_entries"`
	AutoEncrypt                      AutoEncrypt         `mapstructure:"auto_encrypt"`
	Connect                          Connect             `mapstructure:"connect"`
	DNS                              DNS                 `mapstructure:"dns_config"`
	DNSDomain                        *string             `mapstructure:"domain"`
	DNSAltDomain                     *string             `mapstructure:"alt_domain"`
	DNSRecursors                     []string            `mapstructure:"recursors"`
	DataDir                          *string             `mapstructure:"data_dir"`
	Datacenter                       *string             `mapstructure:"datacenter"`
	DefaultQueryTime                 *string             `mapstructure:"default_query_time"`
	DisableAnonymousSignature        *bool               `mapstructure:"disable_anonymous_signature"`
	DisableCoordinates               *bool               `mapstructure:"disable_coordinates"`
	DisableHostNodeID                *bool               `mapstructure:"disable_host_node_id"`
	DisableHTTPUnprintableCharFilter *bool               `mapstructure:"disable_http_unprintable_char_filter"`
	DisableKeyringFile               *bool               `mapstructure:"disable_keyring_file"`
	DisableRemoteExec                *bool               `mapstructure:"disable_remote_exec"`
	DisableUpdateCheck               *bool               `mapstructure:"disable_update_check"`
	DiscardCheckOutput               *bool               `mapstructure:"discard_check_output"`
	DiscoveryMaxStale                *string             `mapstructure:"discovery_max_stale"`
	EnableAgentTLSForChecks          *bool               `mapstructure:"enable_agent_tls_for_checks"`
	EnableCentralServiceConfig       *bool               `mapstructure:"enable_central_service_config"`
	EnableDebug                      *bool               `mapstructure:"enable_debug"`
	EnableScriptChecks               *bool               `mapstructure:"enable_script_checks"`
	EnableLocalScriptChecks          *bool               `mapstructure:"enable_local_script_checks"`
	EnableSyslog                     *bool               `mapstructure:"enable_syslog"`
	EncryptKey                       *string             `mapstructure:"encrypt"`
	EncryptVerifyIncoming            *bool               `mapstructure:"encrypt_verify_incoming"`
	EncryptVerifyOutgoing            *bool               `mapstructure:"encrypt_verify_outgoing"`
	GossipLAN                        GossipLANConfig     `mapstructure:"gossip_lan"`
	GossipWAN                        GossipWANConfig     `mapstructure:"gossip_wan"`
	HTTPConfig                       HTTPConfig          `mapstructure:"http_config"`
	LeaveOnTerm                      *bool               `mapstructure:"leave_on_terminate"`
	LicensePath                      *string             `mapstructure:"license_path"`
	Limits                           Limits              `mapstructure:"limits"`
	LogLevel                         *string             `mapstructure:"log_level"`
	LogJSON                          *bool               `mapstructure:"log_json"`
	LogFile                          *string             `mapstructure:"log_file"`
	LogRotateDuration                *string             `mapstructure:"log_rotate_duration"`
	LogRotateBytes                   *int                `mapstructure:"log_rotate_bytes"`
	LogRotateMaxFiles                *int                `mapstructure:"log_rotate_max_files"`
	MaxQueryTime                     *string             `mapstructure:"max_query_time"`
	NodeID                           *string             `mapstructure:"node_id"`
	NodeMeta                         map[string]string   `mapstructure:"node_meta"`
	NodeName                         *string             `mapstructure:"node_name"`
	Peering                          Peering             `mapstructure:"peering"`
	Performance                      Performance         `mapstructure:"performance"`
	PidFile                          *string             `mapstructure:"pid_file"`
	Ports                            Ports               `mapstructure:"ports"`
	PrimaryDatacenter                *string             `mapstructure:"primary_datacenter"`
	PrimaryGateways                  []string            `mapstructure:"primary_gateways"`
	PrimaryGatewaysInterval          *string             `mapstructure:"primary_gateways_interval"`
	RPCProtocol                      *int                `mapstructure:"protocol"`
	RaftProtocol                     *int                `mapstructure:"raft_protocol"`
	RaftSnapshotThreshold            *int                `mapstructure:"raft_snapshot_threshold"`
	RaftSnapshotInterval             *string             `mapstructure:"raft_snapshot_interval"`
	RaftTrailingLogs                 *int                `mapstructure:"raft_trailing_logs"`
	ReconnectTimeoutLAN              *string             `mapstructure:"reconnect_timeout"`
	ReconnectTimeoutWAN              *string             `mapstructure:"reconnect_timeout_wan"`
	RejoinAfterLeave                 *bool               `mapstructure:"rejoin_after_leave"`
	AutoReloadConfig                 *bool               `mapstructure:"auto_reload_config"`
	RetryJoinIntervalLAN             *string             `mapstructure:"retry_interval"`
	RetryJoinIntervalWAN             *string             `mapstructure:"retry_interval_wan"`
	RetryJoinLAN                     []string            `mapstructure:"retry_join"`
	RetryJoinMaxAttemptsLAN          *int                `mapstructure:"retry_max"`
	RetryJoinMaxAttemptsWAN          *int                `mapstructure:"retry_max_wan"`
	RetryJoinWAN                     []string            `mapstructure:"retry_join_wan"`
	SerfAllowedCIDRsLAN              []string            `mapstructure:"serf_lan_allowed_cidrs"`
	SerfAllowedCIDRsWAN              []string            `mapstructure:"serf_wan_allowed_cidrs"`
	SerfBindAddrLAN                  *string             `mapstructure:"serf_lan"`
	SerfBindAddrWAN                  *string             `mapstructure:"serf_wan"`
	ServerMode                       *bool               `mapstructure:"server"`
	ServerName                       *string             `mapstructure:"server_name"`
	Service                          *ServiceDefinition  `mapstructure:"service"`
	Services                         []ServiceDefinition `mapstructure:"services"`
	SessionTTLMin                    *string             `mapstructure:"session_ttl_min"`
	SkipLeaveOnInt                   *bool               `mapstructure:"skip_leave_on_interrupt"`
	StartJoinAddrsLAN                []string            `mapstructure:"start_join"`
	StartJoinAddrsWAN                []string            `mapstructure:"start_join_wan"`
	SyslogFacility                   *string             `mapstructure:"syslog_facility"`
	TLS                              TLS                 `mapstructure:"tls"`
	TaggedAddresses                  map[string]string   `mapstructure:"tagged_addresses"`
	Telemetry                        Telemetry           `mapstructure:"telemetry"`
	TranslateWANAddrs                *bool               `mapstructure:"translate_wan_addrs"`

	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UI *bool `mapstructure:"ui"`
	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UIContentPath *string `mapstructure:"ui_content_path"`
	// DEPRECATED (ui-config) - moved to the ui_config stanza
	UIDir    *string     `mapstructure:"ui_dir"`
	UIConfig RawUIConfig `mapstructure:"ui_config"`

	UnixSocket UnixSocket               `mapstructure:"unix_sockets"`
	Watches    []map[string]interface{} `mapstructure:"watches"`

	RPC RPC `mapstructure:"rpc"`

	RaftBoltDBConfig *consul.RaftBoltDBConfig `mapstructure:"raft_boltdb"`

	// UseStreamingBackend instead of blocking queries for service health and
	// any other endpoints which support streaming.
	UseStreamingBackend *bool `mapstructure:"use_streaming_backend"`

	// This isn't used by Consul but we've documented a feature where users
	// can deploy their snapshot agent configs alongside their Consul configs
	// so we have a placeholder here so it can be parsed but this doesn't
	// manifest itself in any way inside the runtime config.
	SnapshotAgent map[string]interface{} `mapstructure:"snapshot_agent"`

	// non-user configurable values
	AEInterval                 *string    `mapstructure:"ae_interval"`
	CheckDeregisterIntervalMin *string    `mapstructure:"check_deregister_interval_min"`
	CheckReapInterval          *string    `mapstructure:"check_reap_interval"`
	Consul                     Consul     `mapstructure:"consul"`
	Revision                   *string    `mapstructure:"revision"`
	SegmentLimit               *int       `mapstructure:"segment_limit"`
	SegmentNameLimit           *int       `mapstructure:"segment_name_limit"`
	SyncCoordinateIntervalMin  *string    `mapstructure:"sync_coordinate_interval_min"`
	SyncCoordinateRateTarget   *float64   `mapstructure:"sync_coordinate_rate_target"`
	Version                    *string    `mapstructure:"version"`
	VersionPrerelease          *string    `mapstructure:"version_prerelease"`
	VersionMetadata            *string    `mapstructure:"version_metadata"`
	BuildDate                  *time.Time `mapstructure:"build_date"`

	// Enterprise Only
	Audit Audit `mapstructure:"audit"`
	// Enterprise Only
	ReadReplica *bool `mapstructure:"read_replica" alias:"non_voting_server"`
	// Enterprise Only
	SegmentName *string `mapstructure:"segment"`
	// Enterprise Only
	Segments []Segment `mapstructure:"segments"`
	// Enterprise Only
	Partition *string `mapstructure:"partition"`

	// Enterprise Only - not user configurable
	LicensePollBaseTime   *string `mapstructure:"license_poll_base_time"`
	LicensePollMaxTime    *string `mapstructure:"license_poll_max_time"`
	LicenseUpdateBaseTime *string `mapstructure:"license_update_base_time"`
	LicenseUpdateMaxTime  *string `mapstructure:"license_update_max_time"`

	// license reporting
	Reporting Reporting `mapstructure:"reporting"`
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
	DNS   *string `mapstructure:"dns"`
	HTTP  *string `mapstructure:"http"`
	HTTPS *string `mapstructure:"https"`
	GRPC  *string `mapstructure:"grpc"`
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
	TLS *bool `mapstructure:"tls"`

	// Additional DNS SAN entries that clients request for their certificates.
	DNSSAN []string `mapstructure:"dns_san"`

	// Additional IP SAN entries that clients request for their certificates.
	IPSAN []string `mapstructure:"ip_san"`

	// AllowTLS enables the RPC endpoint on the server to answer
	// AutoEncrypt.Sign requests.
	AllowTLS *bool `mapstructure:"allow_tls"`
}

// Connect is the agent-global connect configuration.
type Connect struct {
	// Enabled opts the agent into connect. It should be set on all clients and
	// servers in a cluster for correct connect operation.
	Enabled                         *bool                  `mapstructure:"enabled"`
	CAProvider                      *string                `mapstructure:"ca_provider"`
	CAConfig                        map[string]interface{} `mapstructure:"ca_config"`
	MeshGatewayWANFederationEnabled *bool                  `mapstructure:"enable_mesh_gateway_wan_federation"`
	EnableServerlessPlugin          *bool                  `mapstructure:"enable_serverless_plugin"`

	// TestCALeafRootChangeSpread controls how long after a CA roots change before new leaf certs will be generated.
	// This is only tuned in tests, generally set to 1ns to make tests deterministic with when to expect updated leaf
	// certs by. This configuration is not exposed to users (not documented, and agent/config/default.go will override it)
	TestCALeafRootChangeSpread *string `mapstructure:"test_ca_leaf_root_change_spread"`
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
	CirconusAPIApp                     *string  `mapstructure:"circonus_api_app"`
	CirconusAPIToken                   *string  `mapstructure:"circonus_api_token"`
	CirconusAPIURL                     *string  `mapstructure:"circonus_api_url"`
	CirconusBrokerID                   *string  `mapstructure:"circonus_broker_id"`
	CirconusBrokerSelectTag            *string  `mapstructure:"circonus_broker_select_tag"`
	CirconusCheckDisplayName           *string  `mapstructure:"circonus_check_display_name"`
	CirconusCheckForceMetricActivation *string  `mapstructure:"circonus_check_force_metric_activation"`
	CirconusCheckID                    *string  `mapstructure:"circonus_check_id"`
	CirconusCheckInstanceID            *string  `mapstructure:"circonus_check_instance_id"`
	CirconusCheckSearchTag             *string  `mapstructure:"circonus_check_search_tag"`
	CirconusCheckTags                  *string  `mapstructure:"circonus_check_tags"`
	CirconusSubmissionInterval         *string  `mapstructure:"circonus_submission_interval"`
	CirconusSubmissionURL              *string  `mapstructure:"circonus_submission_url"`
	DisableHostname                    *bool    `mapstructure:"disable_hostname"`
	DogstatsdAddr                      *string  `mapstructure:"dogstatsd_addr"`
	DogstatsdTags                      []string `mapstructure:"dogstatsd_tags"`
	RetryFailedConfiguration           *bool    `mapstructure:"retry_failed_connection"`
	FilterDefault                      *bool    `mapstructure:"filter_default"`
	PrefixFilter                       []string `mapstructure:"prefix_filter"`
	MetricsPrefix                      *string  `mapstructure:"metrics_prefix"`
	PrometheusRetentionTime            *string  `mapstructure:"prometheus_retention_time"`
	StatsdAddr                         *string  `mapstructure:"statsd_address"`
	StatsiteAddr                       *string  `mapstructure:"statsite_address"`
}

type Ports struct {
	DNS            *int `mapstructure:"dns"`
	HTTP           *int `mapstructure:"http"`
	HTTPS          *int `mapstructure:"https"`
	SerfLAN        *int `mapstructure:"serf_lan"`
	SerfWAN        *int `mapstructure:"serf_wan"`
	Server         *int `mapstructure:"server"`
	GRPC           *int `mapstructure:"grpc"`
	ProxyMinPort   *int `mapstructure:"proxy_min_port"`
	ProxyMaxPort   *int `mapstructure:"proxy_max_port"`
	SidecarMinPort *int `mapstructure:"sidecar_min_port"`
	SidecarMaxPort *int `mapstructure:"sidecar_max_port"`
	ExposeMinPort  *int `mapstructure:"expose_min_port"`
	ExposeMaxPort  *int `mapstructure:"expose_max_port"`
}

type UnixSocket struct {
	Group *string `mapstructure:"group"`
	Mode  *string `mapstructure:"mode"`
	User  *string `mapstructure:"user"`
}

type Limits struct {
	HTTPMaxConnsPerClient *int     `mapstructure:"http_max_conns_per_client"`
	HTTPSHandshakeTimeout *string  `mapstructure:"https_handshake_timeout"`
	RPCClientTimeout      *string  `mapstructure:"rpc_client_timeout"`
	RPCHandshakeTimeout   *string  `mapstructure:"rpc_handshake_timeout"`
	RPCMaxBurst           *int     `mapstructure:"rpc_max_burst"`
	RPCMaxConnsPerClient  *int     `mapstructure:"rpc_max_conns_per_client"`
	RPCRate               *float64 `mapstructure:"rpc_rate"`
	KVMaxValueSize        *uint64  `mapstructure:"kv_max_value_size"`
	TxnMaxReqLen          *uint64  `mapstructure:"txn_max_req_len"`
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
	InitialManagement *string `mapstructure:"initial_management"`
	Replication       *string `mapstructure:"replication"`
	AgentRecovery     *string `mapstructure:"agent_recovery"`
	Default           *string `mapstructure:"default"`
	Agent             *string `mapstructure:"agent"`

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

type TLSProtocolConfig struct {
	CAFile               *string `mapstructure:"ca_file"`
	CAPath               *string `mapstructure:"ca_path"`
	CertFile             *string `mapstructure:"cert_file"`
	KeyFile              *string `mapstructure:"key_file"`
	TLSMinVersion        *string `mapstructure:"tls_min_version"`
	TLSCipherSuites      *string `mapstructure:"tls_cipher_suites"`
	VerifyIncoming       *bool   `mapstructure:"verify_incoming"`
	VerifyOutgoing       *bool   `mapstructure:"verify_outgoing"`
	VerifyServerHostname *bool   `mapstructure:"verify_server_hostname"`
	UseAutoCert          *bool   `mapstructure:"use_auto_cert"`
}

func (c TLSProtocolConfig) IsZero() bool {
	v := reflect.ValueOf(c)

	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).IsNil() {
			return false
		}
	}
	return true
}

type TLS struct {
	Defaults    TLSProtocolConfig `mapstructure:"defaults"`
	InternalRPC TLSProtocolConfig `mapstructure:"internal_rpc"`
	HTTPS       TLSProtocolConfig `mapstructure:"https"`
	GRPC        TLSProtocolConfig `mapstructure:"grpc"`

	// SpecifiedTLSStanza indicates whether the per-protocol tls stanza from configuration was used.
	// If unspecified, and TLS is configured, that implies that the deprecated flags were used.
	// The flag was added exclusively for the 1.13 patch series for backwards compatibility purposes.
	SpecifiedTLSStanza *bool `mapstructure:"-"`

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
	GRPCModifiedByDeprecatedConfig *struct{} `mapstructure:"-"`
}

// ContainsDefaults indicates whether the user-settable values in this type are the defaults.
func (t *TLS) ContainsDefaults() bool {
	return t.Defaults.IsZero() && t.InternalRPC.IsZero() && t.HTTPS.IsZero() && t.GRPC.IsZero()
}

type Peering struct {
	Enabled *bool `mapstructure:"enabled"`

	// TestAllowPeerRegistrations controls whether CatalogRegister endpoints allow registrations for objects with `PeerName`
	// This always gets overridden in NonUserSource()
	TestAllowPeerRegistrations *bool `mapstructure:"test_allow_peer_registrations"`
}

type License struct {
	Enabled *bool `mapstructure:"enabled"`
}

type Reporting struct {
	License License `mapstructure:"license"`
}
