package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	"golang.org/x/time/rate"
)

// RuntimeConfig specifies the configuration the consul agent actually
// uses. Is is derived from one or more Config structures which can come
// from files, flags and/or environment variables.
type RuntimeConfig struct {
	// non-user configurable values
	AEInterval time.Duration

	// ACLDisabledTTL is used by clients to determine how long they will
	// wait to check again with the servers if they discover ACLs are not
	// enabled. (not user configurable)
	//
	// hcl: acl_disabled_ttl = "duration"
	ACLDisabledTTL time.Duration

	CheckDeregisterIntervalMin time.Duration
	CheckReapInterval          time.Duration
	SegmentLimit               int
	SegmentNameLimit           int
	SyncCoordinateRateTarget   float64
	SyncCoordinateIntervalMin  time.Duration
	Revision                   string
	Version                    string
	VersionPrerelease          string

	// consul config
	ConsulCoordinateUpdateMaxBatches int
	ConsulCoordinateUpdateBatchSize  int
	ConsulCoordinateUpdatePeriod     time.Duration
	ConsulRaftElectionTimeout        time.Duration
	ConsulRaftHeartbeatTimeout       time.Duration
	ConsulRaftLeaderLeaseTimeout     time.Duration
	ConsulSerfLANGossipInterval      time.Duration
	ConsulSerfLANProbeInterval       time.Duration
	ConsulSerfLANProbeTimeout        time.Duration
	ConsulSerfLANSuspicionMult       int
	ConsulSerfWANGossipInterval      time.Duration
	ConsulSerfWANProbeInterval       time.Duration
	ConsulSerfWANProbeTimeout        time.Duration
	ConsulSerfWANSuspicionMult       int
	ConsulServerHealthInterval       time.Duration

	// ACLAgentMasterToken is a special token that has full read and write
	// privileges for this agent, and can be used to call agent endpoints
	// when no servers are available.
	//
	// hcl: acl_agent_master_token = string
	ACLAgentMasterToken string

	// ACLAgentToken is the default token used to make requests for the agent
	// itself, such as for registering itself with the catalog. If not
	// configured, the 'acl_token' will be used.
	//
	// hcl: acl_agent_token = string
	ACLAgentToken string

	// ACLDatacenter is the central datacenter that holds authoritative
	// ACL records. This must be the same for the entire cluster.
	// If this is not set, ACLs are not enabled. Off by default.
	//
	// hcl: acl_datacenter = string
	ACLDatacenter string

	// ACLDefaultPolicy is used to control the ACL interaction when
	// there is no defined policy. This can be "allow" which means
	// ACLs are used to black-list, or "deny" which means ACLs are
	// white-lists.
	//
	// hcl: acl_default_policy = ("allow"|"deny")
	ACLDefaultPolicy string

	// ACLDownPolicy is used to control the ACL interaction when we cannot
	// reach the ACLDatacenter and the token is not in the cache.
	// There are the following modes:
	//   * allow - Allow all requests
	//   * deny - Deny all requests
	//   * extend-cache - Ignore the cache expiration, and allow cached
	//                    ACL's to be used to service requests. This
	//                    is the default. If the ACL is not in the cache,
	//                    this acts like deny.
	//
	// hcl: acl_down_policy = ("allow"|"deny"|"extend-cache")
	ACLDownPolicy string

	// ACLEnforceVersion8 is used to gate a set of ACL policy features that
	// are opt-in prior to Consul 0.8 and opt-out in Consul 0.8 and later.
	//
	// hcl: acl_enforce_version_8 = (true|false)
	ACLEnforceVersion8 bool

	// ACLEnableKeyListPolicy is used to opt-in to the "list" policy added to
	// KV ACLs in Consul 1.0.
	//
	// See https://www.consul.io/docs/guides/acl.html#list-policy-for-keys for
	// more details.
	//
	// hcl: acl_enable_key_list_policy = (true|false)
	ACLEnableKeyListPolicy bool

	// ACLMasterToken is used to bootstrap the ACL system. It should be specified
	// on the servers in the ACLDatacenter. When the leader comes online, it ensures
	// that the Master token is available. This provides the initial token.
	//
	// hcl: acl_master_token = string
	ACLMasterToken string

	// ACLReplicationToken is used to fetch ACLs from the ACLDatacenter in
	// order to replicate them locally. Setting this to a non-empty value
	// also enables replication. Replication is only available in datacenters
	// other than the ACLDatacenter.
	//
	// hcl: acl_replication_token = string
	ACLReplicationToken string

	// ACLTTL is used to control the time-to-live of cached ACLs . This has
	// a major impact on performance. By default, it is set to 30 seconds.
	//
	// hcl: acl_ttl = "duration"
	ACLTTL time.Duration

	// ACLToken is the default token used to make requests if a per-request
	// token is not provided. If not configured the 'anonymous' token is used.
	//
	// hcl: acl_token = string
	ACLToken string

	// AutopilotCleanupDeadServers enables the automatic cleanup of dead servers when new ones
	// are added to the peer list. Defaults to true.
	//
	// hcl: autopilot { cleanup_dead_servers = (true|false) }
	AutopilotCleanupDeadServers bool

	// AutopilotDisableUpgradeMigration will disable Autopilot's upgrade migration
	// strategy of waiting until enough newer-versioned servers have been added to the
	// cluster before promoting them to voters. (Enterprise-only)
	//
	// hcl: autopilot { disable_upgrade_migration = (true|false)
	AutopilotDisableUpgradeMigration bool

	// AutopilotLastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	//
	// hcl: autopilot { last_contact_threshold = "duration" }
	AutopilotLastContactThreshold time.Duration

	// AutopilotMaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy. The value must be positive.
	//
	// hcl: autopilot { max_trailing_logs = int }
	AutopilotMaxTrailingLogs int

	// AutopilotRedundancyZoneTag is the Meta tag to use for separating servers
	// into zones for redundancy. If left blank, this feature will be disabled.
	// (Enterprise-only)
	//
	// hcl: autopilot { redundancy_zone_tag = string }
	AutopilotRedundancyZoneTag string

	// AutopilotServerStabilizationTime is the minimum amount of time a server must be
	// in a stable, healthy state before it can be added to the cluster. Only
	// applicable with Raft protocol version 3 or higher.
	//
	// hcl: autopilot { server_stabilization_time = "duration" }
	AutopilotServerStabilizationTime time.Duration

	// AutopilotUpgradeVersionTag is the node tag to use for version info when
	// performing upgrade migrations. If left blank, the Consul version will be used.
	//
	// (Entrprise-only)
	//
	// hcl: autopilot { upgrade_version_tag = string }
	AutopilotUpgradeVersionTag string

	// DNSAllowStale is used to enable lookups with stale
	// data. This gives horizontal read scalability since
	// any Consul server can service the query instead of
	// only the leader.
	//
	// hcl: dns_config { allow_stale = (true|false) }
	DNSAllowStale bool

	// DNSDisableCompression is used to control whether DNS responses are
	// compressed. In Consul 0.7 this was turned on by default and this
	// config was added as an opt-out.
	//
	// hcl: dns_config { disable_compression = (true|false) }
	DNSDisableCompression bool

	// DNSDomain is the DNS domain for the records. Should end with a dot.
	// Defaults to "consul."
	//
	// hcl: domain = string
	// flag: -domain string
	DNSDomain string

	// DNSEnableTruncate is used to enable setting the truncate
	// flag for UDP DNS queries.  This allows unmodified
	// clients to re-query the consul server using TCP
	// when the total number of records exceeds the number
	// returned by default for UDP.
	//
	// hcl: dns_config { enable_truncate = (true|false) }
	DNSEnableTruncate bool

	// DNSMaxStale is used to bound how stale of a result is
	// accepted for a DNS lookup. This can be used with
	// AllowStale to limit how old of a value is served up.
	// If the stale result exceeds this, another non-stale
	// stale read is performed.
	//
	// hcl: dns_config { max_stale = "duration" }
	DNSMaxStale time.Duration

	// DNSNodeTTL provides the TTL value for a node query.
	//
	// hcl: dns_config { node_ttl = "duration" }
	DNSNodeTTL time.Duration

	// DNSOnlyPassing is used to determine whether to filter nodes
	// whose health checks are in any non-passing state. By
	// default, only nodes in a critical state are excluded.
	//
	// hcl: dns_config { only_passing = "duration" }
	DNSOnlyPassing bool

	// DNSRecursorTimeout specifies the timeout in seconds
	// for Consul's internal dns client used for recursion.
	// This value is used for the connection, read and write timeout.
	//
	// hcl: dns_config { recursor_timeout = "duration" }
	DNSRecursorTimeout time.Duration

	// DNSServiceTTL provides the TTL value for a service
	// query for given service. The "*" wildcard can be used
	// to set a default for all services.
	//
	// hcl: dns_config { service_ttl = map[string]"duration" }
	DNSServiceTTL map[string]time.Duration

	// DNSUDPAnswerLimit is used to limit the maximum number of DNS Resource
	// Records returned in the ANSWER section of a DNS response. This is
	// not normally useful and will be limited based on the querying
	// protocol, however systems that implemented §6 Rule 9 in RFC3484
	// may want to set this to `1` in order to subvert §6 Rule 9 and
	// re-obtain the effect of randomized resource records (i.e. each
	// answer contains only one IP, but the IP changes every request).
	// RFC3484 sorts answers in a deterministic order, which defeats the
	// purpose of randomized DNS responses.  This RFC has been obsoleted
	// by RFC6724 and restores the desired behavior of randomized
	// responses, however a large number of Linux hosts using glibc(3)
	// implemented §6 Rule 9 and may need this option (e.g. CentOS 5-6,
	// Debian Squeeze, etc).
	//
	// hcl: dns_config { udp_answer_limit = int }
	DNSUDPAnswerLimit int

	// DNSRecursors can be set to allow the DNS servers to recursively
	// resolve non-consul domains.
	//
	// hcl: recursors = []string
	// flag: -recursor string [-recursor string]
	DNSRecursors []string

	// HTTPBlockEndpoints is a list of endpoint prefixes to block in the
	// HTTP API. Any requests to these will get a 403 response.
	//
	// hcl: http_config { block_endpoints = []string }
	HTTPBlockEndpoints []string

	// HTTPResponseHeaders are used to add HTTP header response fields to the HTTP API responses.
	//
	// hcl: http_config { response_headers = map[string]string }
	HTTPResponseHeaders map[string]string

	// TelemetryCirconus*: see https://github.com/circonus-labs/circonus-gometrics
	// for more details on the various configuration options.
	// Valid configuration combinations:
	//    - CirconusAPIToken
	//      metric management enabled (search for existing check or create a new one)
	//    - CirconusSubmissionUrl
	//      metric management disabled (use check with specified submission_url,
	//      broker must be using a public SSL certificate)
	//    - CirconusAPIToken + CirconusCheckSubmissionURL
	//      metric management enabled (use check with specified submission_url)
	//    - CirconusAPIToken + CirconusCheckID
	//      metric management enabled (use check with specified id)

	// TelemetryCirconusAPIApp is an app name associated with API token.
	// Default: "consul"
	//
	// hcl: telemetry { circonus_api_app = string }
	TelemetryCirconusAPIApp string

	// TelemetryCirconusAPIToken is a valid API Token used to create/manage check. If provided,
	// metric management is enabled.
	// Default: none
	//
	// hcl: telemetry { circonous_api_token = string }
	TelemetryCirconusAPIToken string

	// TelemetryCirconusAPIURL is the base URL to use for contacting the Circonus API.
	// Default: "https://api.circonus.com/v2"
	//
	// hcl: telemetry { circonus_api_url = string }
	TelemetryCirconusAPIURL string

	// TelemetryCirconusBrokerID is an explicit broker to use when creating a new check. The numeric portion
	// of broker._cid. If metric management is enabled and neither a Submission URL nor Check ID
	// is provided, an attempt will be made to search for an existing check using Instance ID and
	// Search Tag. If one is not found, a new HTTPTRAP check will be created.
	// Default: use Select Tag if provided, otherwise, a random Enterprise Broker associated
	// with the specified API token or the default Circonus Broker.
	// Default: none
	//
	// hcl: telemetry { circonus_broker_id = string }
	TelemetryCirconusBrokerID string

	// TelemetryCirconusBrokerSelectTag is a special tag which will be used to select a broker when
	// a Broker ID is not provided. The best use of this is to as a hint for which broker
	// should be used based on *where* this particular instance is running.
	// (e.g. a specific geo location or datacenter, dc:sfo)
	// Default: none
	//
	// hcl: telemetry { circonus_broker_select_tag = string }
	TelemetryCirconusBrokerSelectTag string

	// TelemetryCirconusCheckDisplayName is the name for the check which will be displayed in the Circonus UI.
	// Default: value of CirconusCheckInstanceID
	//
	// hcl: telemetry { circonus_check_display_name = string }
	TelemetryCirconusCheckDisplayName string

	// TelemetryCirconusCheckForceMetricActivation will force enabling metrics, as they are encountered,
	// if the metric already exists and is NOT active. If check management is enabled, the default
	// behavior is to add new metrics as they are encoutered. If the metric already exists in the
	// check, it will *NOT* be activated. This setting overrides that behavior.
	// Default: "false"
	//
	// hcl: telemetry { circonus_check_metrics_activation = (true|false)
	TelemetryCirconusCheckForceMetricActivation string

	// TelemetryCirconusCheckID is the check id (not check bundle id) from a previously created
	// HTTPTRAP check. The numeric portion of the check._cid field.
	// Default: none
	//
	// hcl: telemetry { circonus_check_id = string }
	TelemetryCirconusCheckID string

	// TelemetryCirconusCheckInstanceID serves to uniquely identify the metrics coming from this "instance".
	// It can be used to maintain metric continuity with transient or ephemeral instances as
	// they move around within an infrastructure.
	// Default: hostname:app
	//
	// hcl: telemetry { circonus_check_instance_id = string }
	TelemetryCirconusCheckInstanceID string

	// TelemetryCirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:consul)
	//
	// hcl: telemetry { circonus_check_search_tag = string }
	TelemetryCirconusCheckSearchTag string

	// TelemetryCirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:consul)
	//
	// hcl: telemetry { circonus_check_tags = string }
	TelemetryCirconusCheckTags string

	// TelemetryCirconusSubmissionInterval is the interval at which metrics are submitted to Circonus.
	// Default: 10s
	//
	// hcl: telemetry { circonus_submission_interval = "duration" }
	TelemetryCirconusSubmissionInterval string

	// TelemetryCirconusCheckSubmissionURL is the check.config.submission_url field from a
	// previously created HTTPTRAP check.
	// Default: none
	//
	// hcl: telemetry { circonus_submission_url = string }
	TelemetryCirconusSubmissionURL string

	// DisableHostname will disable hostname prefixing for all metrics.
	//
	// hcl: telemetry { disable_hostname = (true|false)
	TelemetryDisableHostname bool

	// TelemetryDogStatsdAddr is the address of a dogstatsd instance. If provided,
	// metrics will be sent to that instance
	//
	// hcl: telemetry { dogstatsd_addr = string }
	TelemetryDogstatsdAddr string

	// TelemetryDogStatsdTags are the global tags that should be sent with each packet to dogstatsd
	// It is a list of strings, where each string looks like "my_tag_name:my_tag_value"
	//
	// hcl: telemetry { dogstatsd_tags = []string }
	TelemetryDogstatsdTags []string

	// TelemetryFilterDefault is the default for whether to allow a metric that's not
	// covered by the filter.
	//
	// hcl: telemetry { filter_default = (true|false) }
	TelemetryFilterDefault bool

	// TelemetryAllowedPrefixes is a list of filter rules to apply for allowing metrics
	// by prefix. Use the 'prefix_filter' option and prefix rules with '+' to be
	// included.
	//
	// hcl: telemetry { prefix_filter = []string{"+<expr>", "+<expr>", ...} }
	TelemetryAllowedPrefixes []string

	// TelemetryBlockedPrefixes is a list of filter rules to apply for blocking metrics
	// by prefix. Use the 'prefix_filter' option and prefix rules with '-' to be
	// excluded.
	//
	// hcl: telemetry { prefix_filter = []string{"-<expr>", "-<expr>", ...} }
	TelemetryBlockedPrefixes []string

	// TelemetryMetricsPrefix is the prefix used to write stats values to.
	// Default: "consul."
	//
	// hcl: telemetry { metrics_prefix = string }
	TelemetryMetricsPrefix string

	// TelemetryStatsdAddr is the address of a statsd instance. If provided,
	// metrics will be sent to that instance.
	//
	// hcl: telemetry { statsd_addr = string }
	TelemetryStatsdAddr string

	// TelemetryStatsiteAddr is the address of a statsite instance. If provided,
	// metrics will be streamed to that instance.
	//
	// hcl: telemetry { statsite_addr = string }
	TelemetryStatsiteAddr string

	// Datacenter is the datacenter this node is in. Defaults to "dc1".
	//
	// Datacenter is exposed via /v1/agent/self from here and
	// used in lots of places like CLI commands. Treat this as an interface
	// that must be stable.
	// todo(fs): do we need this warning? RuntimeConfig was meant to be refactorable.
	//
	// hcl: datacenter = string
	// flag: -datacenter string
	Datacenter string

	// Node name is the name we use to advertise. Defaults to hostname.
	//
	// NodeName is exposed via /v1/agent/self from here and
	// used in lots of places like CLI commands. Treat this as an interface
	// that must be stable.
	// todo(fs): do we need this warning? RuntimeConfig was meant to be refactorable.
	//
	// hcl: node_name = string
	// flag: -node string
	NodeName string

	AdvertiseAddrLAN *net.IPAddr
	AdvertiseAddrWAN *net.IPAddr

	// BindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip, Server RPC)
	//
	// The value can be either an ip address or a go-sockaddr
	// template which resolves to a single ip address.
	//
	// hcl: bind_addr = string
	// flag: -bind string
	BindAddr *net.IPAddr

	// Bootstrap is used to bring up the first Consul server, and
	// permits that node to elect itself leader
	//
	// hcl: bootstrap = (true|false)
	// flag: -bootstrap
	Bootstrap bool

	// BootstrapExpect tries to automatically bootstrap the Consul cluster,
	// by withholding peers until enough servers join.
	//
	// hcl: bootstrap_expect = int
	// flag: -bootstrap-expect=int
	BootstrapExpect int

	// CAFile is a path to a certificate authority file. This is used with
	// VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	//
	// hcl: ca_file = string
	CAFile string

	// CAPath is a path to a directory of certificate authority files. This is
	// used with VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	//
	// hcl: ca_path = string
	CAPath string

	// CertFile is used to provide a TLS certificate that is used for serving
	// TLS connections. Must be provided to serve TLS connections.
	//
	// hcl: cert_file = string
	CertFile string

	CheckUpdateInterval time.Duration
	Checks              []*structs.CheckDefinition
	ClientAddrs         []*net.IPAddr
	DNSAddrs            []net.Addr
	DNSPort             int

	// DataDir is the path to the directory where the local state is stored.
	//
	// hcl: data_dir = string
	// flag: -data-dir string
	DataDir string

	// DevMode enables a fast-path mode of operation to bring up an in-memory
	// server with minimal configuration. Useful for developing Consul.
	//
	// flag: -dev
	DevMode bool

	DisableAnonymousSignature bool
	DisableCoordinates        bool

	// DisableHostNodeID will prevent Consul from using information from the
	// host to generate a node ID, and will cause Consul to generate a
	// random ID instead.
	//
	// hcl: disable_host_node_id = (true|false)
	DisableHostNodeID bool

	// DisableKeyringFile disables writing the keyring to a file.
	//
	// hcl: disable_keyring_file = (true|false)
	// flag: -disable-keyring-file
	DisableKeyringFile bool

	DisableRemoteExec  bool
	DisableUpdateCheck bool
	DiscardCheckOutput bool

	// EnableACLReplication is used to turn on ACL replication when using
	// /v1/agent/token/acl_replication_token to introduce the token, instead
	// of setting acl_replication_token in the config. Setting the token via
	// config will also set this to true for backward compatibility.
	//
	// hcl: enable_acl_replication = (true|false)
	// todo(fs): rename to ACLEnableReplication
	EnableACLReplication bool

	// EnableDebug is used to enable various debugging features.
	//
	// hcl: enable_debug = (true|false)
	EnableDebug bool

	EnableScriptChecks bool
	EnableSyslog       bool
	EnableUI           bool

	// EncryptKey contains the encryption key to use for the Serf communication.
	//
	// hcl: encrypt = string
	// flag: -encrypt string
	EncryptKey string

	// EncryptVerifyIncoming enforces incoming gossip encryption and can be
	// used to upshift to encrypted gossip on a running cluster.
	//
	// hcl: encrypt_verify_incoming = (true|false)
	EncryptVerifyIncoming bool

	// EncryptVerifyOutgoing enforces outgoing gossip encryption and can be
	// used to upshift to encrypted gossip on a running cluster.
	//
	// hcl: encrypt_verify_outgoing = (true|false)
	EncryptVerifyOutgoing bool

	HTTPAddrs  []net.Addr
	HTTPPort   int
	HTTPSAddrs []net.Addr
	HTTPSPort  int

	// KeyFile is used to provide a TLS key that is used for serving TLS
	// connections. Must be provided to serve TLS connections.
	//
	// hcl: key_file = string
	KeyFile string

	LeaveDrainTime time.Duration

	// LeaveOnTerm controls if Serf does a graceful leave when receiving
	// the TERM signal. Defaults true on clients, false on servers. (reloadable)
	//
	// hcl: leave_on_terminate = (true|false)
	LeaveOnTerm bool

	// LogLevel is the level of the logs to write. Defaults to "INFO".
	//
	// hcl: log_level = string
	LogLevel string

	// Node ID is a unique ID for this node across space and time. Defaults
	// to a randomly-generated ID that persists in the data-dir.
	//
	// todo(fs): don't we have a requirement for this to be a UUID in a specific format?
	//
	// hcl: node_id = string
	// flag: -node-id string
	NodeID types.NodeID

	// NodeMeta contains metadata key/value pairs. These are excluded from JSON output
	// because they can be reloaded and might be stale when shown from the
	// config instead of the local state.
	// todo(fs): should the sanitizer omit them from output as well since they could be stale?
	//
	// hcl: node_meta = map[string]string
	// flag: -node-meta "key:value" -node-meta "key:value" ...
	NodeMeta map[string]string

	// NonVotingServer is whether this server will act as a non-voting member
	// of the cluster to help provide read scalability. (Enterprise-only)
	//
	// hcl: non_voting_server = (true|false)
	// flag: -non-voting-server
	NonVotingServer bool

	PidFile          string
	RPCAdvertiseAddr *net.TCPAddr
	RPCBindAddr      *net.TCPAddr
	RPCHoldTimeout   time.Duration

	// RPCRateLimit and RPCMaxBurst control how frequently RPC calls are allowed
	// to happen. In any large enough time interval, rate limiter limits the
	// rate to RPCRate tokens per second, with a maximum burst size of
	// RPCMaxBurst events. As a special case, if RPCRate == Inf (the infinite
	// rate), RPCMaxBurst is ignored.
	//
	// See https://en.wikipedia.org/wiki/Token_bucket for more about token
	// buckets.
	//
	// hcl: limit { rpc_rate = (float64|MaxFloat64) rpc_max_burst = int }
	RPCRateLimit rate.Limit
	RPCMaxBurst  int

	// RPCProtocol is the Consul protocol version to use.
	//
	// hcl: protocol = int
	RPCProtocol int

	// RaftProtocol sets the Raft protocol version to use on this server.
	// Defaults to 3.
	//
	// hcl: raft_protocol = int
	RaftProtocol int

	ReconnectTimeoutLAN     time.Duration
	ReconnectTimeoutWAN     time.Duration
	RejoinAfterLeave        bool
	RetryJoinIntervalLAN    time.Duration
	RetryJoinIntervalWAN    time.Duration
	RetryJoinLAN            []string
	RetryJoinMaxAttemptsLAN int
	RetryJoinMaxAttemptsWAN int
	RetryJoinWAN            []string

	// SegmentName is the network segment for this client to join.
	// (Enterprise-only)
	//
	// hcl: segment = string
	SegmentName string

	// Segments is the list of network segments for this server to
	// initialize.
	//
	// hcl: segment = [
	//   {
	//     # name is the name of the segment
	//     name = string
	//
	//     # bind is the bind ip address for this segment.
	//     bind = string
	//
	//     # port is the bind port for this segment.
	//     port = int
	//
	//     # advertise is the advertise ip address for this segment.
	//     # Defaults to the bind address if not set.
	//     advertise = string
	//
	//     # rpc_listener controls whether or not to bind a separate
	//     # RPC listener to the bind address.
	//     rpc_listener = (true|false)
	//   },
	//   ...
	// ]
	Segments []structs.NetworkSegment

	SerfAdvertiseAddrLAN *net.TCPAddr
	SerfAdvertiseAddrWAN *net.TCPAddr
	SerfBindAddrLAN      *net.TCPAddr
	SerfBindAddrWAN      *net.TCPAddr
	SerfPortLAN          int
	SerfPortWAN          int

	// ServerMode controls if this agent acts like a Consul server,
	// or merely as a client. Servers have more state, take part
	// in leader election, etc.
	//
	// hcl: server = (true|false)
	// flag: -server
	ServerMode bool

	// ServerName is used with the TLS certificates to ensure the name we
	// provide matches the certificate.
	//
	// hcl: server_name = string
	ServerName string

	ServerPort    int
	Services      []*structs.ServiceDefinition
	SessionTTLMin time.Duration

	// SkipLeaveOnInt controls if Serf skips a graceful leave when
	// receiving the INT signal. Defaults false on clients, true on
	// servers. (reloadable)
	//
	// hcl: skip_leave_on_interrupt = (true|false)
	SkipLeaveOnInt bool

	StartJoinAddrsLAN []string
	StartJoinAddrsWAN []string
	SyslogFacility    string

	// TLSCipherSuites is used to specify the list of supported ciphersuites.
	//
	// The values should be a list of the following values:
	//
	//   TLS_RSA_WITH_RC4_128_SHA
	//   TLS_RSA_WITH_3DES_EDE_CBC_SHA
	//   TLS_RSA_WITH_AES_128_CBC_SHA
	//   TLS_RSA_WITH_AES_256_CBC_SHA
	//   TLS_RSA_WITH_AES_128_GCM_SHA256
	//   TLS_RSA_WITH_AES_256_GCM_SHA384
	//   TLS_ECDHE_ECDSA_WITH_RC4_128_SHA
	//   TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
	//   TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_RC4_128_SHA
	//   TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	//   TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
	//   TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
	//   TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
	//
	// todo(fs): IMHO, we should also support the raw 0xNNNN values from
	// todo(fs): https://golang.org/pkg/crypto/tls/#pkg-constants
	// todo(fs): since they are standardized by IANA.
	//
	// hcl: tls_cipher_suites = []string
	TLSCipherSuites []uint16

	// TLSMinVersion is used to set the minimum TLS version used for TLS
	// connections. Should be either "tls10", "tls11", or "tls12".
	//
	// hcl: tls_min_version = string
	TLSMinVersion string

	// TLSPreferServerCipherSuites specifies whether to prefer the server's
	// cipher suite over the client cipher suites.
	//
	// hcl: tls_prefer_server_cipher_suites = (true|false)
	TLSPreferServerCipherSuites bool

	// TaggedAddresses are used to publish a set of addresses for
	// for a node, which can be used by the remote agent. We currently
	// populate only the "wan" tag based on the SerfWan advertise address,
	// but this structure is here for possible future features with other
	// user-defined tags. The "wan" tag will be used by remote agents if
	// they are configured with TranslateWANAddrs set to true.
	//
	// hcl: tagged_addresses = map[string]string
	TaggedAddresses map[string]string

	// TranslateWANAddrs controls whether or not Consul should prefer
	// the "wan" tagged address when doing lookups in remote datacenters.
	// See TaggedAddresses below for more details.
	//
	// hcl: translate_wan_addrs = (true|false)
	TranslateWANAddrs bool

	UIDir           string
	UnixSocketGroup string
	UnixSocketMode  string
	UnixSocketUser  string

	// VerifyIncoming is used to verify the authenticity of incoming
	// connections. This means that TCP requests are forbidden, only allowing
	// for TLS. TLS connections must match a provided certificate authority.
	// This can be used to force client auth.
	//
	// hcl: verify_incoming = (true|false)
	VerifyIncoming bool

	// VerifyIncomingHTTPS is used to verify the authenticity of incoming HTTPS
	// connections. This means that TCP requests are forbidden, only allowing
	// for TLS. TLS connections must match a provided certificate authority.
	// This can be used to force client auth.
	//
	// hcl: verify_incoming_https = (true|false)
	VerifyIncomingHTTPS bool

	// VerifyIncomingRPC is used to verify the authenticity of incoming RPC
	// connections. This means that TCP requests are forbidden, only allowing
	// for TLS. TLS connections must match a provided certificate authority.
	// This can be used to force client auth.
	//
	// hcl: verify_incoming_rpc = (true|false)
	VerifyIncomingRPC bool

	// VerifyOutgoing is used to verify the authenticity of outgoing
	// connections. This means that TLS requests are used. TLS connections must
	// match a provided certificate authority. This is used to verify
	// authenticity of server nodes.
	//
	// hcl: verify_outgoing = (true|false)
	VerifyOutgoing bool

	// VerifyServerHostname is used to enable hostname verification of servers.
	// This ensures that the certificate presented is valid for
	// server.<datacenter>.<domain>. This prevents a compromised client from
	// being restarted as a server, and then intercepting request traffic as
	// well as being added as a raft peer. This should be enabled by default
	// with VerifyOutgoing, but for legacy reasons we cannot break existing
	// clients.
	//
	// hcl: verify_server_hostname = (true|false)
	VerifyServerHostname bool

	Watches []map[string]interface{}
}

// IncomingHTTPSConfig returns the TLS configuration for HTTPS
// connections to consul.
func (c *RuntimeConfig) IncomingHTTPSConfig() (*tls.Config, error) {
	tc := &tlsutil.Config{
		VerifyIncoming:           c.VerifyIncoming || c.VerifyIncomingHTTPS,
		VerifyOutgoing:           c.VerifyOutgoing,
		CAFile:                   c.CAFile,
		CAPath:                   c.CAPath,
		CertFile:                 c.CertFile,
		KeyFile:                  c.KeyFile,
		NodeName:                 c.NodeName,
		ServerName:               c.ServerName,
		TLSMinVersion:            c.TLSMinVersion,
		CipherSuites:             c.TLSCipherSuites,
		PreferServerCipherSuites: c.TLSPreferServerCipherSuites,
	}
	return tc.IncomingTLSConfig()
}

// Sanitized returns a JSON/HCL compatible representation of the runtime
// configuration where all fields with potential secrets had their
// values replaced by 'hidden'. In addition, network addresses and
// time.Duration values are formatted to improve readability.
func (c *RuntimeConfig) Sanitized() map[string]interface{} {
	return sanitize("rt", reflect.ValueOf(c)).Interface().(map[string]interface{})
}

// isSecret determines whether a field name represents a field which
// may contain a secret.
func isSecret(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "key") || strings.Contains(name, "token") || strings.Contains(name, "secret")
}

// cleanRetryJoin sanitizes the go-discover config strings key=val key=val...
// by scrubbing the individual key=val combinations.
func cleanRetryJoin(a string) string {
	var fields []string
	for _, f := range strings.Fields(a) {
		if isSecret(f) {
			kv := strings.SplitN(f, "=", 2)
			fields = append(fields, kv[0]+"=hidden")
		} else {
			fields = append(fields, f)
		}
	}
	return strings.Join(fields, " ")
}

func sanitize(name string, v reflect.Value) reflect.Value {
	typ := v.Type()
	switch {

	// check before isStruct and isPtr
	case isNetAddr(typ):
		if v.IsNil() {
			return reflect.ValueOf("")
		}
		switch x := v.Interface().(type) {
		case *net.TCPAddr:
			return reflect.ValueOf("tcp://" + x.String())
		case *net.UDPAddr:
			return reflect.ValueOf("udp://" + x.String())
		case *net.UnixAddr:
			return reflect.ValueOf("unix://" + x.String())
		case *net.IPAddr:
			return reflect.ValueOf(x.IP.String())
		default:
			return v
		}

	// check before isNumber
	case isDuration(typ):
		x := v.Interface().(time.Duration)
		return reflect.ValueOf(x.String())

	case isString(typ):
		if strings.HasPrefix(name, "RetryJoinLAN[") || strings.HasPrefix(name, "RetryJoinWAN[") {
			x := v.Interface().(string)
			return reflect.ValueOf(cleanRetryJoin(x))
		}
		if isSecret(name) {
			return reflect.ValueOf("hidden")
		}
		return v

	case isNumber(typ) || isBool(typ):
		return v

	case isPtr(typ):
		if v.IsNil() {
			return v
		}
		return sanitize(name, v.Elem())

	case isStruct(typ):
		m := map[string]interface{}{}
		for i := 0; i < typ.NumField(); i++ {
			key := typ.Field(i).Name
			m[key] = sanitize(key, v.Field(i)).Interface()
		}
		return reflect.ValueOf(m)

	case isArray(typ) || isSlice(typ):
		ma := make([]interface{}, 0)
		for i := 0; i < v.Len(); i++ {
			ma = append(ma, sanitize(fmt.Sprintf("%s[%d]", name, i), v.Index(i)).Interface())
		}
		return reflect.ValueOf(ma)

	case isMap(typ):
		m := map[string]interface{}{}
		for _, k := range v.MapKeys() {
			key := k.String()
			m[key] = sanitize(key, v.MapIndex(k)).Interface()
		}
		return reflect.ValueOf(m)

	default:
		return v
	}
}

func isDuration(t reflect.Type) bool { return t == reflect.TypeOf(time.Second) }
func isMap(t reflect.Type) bool      { return t.Kind() == reflect.Map }
func isNetAddr(t reflect.Type) bool  { return t.Implements(reflect.TypeOf((*net.Addr)(nil)).Elem()) }
func isPtr(t reflect.Type) bool      { return t.Kind() == reflect.Ptr }
func isArray(t reflect.Type) bool    { return t.Kind() == reflect.Array }
func isSlice(t reflect.Type) bool    { return t.Kind() == reflect.Slice }
func isString(t reflect.Type) bool   { return t.Kind() == reflect.String }
func isStruct(t reflect.Type) bool   { return t.Kind() == reflect.Struct }
func isBool(t reflect.Type) bool     { return t.Kind() == reflect.Bool }
func isNumber(t reflect.Type) bool   { return isInt(t) || isUint(t) || isFloat(t) || isComplex(t) }
func isInt(t reflect.Type) bool {
	return t.Kind() == reflect.Int ||
		t.Kind() == reflect.Int8 ||
		t.Kind() == reflect.Int16 ||
		t.Kind() == reflect.Int32 ||
		t.Kind() == reflect.Int64
}
func isUint(t reflect.Type) bool {
	return t.Kind() == reflect.Uint ||
		t.Kind() == reflect.Uint8 ||
		t.Kind() == reflect.Uint16 ||
		t.Kind() == reflect.Uint32 ||
		t.Kind() == reflect.Uint64
}
func isFloat(t reflect.Type) bool { return t.Kind() == reflect.Float32 || t.Kind() == reflect.Float64 }
func isComplex(t reflect.Type) bool {
	return t.Kind() == reflect.Complex64 || t.Kind() == reflect.Complex128
}
