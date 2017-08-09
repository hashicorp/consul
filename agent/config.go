package agent

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/watch"
	"github.com/hashicorp/go-sockaddr/template"
	"github.com/mitchellh/mapstructure"
)

// Ports is used to simplify the configuration by
// providing default ports, and allowing the addresses
// to only be specified once
type PortConfig struct {
	DNS     int // DNS Query interface
	HTTP    int // HTTP API
	HTTPS   int // HTTPS API
	SerfLan int `mapstructure:"serf_lan"` // LAN gossip (Client + Server)
	SerfWan int `mapstructure:"serf_wan"` // WAN gossip (Server only)
	Server  int // Server internal RPC

	// RPC is deprecated and is no longer used. It will be removed in a future
	// version.
	RPC int // CLI RPC
}

// AddressConfig is used to provide address overrides
// for specific services. By default, either ClientAddress
// or ServerAddress is used.
type AddressConfig struct {
	DNS   string // DNS Query interface
	HTTP  string // HTTP API
	HTTPS string // HTTPS API

	// RPC is deprecated and is no longer used. It will be removed in a future
	// version.
	RPC string // CLI RPC
}

type AdvertiseAddrsConfig struct {
	SerfLan    *net.TCPAddr `mapstructure:"-"`
	SerfLanRaw string       `mapstructure:"serf_lan"`
	SerfWan    *net.TCPAddr `mapstructure:"-"`
	SerfWanRaw string       `mapstructure:"serf_wan"`
	RPC        *net.TCPAddr `mapstructure:"-"`
	RPCRaw     string       `mapstructure:"rpc"`
}

// DNSConfig is used to fine tune the DNS sub-system.
// It can be used to control cache values, and stale
// reads
type DNSConfig struct {
	// NodeTTL provides the TTL value for a node query
	NodeTTL    time.Duration `mapstructure:"-"`
	NodeTTLRaw string        `mapstructure:"node_ttl" json:"-"`

	// ServiceTTL provides the TTL value for a service
	// query for given service. The "*" wildcard can be used
	// to set a default for all services.
	ServiceTTL    map[string]time.Duration `mapstructure:"-"`
	ServiceTTLRaw map[string]string        `mapstructure:"service_ttl" json:"-"`

	// AllowStale is used to enable lookups with stale
	// data. This gives horizontal read scalability since
	// any Consul server can service the query instead of
	// only the leader.
	AllowStale *bool `mapstructure:"allow_stale"`

	// EnableTruncate is used to enable setting the truncate
	// flag for UDP DNS queries.  This allows unmodified
	// clients to re-query the consul server using TCP
	// when the total number of records exceeds the number
	// returned by default for UDP.
	EnableTruncate bool `mapstructure:"enable_truncate"`

	// UDPAnswerLimit is used to limit the maximum number of DNS Resource
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
	UDPAnswerLimit int `mapstructure:"udp_answer_limit"`

	// MaxStale is used to bound how stale of a result is
	// accepted for a DNS lookup. This can be used with
	// AllowStale to limit how old of a value is served up.
	// If the stale result exceeds this, another non-stale
	// stale read is performed.
	MaxStale    time.Duration `mapstructure:"-"`
	MaxStaleRaw string        `mapstructure:"max_stale" json:"-"`

	// OnlyPassing is used to determine whether to filter nodes
	// whose health checks are in any non-passing state. By
	// default, only nodes in a critical state are excluded.
	OnlyPassing bool `mapstructure:"only_passing"`

	// DisableCompression is used to control whether DNS responses are
	// compressed. In Consul 0.7 this was turned on by default and this
	// config was added as an opt-out.
	DisableCompression bool `mapstructure:"disable_compression"`

	// RecursorTimeout specifies the timeout in seconds
	// for Consul's internal dns client used for recursion.
	// This value is used for the connection, read and write timeout.
	// Default: 2s
	RecursorTimeout    time.Duration `mapstructure:"-"`
	RecursorTimeoutRaw string        `mapstructure:"recursor_timeout" json:"-"`
}

// HTTPConfig is used to fine tune the Http sub-system.
type HTTPConfig struct {
	// BlockEndpoints is a list of endpoint prefixes to block in the
	// HTTP API. Any requests to these will get a 403 response.
	BlockEndpoints []string `mapstructure:"block_endpoints"`

	// ResponseHeaders are used to add HTTP header response fields to the HTTP API responses.
	ResponseHeaders map[string]string `mapstructure:"response_headers"`
}

// RetryJoinEC2 is used to configure discovery of instances via Amazon's EC2 api
type RetryJoinEC2 struct {
	// The AWS region to look for instances in
	Region string `mapstructure:"region"`

	// The tag key and value to use when filtering instances
	TagKey   string `mapstructure:"tag_key"`
	TagValue string `mapstructure:"tag_value"`

	// The AWS credentials to use for making requests to EC2
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	SecretAccessKey string `mapstructure:"secret_access_key" json:"-"`
}

// RetryJoinGCE is used to configure discovery of instances via Google Compute
// Engine's API.
type RetryJoinGCE struct {
	// The name of the project the instances reside in.
	ProjectName string `mapstructure:"project_name"`

	// A regular expression (RE2) pattern for the zones you want to discover the instances in.
	// Example: us-west1-.*, or us-(?west|east).*.
	ZonePattern string `mapstructure:"zone_pattern"`

	// The tag value to search for when filtering instances.
	TagValue string `mapstructure:"tag_value"`

	// A path to a JSON file with the service account credentials necessary to
	// connect to GCE. If this is not defined, the following chain is respected:
	// 1. A JSON file whose path is specified by the
	//		GOOGLE_APPLICATION_CREDENTIALS environment variable.
	// 2. A JSON file in a location known to the gcloud command-line tool.
	//    On Windows, this is %APPDATA%/gcloud/application_default_credentials.json.
	//  	On other systems, $HOME/.config/gcloud/application_default_credentials.json.
	// 3. On Google Compute Engine, it fetches credentials from the metadata
	//    server.  (In this final case any provided scopes are ignored.)
	CredentialsFile string `mapstructure:"credentials_file"`
}

// RetryJoinAzure is used to configure discovery of instances via AzureRM API
type RetryJoinAzure struct {
	// The tag name and value to use when filtering instances
	TagName  string `mapstructure:"tag_name"`
	TagValue string `mapstructure:"tag_value"`

	// The Azure credentials to use for making requests to AzureRM
	SubscriptionID  string `mapstructure:"subscription_id" json:"-"`
	TenantID        string `mapstructure:"tenant_id" json:"-"`
	ClientID        string `mapstructure:"client_id" json:"-"`
	SecretAccessKey string `mapstructure:"secret_access_key" json:"-"`
}

// Performance is used to tune the performance of Consul's subsystems.
type Performance struct {
	// RaftMultiplier is an integer multiplier used to scale Raft timing
	// parameters: HeartbeatTimeout, ElectionTimeout, and LeaderLeaseTimeout.
	RaftMultiplier uint `mapstructure:"raft_multiplier"`
}

// Telemetry is the telemetry configuration for the server
type Telemetry struct {
	// StatsiteAddr is the address of a statsite instance. If provided,
	// metrics will be streamed to that instance.
	StatsiteAddr string `mapstructure:"statsite_address"`

	// StatsdAddr is the address of a statsd instance. If provided,
	// metrics will be sent to that instance.
	StatsdAddr string `mapstructure:"statsd_address"`

	// StatsitePrefix is the prefix used to write stats values to. By
	// default this is set to 'consul'.
	StatsitePrefix string `mapstructure:"statsite_prefix"`

	// DisableHostname will disable hostname prefixing for all metrics
	DisableHostname bool `mapstructure:"disable_hostname"`

	// PrefixFilter is a list of filter rules to apply for allowing/blocking metrics
	// by prefix.
	PrefixFilter    []string `mapstructure:"prefix_filter"`
	AllowedPrefixes []string `mapstructure:"-" json:"-"`
	BlockedPrefixes []string `mapstructure:"-" json:"-"`

	// FilterDefault is the default for whether to allow a metric that's not
	// covered by the filter.
	FilterDefault *bool `mapstructure:"filter_default"`

	// DogStatsdAddr is the address of a dogstatsd instance. If provided,
	// metrics will be sent to that instance
	DogStatsdAddr string `mapstructure:"dogstatsd_addr"`

	// DogStatsdTags are the global tags that should be sent with each packet to dogstatsd
	// It is a list of strings, where each string looks like "my_tag_name:my_tag_value"
	DogStatsdTags []string `mapstructure:"dogstatsd_tags"`

	// Circonus: see https://github.com/circonus-labs/circonus-gometrics
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

	// CirconusAPIToken is a valid API Token used to create/manage check. If provided,
	// metric management is enabled.
	// Default: none
	CirconusAPIToken string `mapstructure:"circonus_api_token" json:"-"`
	// CirconusAPIApp is an app name associated with API token.
	// Default: "consul"
	CirconusAPIApp string `mapstructure:"circonus_api_app"`
	// CirconusAPIURL is the base URL to use for contacting the Circonus API.
	// Default: "https://api.circonus.com/v2"
	CirconusAPIURL string `mapstructure:"circonus_api_url"`
	// CirconusSubmissionInterval is the interval at which metrics are submitted to Circonus.
	// Default: 10s
	CirconusSubmissionInterval string `mapstructure:"circonus_submission_interval"`
	// CirconusCheckSubmissionURL is the check.config.submission_url field from a
	// previously created HTTPTRAP check.
	// Default: none
	CirconusCheckSubmissionURL string `mapstructure:"circonus_submission_url"`
	// CirconusCheckID is the check id (not check bundle id) from a previously created
	// HTTPTRAP check. The numeric portion of the check._cid field.
	// Default: none
	CirconusCheckID string `mapstructure:"circonus_check_id"`
	// CirconusCheckForceMetricActivation will force enabling metrics, as they are encountered,
	// if the metric already exists and is NOT active. If check management is enabled, the default
	// behavior is to add new metrics as they are encoutered. If the metric already exists in the
	// check, it will *NOT* be activated. This setting overrides that behavior.
	// Default: "false"
	CirconusCheckForceMetricActivation string `mapstructure:"circonus_check_force_metric_activation"`
	// CirconusCheckInstanceID serves to uniquely identify the metrics coming from this "instance".
	// It can be used to maintain metric continuity with transient or ephemeral instances as
	// they move around within an infrastructure.
	// Default: hostname:app
	CirconusCheckInstanceID string `mapstructure:"circonus_check_instance_id"`
	// CirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:consul)
	CirconusCheckSearchTag string `mapstructure:"circonus_check_search_tag"`
	// CirconusCheckTags is a comma separated list of tags to apply to the check. Note that
	// the value of CirconusCheckSearchTag will always be added to the check.
	// Default: none
	CirconusCheckTags string `mapstructure:"circonus_check_tags"`
	// CirconusCheckDisplayName is the name for the check which will be displayed in the Circonus UI.
	// Default: value of CirconusCheckInstanceID
	CirconusCheckDisplayName string `mapstructure:"circonus_check_display_name"`
	// CirconusBrokerID is an explicit broker to use when creating a new check. The numeric portion
	// of broker._cid. If metric management is enabled and neither a Submission URL nor Check ID
	// is provided, an attempt will be made to search for an existing check using Instance ID and
	// Search Tag. If one is not found, a new HTTPTRAP check will be created.
	// Default: use Select Tag if provided, otherwise, a random Enterprise Broker associated
	// with the specified API token or the default Circonus Broker.
	// Default: none
	CirconusBrokerID string `mapstructure:"circonus_broker_id"`
	// CirconusBrokerSelectTag is a special tag which will be used to select a broker when
	// a Broker ID is not provided. The best use of this is to as a hint for which broker
	// should be used based on *where* this particular instance is running.
	// (e.g. a specific geo location or datacenter, dc:sfo)
	// Default: none
	CirconusBrokerSelectTag string `mapstructure:"circonus_broker_select_tag"`
}

// Autopilot is used to configure helpful features for operating Consul servers.
type Autopilot struct {
	// CleanupDeadServers enables the automatic cleanup of dead servers when new ones
	// are added to the peer list. Defaults to true.
	CleanupDeadServers *bool `mapstructure:"cleanup_dead_servers"`

	// LastContactThreshold is the limit on the amount of time a server can go
	// without leader contact before being considered unhealthy.
	LastContactThreshold    *time.Duration `mapstructure:"-" json:"-"`
	LastContactThresholdRaw string         `mapstructure:"last_contact_threshold"`

	// MaxTrailingLogs is the amount of entries in the Raft Log that a server can
	// be behind before being considered unhealthy.
	MaxTrailingLogs *uint64 `mapstructure:"max_trailing_logs"`

	// ServerStabilizationTime is the minimum amount of time a server must be
	// in a stable, healthy state before it can be added to the cluster. Only
	// applicable with Raft protocol version 3 or higher.
	ServerStabilizationTime    *time.Duration `mapstructure:"-" json:"-"`
	ServerStabilizationTimeRaw string         `mapstructure:"server_stabilization_time"`

	// (Enterprise-only) RedundancyZoneTag is the Meta tag to use for separating servers
	// into zones for redundancy. If left blank, this feature will be disabled.
	RedundancyZoneTag string `mapstructure:"redundancy_zone_tag"`

	// (Enterprise-only) DisableUpgradeMigration will disable Autopilot's upgrade migration
	// strategy of waiting until enough newer-versioned servers have been added to the
	// cluster before promoting them to voters.
	DisableUpgradeMigration *bool `mapstructure:"disable_upgrade_migration"`

	// (Enterprise-only) UpgradeVersionTag is the node tag to use for version info when
	// performing upgrade migrations. If left blank, the Consul version will be used.
	UpgradeVersionTag string `mapstructure:"upgrade_version_tag"`
}

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
	// DevMode enables a fast-path mode of operation to bring up an in-memory
	// server with minimal configuration. Useful for developing Consul.
	DevMode bool `mapstructure:"-"`

	// Performance is used to tune the performance of Consul's subsystems.
	Performance Performance `mapstructure:"performance"`

	// Bootstrap is used to bring up the first Consul server, and
	// permits that node to elect itself leader
	Bootstrap bool `mapstructure:"bootstrap"`

	// BootstrapExpect tries to automatically bootstrap the Consul cluster,
	// by withholding peers until enough servers join.
	BootstrapExpect int `mapstructure:"bootstrap_expect"`

	// Server controls if this agent acts like a Consul server,
	// or merely as a client. Servers have more state, take part
	// in leader election, etc.
	Server bool `mapstructure:"server"`

	// (Enterprise-only) NonVotingServer is whether this server will act as a non-voting member
	// of the cluster to help provide read scalability.
	NonVotingServer bool `mapstructure:"non_voting_server"`

	// Datacenter is the datacenter this node is in. Defaults to dc1
	Datacenter string `mapstructure:"datacenter"`

	// DataDir is the directory to store our state in
	DataDir string `mapstructure:"data_dir"`

	// DNSRecursors can be set to allow the DNS servers to recursively
	// resolve non-consul domains. It is deprecated, and merges into the
	// recursors array.
	DNSRecursor string `mapstructure:"recursor"`

	// DNSRecursors can be set to allow the DNS servers to recursively
	// resolve non-consul domains
	DNSRecursors []string `mapstructure:"recursors"`

	// DNS configuration
	DNSConfig DNSConfig `mapstructure:"dns_config"`

	// Domain is the DNS domain for the records. Defaults to "consul."
	Domain string `mapstructure:"domain"`

	// HTTP configuration
	HTTPConfig HTTPConfig `mapstructure:"http_config"`

	// Encryption key to use for the Serf communication
	EncryptKey string `mapstructure:"encrypt" json:"-"`

	// Disables writing the keyring to a file.
	DisableKeyringFile bool `mapstructure:"disable_keyring_file"`

	// EncryptVerifyIncoming and EncryptVerifyOutgoing are used to enforce
	// incoming/outgoing gossip encryption and can be used to upshift to
	// encrypted gossip on a running cluster.
	EncryptVerifyIncoming *bool `mapstructure:"encrypt_verify_incoming"`
	EncryptVerifyOutgoing *bool `mapstructure:"encrypt_verify_outgoing"`

	// LogLevel is the level of the logs to putout
	LogLevel string `mapstructure:"log_level"`

	// Node ID is a unique ID for this node across space and time. Defaults
	// to a randomly-generated ID that persists in the data-dir.
	NodeID types.NodeID `mapstructure:"node_id"`

	// DisableHostNodeID will prevent Consul from using information from the
	// host to generate a node ID, and will cause Consul to generate a
	// random ID instead.
	DisableHostNodeID *bool `mapstructure:"disable_host_node_id"`

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string `mapstructure:"node_name"`

	// ClientAddr is used to control the address we bind to for
	// client services (DNS, HTTP, HTTPS, RPC)
	ClientAddr string `mapstructure:"client_addr"`

	// BindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip, Server RPC)
	BindAddr string `mapstructure:"bind_addr"`

	// SerfWanBindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip) Serf
	SerfWanBindAddr string `mapstructure:"serf_wan_bind"`

	// SerfLanBindAddr is used to control the address we bind to.
	// If not specified, the first private IP we find is used.
	// This controls the address we use for cluster facing
	// services (Gossip) Serf
	SerfLanBindAddr string `mapstructure:"serf_lan_bind"`

	// AdvertiseAddr is the address we use for advertising our Serf,
	// and Consul RPC IP. If not specified, bind address is used.
	AdvertiseAddr string `mapstructure:"advertise_addr"`

	// AdvertiseAddrs configuration
	AdvertiseAddrs AdvertiseAddrsConfig `mapstructure:"advertise_addrs"`

	// AdvertiseAddrWan is the address we use for advertising our
	// Serf WAN IP. If not specified, the general advertise address is used.
	AdvertiseAddrWan string `mapstructure:"advertise_addr_wan"`

	// TranslateWanAddrs controls whether or not Consul should prefer
	// the "wan" tagged address when doing lookups in remote datacenters.
	// See TaggedAddresses below for more details.
	TranslateWanAddrs bool `mapstructure:"translate_wan_addrs"`

	// Port configurations
	Ports PortConfig

	// Address configurations
	Addresses AddressConfig

	// Tagged addresses. These are used to publish a set of addresses for
	// for a node, which can be used by the remote agent. We currently
	// populate only the "wan" tag based on the SerfWan advertise address,
	// but this structure is here for possible future features with other
	// user-defined tags. The "wan" tag will be used by remote agents if
	// they are configured with TranslateWanAddrs set to true.
	TaggedAddresses map[string]string

	// Node metadata key/value pairs. These are excluded from JSON output
	// because they can be reloaded and might be stale when shown from the
	// config instead of the local state.
	Meta map[string]string `mapstructure:"node_meta" json:"-"`

	// LeaveOnTerm controls if Serf does a graceful leave when receiving
	// the TERM signal. Defaults true on clients, false on servers. This can
	// be changed on reload.
	LeaveOnTerm *bool `mapstructure:"leave_on_terminate"`

	// SkipLeaveOnInt controls if Serf skips a graceful leave when
	// receiving the INT signal. Defaults false on clients, true on
	// servers. This can be changed on reload.
	SkipLeaveOnInt *bool `mapstructure:"skip_leave_on_interrupt"`

	// Autopilot is used to configure helpful features for operating Consul servers.
	Autopilot Autopilot `mapstructure:"autopilot"`

	Telemetry Telemetry `mapstructure:"telemetry"`

	// Protocol is the Consul protocol version to use.
	Protocol int `mapstructure:"protocol"`

	// RaftProtocol sets the Raft protocol version to use on this server.
	RaftProtocol int `mapstructure:"raft_protocol"`

	// EnableDebug is used to enable various debugging features
	EnableDebug bool `mapstructure:"enable_debug"`

	// VerifyIncoming is used to verify the authenticity of incoming connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncoming bool `mapstructure:"verify_incoming"`

	// VerifyIncomingRPC is used to verify the authenticity of incoming RPC connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncomingRPC bool `mapstructure:"verify_incoming_rpc"`

	// VerifyIncomingHTTPS is used to verify the authenticity of incoming HTTPS connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncomingHTTPS bool `mapstructure:"verify_incoming_https"`

	// VerifyOutgoing is used to verify the authenticity of outgoing connections.
	// This means that TLS requests are used. TLS connections must match a provided
	// certificate authority. This is used to verify authenticity of server nodes.
	VerifyOutgoing bool `mapstructure:"verify_outgoing"`

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<datacenter>.<domain>.
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool `mapstructure:"verify_server_hostname"`

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string `mapstructure:"ca_file"`

	// CAPath is a path to a directory of certificate authority files. This is used with
	// VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	CAPath string `mapstructure:"ca_path"`

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string `mapstructure:"key_file"`

	// ServerName is used with the TLS certificates to ensure the name we
	// provide matches the certificate
	ServerName string `mapstructure:"server_name"`

	// TLSMinVersion is used to set the minimum TLS version used for TLS connections.
	TLSMinVersion string `mapstructure:"tls_min_version"`

	// TLSCipherSuites is used to specify the list of supported ciphersuites.
	TLSCipherSuites    []uint16 `mapstructure:"-" json:"-"`
	TLSCipherSuitesRaw string   `mapstructure:"tls_cipher_suites"`

	// TLSPreferServerCipherSuites specifies whether to prefer the server's ciphersuite
	// over the client ciphersuites.
	TLSPreferServerCipherSuites bool `mapstructure:"tls_prefer_server_cipher_suites"`

	// StartJoin is a list of addresses to attempt to join when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	StartJoin []string `mapstructure:"start_join"`

	// StartJoinWan is a list of addresses to attempt to join -wan when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	StartJoinWan []string `mapstructure:"start_join_wan"`

	// RetryJoin is a list of addresses to join with retry enabled.
	RetryJoin []string `mapstructure:"retry_join" json:"-"`

	// RetryMaxAttempts specifies the maximum number of times to retry joining a
	// host on startup. This is useful for cases where we know the node will be
	// online eventually.
	RetryMaxAttempts int `mapstructure:"retry_max"`

	// RetryInterval specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	RetryInterval    time.Duration `mapstructure:"-" json:"-"`
	RetryIntervalRaw string        `mapstructure:"retry_interval"`

	// RetryJoinWan is a list of addresses to join -wan with retry enabled.
	RetryJoinWan []string `mapstructure:"retry_join_wan"`

	// RetryMaxAttemptsWan specifies the maximum number of times to retry joining a
	// -wan host on startup. This is useful for cases where we know the node will be
	// online eventually.
	RetryMaxAttemptsWan int `mapstructure:"retry_max_wan"`

	// RetryIntervalWan specifies the amount of time to wait in between join
	// -wan attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	RetryIntervalWan    time.Duration `mapstructure:"-" json:"-"`
	RetryIntervalWanRaw string        `mapstructure:"retry_interval_wan"`

	// ReconnectTimeout* specify the amount of time to wait to reconnect with
	// another agent before deciding it's permanently gone. This can be used to
	// control the time it takes to reap failed nodes from the cluster.
	ReconnectTimeoutLan    time.Duration `mapstructure:"-"`
	ReconnectTimeoutLanRaw string        `mapstructure:"reconnect_timeout"`
	ReconnectTimeoutWan    time.Duration `mapstructure:"-"`
	ReconnectTimeoutWanRaw string        `mapstructure:"reconnect_timeout_wan"`

	// EnableUI enables the statically-compiled assets for the Consul web UI and
	// serves them at the default /ui/ endpoint automatically.
	EnableUI bool `mapstructure:"ui"`

	// UIDir is the directory containing the Web UI resources.
	// If provided, the UI endpoints will be enabled.
	UIDir string `mapstructure:"ui_dir"`

	// PidFile is the file to store our PID in
	PidFile string `mapstructure:"pid_file"`

	// EnableSyslog is used to also tee all the logs over to syslog. Only supported
	// on linux and OSX. Other platforms will generate an error.
	EnableSyslog bool `mapstructure:"enable_syslog"`

	// SyslogFacility is used to control where the syslog messages go
	// By default, goes to LOCAL0
	SyslogFacility string `mapstructure:"syslog_facility"`

	// RejoinAfterLeave controls our interaction with the cluster after leave.
	// When set to false (default), a leave causes Consul to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	RejoinAfterLeave bool `mapstructure:"rejoin_after_leave"`

	// EnableScriptChecks controls whether health checks which execute
	// scripts are enabled. This includes regular script checks and Docker
	// checks.
	EnableScriptChecks bool `mapstructure:"enable_script_checks"`

	// CheckUpdateInterval controls the interval on which the output of a health check
	// is updated if there is no change to the state. For example, a check in a steady
	// state may run every 5 second generating a unique output (timestamp, etc), forcing
	// constant writes. This allows Consul to defer the write for some period of time,
	// reducing the write pressure when the state is steady.
	CheckUpdateInterval    time.Duration `mapstructure:"-"`
	CheckUpdateIntervalRaw string        `mapstructure:"check_update_interval" json:"-"`

	// CheckReapInterval controls the interval on which we will look for
	// failed checks and reap their associated services, if so configured.
	CheckReapInterval time.Duration `mapstructure:"-"`

	// CheckDeregisterIntervalMin is the smallest allowed interval to set
	// a check's DeregisterCriticalServiceAfter value to.
	CheckDeregisterIntervalMin time.Duration `mapstructure:"-"`

	// ACLToken is the default token used to make requests if a per-request
	// token is not provided. If not configured the 'anonymous' token is used.
	ACLToken string `mapstructure:"acl_token" json:"-"`

	// ACLAgentMasterToken is a special token that has full read and write
	// privileges for this agent, and can be used to call agent endpoints
	// when no servers are available.
	ACLAgentMasterToken string `mapstructure:"acl_agent_master_token" json:"-"`

	// ACLAgentToken is the default token used to make requests for the agent
	// itself, such as for registering itself with the catalog. If not
	// configured, the 'acl_token' will be used.
	ACLAgentToken string `mapstructure:"acl_agent_token" json:"-"`

	// ACLMasterToken is used to bootstrap the ACL system. It should be specified
	// on the servers in the ACLDatacenter. When the leader comes online, it ensures
	// that the Master token is available. This provides the initial token.
	ACLMasterToken string `mapstructure:"acl_master_token" json:"-"`

	// ACLDatacenter is the central datacenter that holds authoritative
	// ACL records. This must be the same for the entire cluster.
	// If this is not set, ACLs are not enabled. Off by default.
	ACLDatacenter string `mapstructure:"acl_datacenter"`

	// ACLTTL is used to control the time-to-live of cached ACLs . This has
	// a major impact on performance. By default, it is set to 30 seconds.
	ACLTTL    time.Duration `mapstructure:"-"`
	ACLTTLRaw string        `mapstructure:"acl_ttl"`

	// ACLDefaultPolicy is used to control the ACL interaction when
	// there is no defined policy. This can be "allow" which means
	// ACLs are used to black-list, or "deny" which means ACLs are
	// white-lists.
	ACLDefaultPolicy string `mapstructure:"acl_default_policy"`

	// ACLDisabledTTL is used by clients to determine how long they will
	// wait to check again with the servers if they discover ACLs are not
	// enabled.
	ACLDisabledTTL time.Duration `mapstructure:"-"`

	// ACLDownPolicy is used to control the ACL interaction when we cannot
	// reach the ACLDatacenter and the token is not in the cache.
	// There are two modes:
	//   * allow - Allow all requests
	//   * deny - Deny all requests
	//   * extend-cache - Ignore the cache expiration, and allow cached
	//                    ACL's to be used to service requests. This
	//                    is the default. If the ACL is not in the cache,
	//                    this acts like deny.
	ACLDownPolicy string `mapstructure:"acl_down_policy"`

	// EnableACLReplication is used to turn on ACL replication when using
	// /v1/agent/token/acl_replication_token to introduce the token, instead
	// of setting acl_replication_token in the config. Setting the token via
	// config will also set this to true for backward compatibility.
	EnableACLReplication bool `mapstructure:"enable_acl_replication"`

	// ACLReplicationToken is used to fetch ACLs from the ACLDatacenter in
	// order to replicate them locally. Setting this to a non-empty value
	// also enables replication. Replication is only available in datacenters
	// other than the ACLDatacenter.
	ACLReplicationToken string `mapstructure:"acl_replication_token" json:"-"`

	// ACLEnforceVersion8 is used to gate a set of ACL policy features that
	// are opt-in prior to Consul 0.8 and opt-out in Consul 0.8 and later.
	ACLEnforceVersion8 *bool `mapstructure:"acl_enforce_version_8"`

	// Watches are used to monitor various endpoints and to invoke a
	// handler to act appropriately. These are managed entirely in the
	// agent layer using the standard APIs.
	Watches []map[string]interface{} `mapstructure:"watches"`

	// DisableRemoteExec is used to turn off the remote execution
	// feature. This is for security to prevent unknown scripts from running.
	DisableRemoteExec *bool `mapstructure:"disable_remote_exec"`

	// DisableUpdateCheck is used to turn off the automatic update and
	// security bulletin checking.
	DisableUpdateCheck bool `mapstructure:"disable_update_check"`

	// DisableAnonymousSignature is used to turn off the anonymous signature
	// send with the update check. This is used to deduplicate messages.
	DisableAnonymousSignature bool `mapstructure:"disable_anonymous_signature"`

	// AEInterval controls the anti-entropy interval. This is how often
	// the agent attempts to reconcile its local state with the server's
	// representation of our state. Defaults to every 60s.
	AEInterval time.Duration `mapstructure:"-" json:"-"`

	// DisableCoordinates controls features related to network coordinates.
	DisableCoordinates bool `mapstructure:"disable_coordinates"`

	// SyncCoordinateRateTarget controls the rate for sending network
	// coordinates to the server, in updates per second. This is the max rate
	// that the server supports, so we scale our interval based on the size
	// of the cluster to try to achieve this in aggregate at the server.
	SyncCoordinateRateTarget float64 `mapstructure:"-" json:"-"`

	// SyncCoordinateIntervalMin sets the minimum interval that coordinates
	// will be sent to the server. We scale the interval based on the cluster
	// size, but below a certain interval it doesn't make sense send them any
	// faster.
	SyncCoordinateIntervalMin time.Duration `mapstructure:"-" json:"-"`

	// Checks holds the provided check definitions
	Checks []*structs.CheckDefinition `mapstructure:"-" json:"-"`

	// Services holds the provided service definitions
	Services []*structs.ServiceDefinition `mapstructure:"-" json:"-"`

	// ConsulConfig can either be provided or a default one created
	ConsulConfig *consul.Config `mapstructure:"-" json:"-"`

	// Revision is the GitCommit this maps to
	Revision string `mapstructure:"-"`

	// Version is the release version number
	Version string `mapstructure:"-"`

	// VersionPrerelease is a label for pre-release builds
	VersionPrerelease string `mapstructure:"-"`

	// WatchPlans contains the compiled watches
	WatchPlans []*watch.Plan `mapstructure:"-" json:"-"`

	// UnixSockets is a map of socket configuration data
	UnixSockets UnixSocketConfig `mapstructure:"unix_sockets"`

	// Minimum Session TTL
	SessionTTLMin    time.Duration `mapstructure:"-"`
	SessionTTLMinRaw string        `mapstructure:"session_ttl_min"`

	// deprecated fields
	// keep them exported since otherwise the error messages don't show up
	DeprecatedAtlasInfrastructure    string            `mapstructure:"atlas_infrastructure" json:"-"`
	DeprecatedAtlasToken             string            `mapstructure:"atlas_token" json:"-"`
	DeprecatedAtlasACLToken          string            `mapstructure:"atlas_acl_token" json:"-"`
	DeprecatedAtlasJoin              bool              `mapstructure:"atlas_join" json:"-"`
	DeprecatedAtlasEndpoint          string            `mapstructure:"atlas_endpoint" json:"-"`
	DeprecatedHTTPAPIResponseHeaders map[string]string `mapstructure:"http_api_response_headers"`
	DeprecatedRetryJoinEC2           RetryJoinEC2      `mapstructure:"retry_join_ec2"`
	DeprecatedRetryJoinGCE           RetryJoinGCE      `mapstructure:"retry_join_gce"`
	DeprecatedRetryJoinAzure         RetryJoinAzure    `mapstructure:"retry_join_azure"`
}

// IncomingHTTPSConfig returns the TLS configuration for HTTPS
// connections to consul.
func (c *Config) IncomingHTTPSConfig() (*tls.Config, error) {
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

type ProtoAddr struct {
	Proto, Net, Addr string
}

func (p ProtoAddr) String() string {
	return p.Proto + "://" + p.Addr
}

func (c *Config) DNSAddrs() ([]ProtoAddr, error) {
	if c.Ports.DNS <= 0 {
		return nil, nil
	}
	a, err := c.ClientListener(c.Addresses.DNS, c.Ports.DNS)
	if err != nil {
		return nil, err
	}
	addrs := []ProtoAddr{
		{"dns", "tcp", a.String()},
		{"dns", "udp", a.String()},
	}
	return addrs, nil
}

// HTTPAddrs returns the bind addresses for the HTTP server and
// the application protocol which should be served, e.g. 'http'
// or 'https'.
func (c *Config) HTTPAddrs() ([]ProtoAddr, error) {
	var addrs []ProtoAddr
	if c.Ports.HTTP > 0 {
		a, err := c.ClientListener(c.Addresses.HTTP, c.Ports.HTTP)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, ProtoAddr{"http", a.Network(), a.String()})
	}
	if c.Ports.HTTPS > 0 && c.CertFile != "" && c.KeyFile != "" {
		a, err := c.ClientListener(c.Addresses.HTTPS, c.Ports.HTTPS)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, ProtoAddr{"https", a.Network(), a.String()})
	}
	return addrs, nil
}

// Bool is used to initialize bool pointers in struct literals.
func Bool(b bool) *bool {
	return &b
}

// Uint64 is used to initialize uint64 pointers in struct literals.
func Uint64(i uint64) *uint64 {
	return &i
}

// Duration is used to initialize time.Duration pointers in struct literals.
func Duration(d time.Duration) *time.Duration {
	return &d
}

// UnixSocketPermissions contains information about a unix socket, and
// implements the FilePermissions interface.
type UnixSocketPermissions struct {
	Usr   string `mapstructure:"user"`
	Grp   string `mapstructure:"group"`
	Perms string `mapstructure:"mode"`
}

func (u UnixSocketPermissions) User() string {
	return u.Usr
}

func (u UnixSocketPermissions) Group() string {
	return u.Grp
}

func (u UnixSocketPermissions) Mode() string {
	return u.Perms
}

func (s *Telemetry) GoString() string {
	return fmt.Sprintf("*%#v", *s)
}

// UnixSocketConfig stores information about various unix sockets which
// Consul creates and uses for communication.
type UnixSocketConfig struct {
	UnixSocketPermissions `mapstructure:",squash"`
}

// socketPath tests if a given address describes a domain socket,
// and returns the relevant path part of the string if it is.
func socketPath(addr string) string {
	if !strings.HasPrefix(addr, "unix://") {
		return ""
	}
	return strings.TrimPrefix(addr, "unix://")
}

type dirEnts []os.FileInfo

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		Bootstrap:       false,
		BootstrapExpect: 0,
		Server:          false,
		Datacenter:      consul.DefaultDC,
		Domain:          "consul.",
		LogLevel:        "INFO",
		ClientAddr:      "127.0.0.1",
		BindAddr:        "0.0.0.0",
		Ports: PortConfig{
			DNS:     8600,
			HTTP:    8500,
			HTTPS:   -1,
			SerfLan: consul.DefaultLANSerfPort,
			SerfWan: consul.DefaultWANSerfPort,
			Server:  8300,
		},
		DNSConfig: DNSConfig{
			AllowStale:      Bool(true),
			UDPAnswerLimit:  3,
			MaxStale:        10 * 365 * 24 * time.Hour,
			RecursorTimeout: 2 * time.Second,
		},
		Telemetry: Telemetry{
			StatsitePrefix: "consul",
			FilterDefault:  Bool(true),
		},
		Meta:                       make(map[string]string),
		SyslogFacility:             "LOCAL0",
		Protocol:                   consul.ProtocolVersion2Compatible,
		CheckUpdateInterval:        5 * time.Minute,
		CheckDeregisterIntervalMin: time.Minute,
		CheckReapInterval:          30 * time.Second,
		AEInterval:                 time.Minute,
		DisableCoordinates:         false,

		// SyncCoordinateRateTarget is set based on the rate that we want
		// the server to handle as an aggregate across the entire cluster.
		// If you update this, you'll need to adjust CoordinateUpdate* in
		// the server-side config accordingly.
		SyncCoordinateRateTarget:  64.0, // updates / second
		SyncCoordinateIntervalMin: 15 * time.Second,

		ACLTTL:             30 * time.Second,
		ACLDownPolicy:      "extend-cache",
		ACLDefaultPolicy:   "allow",
		ACLDisabledTTL:     120 * time.Second,
		ACLEnforceVersion8: Bool(true),
		DisableRemoteExec:  Bool(true),
		RetryInterval:      30 * time.Second,
		RetryIntervalWan:   30 * time.Second,

		TLSMinVersion: "tls10",

		EncryptVerifyIncoming: Bool(true),
		EncryptVerifyOutgoing: Bool(true),

		DisableHostNodeID: Bool(true),
	}
}

// DevConfig is used to return a set of configuration to use for dev mode.
func DevConfig() *Config {
	conf := DefaultConfig()
	conf.DevMode = true
	conf.LogLevel = "DEBUG"
	conf.Server = true
	conf.EnableDebug = true
	conf.DisableAnonymousSignature = true
	conf.EnableUI = true
	conf.BindAddr = "127.0.0.1"
	conf.DisableKeyringFile = true

	conf.ConsulConfig = consul.DefaultConfig()
	conf.ConsulConfig.SerfLANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	conf.ConsulConfig.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	conf.ConsulConfig.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	conf.ConsulConfig.SerfWANConfig.MemberlistConfig.SuspicionMult = 3
	conf.ConsulConfig.SerfWANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	conf.ConsulConfig.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	conf.ConsulConfig.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	conf.ConsulConfig.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	conf.ConsulConfig.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	conf.ConsulConfig.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	conf.ConsulConfig.CoordinateUpdatePeriod = 100 * time.Millisecond

	return conf
}

// EncryptBytes returns the encryption key configured.
func (c *Config) EncryptBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.EncryptKey)
}

// ClientListener is used to format a listener for a
// port on a ClientAddr
func (c *Config) ClientListener(override string, port int) (net.Addr, error) {
	addr := c.ClientAddr
	if override != "" {
		addr = override
	}
	if path := socketPath(addr); path != "" {
		return &net.UnixAddr{Name: path, Net: "unix"}, nil
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("Failed to parse IP: %v", addr)
	}
	return &net.TCPAddr{IP: ip, Port: port}, nil
}

// VerifyUniqueListeners checks to see if an address was used more than once in
// the config
func (c *Config) VerifyUniqueListeners() error {
	listeners := []struct {
		host  string
		port  int
		descr string
	}{
		{c.Addresses.DNS, c.Ports.DNS, "DNS"},
		{c.Addresses.HTTP, c.Ports.HTTP, "HTTP"},
		{c.Addresses.HTTPS, c.Ports.HTTPS, "HTTPS"},
		{c.AdvertiseAddr, c.Ports.Server, "Server RPC"},
		{c.AdvertiseAddr, c.Ports.SerfLan, "Serf LAN"},
		{c.AdvertiseAddr, c.Ports.SerfWan, "Serf WAN"},
	}

	type key struct {
		host string
		port int
	}
	m := make(map[key]string, len(listeners))

	for _, l := range listeners {
		if l.host == "" {
			l.host = "0.0.0.0"
		} else if strings.HasPrefix(l.host, "unix") {
			// Don't compare ports on unix sockets
			l.port = 0
		}
		if l.host == "0.0.0.0" && l.port <= 0 {
			continue
		}

		k := key{l.host, l.port}
		v, ok := m[k]
		if ok {
			return fmt.Errorf("%s address already configured for %s", l.descr, v)
		}
		m[k] = l.descr
	}
	return nil
}

// DecodeConfig reads the configuration from the given reader in JSON
// format and decodes it into a proper Config structure.
func DecodeConfig(r io.Reader) (*Config, error) {
	var raw interface{}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	// Check the result type
	var result Config
	if obj, ok := raw.(map[string]interface{}); ok {
		// Check for a "services", "service" or "check" key, meaning
		// this is actually a definition entry
		if sub, ok := obj["services"]; ok {
			if list, ok := sub.([]interface{}); ok {
				for _, srv := range list {
					service, err := DecodeServiceDefinition(srv)
					if err != nil {
						return nil, err
					}
					result.Services = append(result.Services, service)
				}
			}
		}
		if sub, ok := obj["service"]; ok {
			service, err := DecodeServiceDefinition(sub)
			if err != nil {
				return nil, err
			}
			result.Services = append(result.Services, service)
		}
		if sub, ok := obj["checks"]; ok {
			if list, ok := sub.([]interface{}); ok {
				for _, chk := range list {
					check, err := DecodeCheckDefinition(chk)
					if err != nil {
						return nil, err
					}
					result.Checks = append(result.Checks, check)
				}
			}
		}
		if sub, ok := obj["check"]; ok {
			check, err := DecodeCheckDefinition(sub)
			if err != nil {
				return nil, err
			}
			result.Checks = append(result.Checks, check)
		}

		// A little hacky but upgrades the old stats config directives to the new way
		if sub, ok := obj["statsd_addr"]; ok && result.Telemetry.StatsdAddr == "" {
			result.Telemetry.StatsdAddr = sub.(string)
		}

		if sub, ok := obj["statsite_addr"]; ok && result.Telemetry.StatsiteAddr == "" {
			result.Telemetry.StatsiteAddr = sub.(string)
		}

		if sub, ok := obj["statsite_prefix"]; ok && result.Telemetry.StatsitePrefix == "" {
			result.Telemetry.StatsitePrefix = sub.(string)
		}

		if sub, ok := obj["dogstatsd_addr"]; ok && result.Telemetry.DogStatsdAddr == "" {
			result.Telemetry.DogStatsdAddr = sub.(string)
		}

		if sub, ok := obj["dogstatsd_tags"].([]interface{}); ok && len(result.Telemetry.DogStatsdTags) == 0 {
			result.Telemetry.DogStatsdTags = make([]string, len(sub))
			for i := range sub {
				result.Telemetry.DogStatsdTags[i] = sub[i].(string)
			}
		}
	}

	// Decode
	var md mapstructure.Metadata
	msdec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   &result,
	})
	if err != nil {
		return nil, err
	}

	if err := msdec.Decode(raw); err != nil {
		return nil, err
	}

	// Check for deprecations
	if result.Ports.RPC != 0 {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: ports.rpc is deprecated and is "+
			"no longer used. Please remove it from your configuration.")
	}
	if result.Addresses.RPC != "" {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: addresses.rpc is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}
	if result.DeprecatedAtlasInfrastructure != "" {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: atlas_infrastructure is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}
	if result.DeprecatedAtlasToken != "" {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: atlas_token is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}
	if result.DeprecatedAtlasACLToken != "" {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: atlas_acl_token is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}
	if result.DeprecatedAtlasJoin != false {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: atlas_join is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}
	if result.DeprecatedAtlasEndpoint != "" {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: atlas_endpoint is deprecated and "+
			"is no longer used. Please remove it from your configuration.")
	}

	// Check unused fields and verify that no bad configuration options were
	// passed to Consul. There are a few additional fields which don't directly
	// use mapstructure decoding, so we need to account for those as well. These
	// telemetry-related fields used to be available as top-level keys, so they
	// are here for backward compatibility with the old format.
	allowedKeys := []string{
		"service", "services", "check", "checks", "statsd_addr", "statsite_addr", "statsite_prefix",
		"dogstatsd_addr", "dogstatsd_tags",
	}

	var unused []string
	for _, field := range md.Unused {
		if !lib.StrContains(allowedKeys, field) {
			unused = append(unused, field)
		}
	}
	if len(unused) > 0 {
		return nil, fmt.Errorf("Config has invalid keys: %s", strings.Join(unused, ","))
	}

	// Handle time conversions
	if raw := result.DNSConfig.NodeTTLRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("NodeTTL invalid: %v", err)
		}
		result.DNSConfig.NodeTTL = dur
	}

	if raw := result.DNSConfig.MaxStaleRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("MaxStale invalid: %v", err)
		}
		result.DNSConfig.MaxStale = dur
	}

	if raw := result.DNSConfig.RecursorTimeoutRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("RecursorTimeout invalid: %v", err)
		}
		result.DNSConfig.RecursorTimeout = dur
	}

	if len(result.DNSConfig.ServiceTTLRaw) != 0 {
		if result.DNSConfig.ServiceTTL == nil {
			result.DNSConfig.ServiceTTL = make(map[string]time.Duration)
		}
		for service, raw := range result.DNSConfig.ServiceTTLRaw {
			dur, err := time.ParseDuration(raw)
			if err != nil {
				return nil, fmt.Errorf("ServiceTTL %s invalid: %v", service, err)
			}
			result.DNSConfig.ServiceTTL[service] = dur
		}
	}

	if raw := result.CheckUpdateIntervalRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("CheckUpdateInterval invalid: %v", err)
		}
		result.CheckUpdateInterval = dur
	}

	if raw := result.ACLTTLRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("ACL TTL invalid: %v", err)
		}
		result.ACLTTL = dur
	}

	if raw := result.RetryIntervalRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("RetryInterval invalid: %v", err)
		}
		result.RetryInterval = dur
	}

	if raw := result.RetryIntervalWanRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("RetryIntervalWan invalid: %v", err)
		}
		result.RetryIntervalWan = dur
	}

	const reconnectTimeoutMin = 8 * time.Hour
	if raw := result.ReconnectTimeoutLanRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("ReconnectTimeoutLan invalid: %v", err)
		}
		if dur < reconnectTimeoutMin {
			return nil, fmt.Errorf("ReconnectTimeoutLan must be >= %s", reconnectTimeoutMin.String())
		}
		result.ReconnectTimeoutLan = dur
	}
	if raw := result.ReconnectTimeoutWanRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("ReconnectTimeoutWan invalid: %v", err)
		}
		if dur < reconnectTimeoutMin {
			return nil, fmt.Errorf("ReconnectTimeoutWan must be >= %s", reconnectTimeoutMin.String())
		}
		result.ReconnectTimeoutWan = dur
	}

	if raw := result.Autopilot.LastContactThresholdRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("LastContactThreshold invalid: %v", err)
		}
		result.Autopilot.LastContactThreshold = &dur
	}
	if raw := result.Autopilot.ServerStabilizationTimeRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("ServerStabilizationTime invalid: %v", err)
		}
		result.Autopilot.ServerStabilizationTime = &dur
	}

	// Merge the single recursor
	if result.DNSRecursor != "" {
		result.DNSRecursors = append(result.DNSRecursors, result.DNSRecursor)
	}

	if raw := result.SessionTTLMinRaw; raw != "" {
		dur, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("Session TTL Min invalid: %v", err)
		}
		result.SessionTTLMin = dur
	}

	if result.AdvertiseAddrs.SerfLanRaw != "" {
		ipStr, err := parseSingleIPTemplate(result.AdvertiseAddrs.SerfLanRaw)
		if err != nil {
			return nil, fmt.Errorf("Serf Advertise LAN address resolution failed: %v", err)
		}
		result.AdvertiseAddrs.SerfLanRaw = ipStr

		addr, err := net.ResolveTCPAddr("tcp", result.AdvertiseAddrs.SerfLanRaw)
		if err != nil {
			return nil, fmt.Errorf("AdvertiseAddrs.SerfLan is invalid: %v", err)
		}
		result.AdvertiseAddrs.SerfLan = addr
	}

	if result.AdvertiseAddrs.SerfWanRaw != "" {
		ipStr, err := parseSingleIPTemplate(result.AdvertiseAddrs.SerfWanRaw)
		if err != nil {
			return nil, fmt.Errorf("Serf Advertise WAN address resolution failed: %v", err)
		}
		result.AdvertiseAddrs.SerfWanRaw = ipStr

		addr, err := net.ResolveTCPAddr("tcp", result.AdvertiseAddrs.SerfWanRaw)
		if err != nil {
			return nil, fmt.Errorf("AdvertiseAddrs.SerfWan is invalid: %v", err)
		}
		result.AdvertiseAddrs.SerfWan = addr
	}

	if result.AdvertiseAddrs.RPCRaw != "" {
		ipStr, err := parseSingleIPTemplate(result.AdvertiseAddrs.RPCRaw)
		if err != nil {
			return nil, fmt.Errorf("RPC Advertise address resolution failed: %v", err)
		}
		result.AdvertiseAddrs.RPCRaw = ipStr

		addr, err := net.ResolveTCPAddr("tcp", result.AdvertiseAddrs.RPCRaw)
		if err != nil {
			return nil, fmt.Errorf("AdvertiseAddrs.RPC is invalid: %v", err)
		}
		result.AdvertiseAddrs.RPC = addr
	}

	// Enforce the max Raft multiplier.
	if result.Performance.RaftMultiplier > consul.MaxRaftMultiplier {
		return nil, fmt.Errorf("Performance.RaftMultiplier must be <= %d", consul.MaxRaftMultiplier)
	}

	if raw := result.TLSCipherSuitesRaw; raw != "" {
		ciphers, err := tlsutil.ParseCiphers(raw)
		if err != nil {
			return nil, fmt.Errorf("TLSCipherSuites invalid: %v", err)
		}
		result.TLSCipherSuites = ciphers
	}

	// This is for backwards compatibility.
	// HTTPAPIResponseHeaders has been replaced with HTTPConfig.ResponseHeaders
	if len(result.DeprecatedHTTPAPIResponseHeaders) > 0 {
		fmt.Fprintln(os.Stderr, "==> DEPRECATION: http_api_response_headers is deprecated and "+
			"is no longer used. Please use http_config.response_headers instead.")
		if result.HTTPConfig.ResponseHeaders == nil {
			result.HTTPConfig.ResponseHeaders = make(map[string]string)
		}
		for field, value := range result.DeprecatedHTTPAPIResponseHeaders {
			result.HTTPConfig.ResponseHeaders[field] = value
		}
		result.DeprecatedHTTPAPIResponseHeaders = nil
	}

	// Set the ACL replication enable if they set a token, for backwards
	// compatibility.
	if result.ACLReplicationToken != "" {
		result.EnableACLReplication = true
	}

	// Parse the metric filters
	for _, rule := range result.Telemetry.PrefixFilter {
		if rule == "" {
			return nil, fmt.Errorf("Cannot have empty filter rule in prefix_filter")
		}
		switch rule[0] {
		case '+':
			result.Telemetry.AllowedPrefixes = append(result.Telemetry.AllowedPrefixes, rule[1:])
		case '-':
			result.Telemetry.BlockedPrefixes = append(result.Telemetry.BlockedPrefixes, rule[1:])
		default:
			return nil, fmt.Errorf("Filter rule must begin with either '+' or '-': %q", rule)
		}
	}

	return &result, nil
}

// DecodeServiceDefinition is used to decode a service definition
func DecodeServiceDefinition(raw interface{}) (*structs.ServiceDefinition, error) {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		goto AFTER_FIX
	}

	// If no 'tags', handle the deprecated 'tag' value.
	if _, ok := rawMap["tags"]; !ok {
		if tag, ok := rawMap["tag"]; ok {
			rawMap["tags"] = []interface{}{tag}
		}
	}

	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "check":
			if err := FixupCheckType(v); err != nil {
				return nil, err
			}
		case "checks":
			chkTypes, ok := v.([]interface{})
			if !ok {
				goto AFTER_FIX
			}
			for _, chkType := range chkTypes {
				if err := FixupCheckType(chkType); err != nil {
					return nil, err
				}
			}
		}
	}
AFTER_FIX:
	var md mapstructure.Metadata
	var result structs.ServiceDefinition
	msdec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   &result,
	})
	if err != nil {
		return nil, err
	}
	if err := msdec.Decode(raw); err != nil {
		return nil, err
	}
	return &result, nil
}

var errInvalidHeaderFormat = errors.New("agent: invalid format of 'header' field")

func FixupCheckType(raw interface{}) error {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	parseDuration := func(v interface{}) (time.Duration, error) {
		if v == nil {
			return 0, nil
		}
		switch x := v.(type) {
		case time.Duration:
			return x, nil
		case float64:
			return time.Duration(x), nil
		case string:
			return time.ParseDuration(x)
		default:
			return 0, fmt.Errorf("invalid format")
		}
	}

	parseHeaderMap := func(v interface{}) (map[string][]string, error) {
		if v == nil {
			return nil, nil
		}
		vm, ok := v.(map[string]interface{})
		if !ok {
			return nil, errInvalidHeaderFormat
		}
		m := map[string][]string{}
		for k, vv := range vm {
			vs, ok := vv.([]interface{})
			if !ok {
				return nil, errInvalidHeaderFormat
			}
			for _, vs := range vs {
				s, ok := vs.(string)
				if !ok {
					return nil, errInvalidHeaderFormat
				}
				m[k] = append(m[k], s)
			}
		}
		return m, nil
	}

	replace := func(oldKey, newKey string, val interface{}) {
		rawMap[newKey] = val
		if oldKey != newKey {
			delete(rawMap, oldKey)
		}
	}

	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "header":
			h, err := parseHeaderMap(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %s", k, err)
			}
			rawMap[k] = h

		case "ttl", "interval", "timeout":
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %v", k, err)
			}
			rawMap[k] = d

		case "deregister_critical_service_after", "deregistercriticalserviceafter":
			d, err := parseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid %q: %v", k, err)
			}
			replace(k, "DeregisterCriticalServiceAfter", d)

		case "docker_container_id":
			replace(k, "DockerContainerID", v)

		case "service_id":
			replace(k, "ServiceID", v)

		case "tls_skip_verify":
			replace(k, "TLSSkipVerify", v)
		}
	}
	return nil
}

// DecodeCheckDefinition is used to decode a check definition
func DecodeCheckDefinition(raw interface{}) (*structs.CheckDefinition, error) {
	if err := FixupCheckType(raw); err != nil {
		return nil, err
	}
	var md mapstructure.Metadata
	var result structs.CheckDefinition
	msdec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   &result,
	})
	if err != nil {
		return nil, err
	}
	if err := msdec.Decode(raw); err != nil {
		return nil, err
	}
	return &result, nil
}

// MergeConfig merges two configurations together to make a single new
// configuration.
func MergeConfig(a, b *Config) *Config {
	var result Config = *a

	// Propagate non-default performance settings
	if b.Performance.RaftMultiplier > 0 {
		result.Performance.RaftMultiplier = b.Performance.RaftMultiplier
	}

	// Copy the strings if they're set
	if b.Bootstrap {
		result.Bootstrap = true
	}
	if b.BootstrapExpect != 0 {
		result.BootstrapExpect = b.BootstrapExpect
	}
	if b.Datacenter != "" {
		result.Datacenter = b.Datacenter
	}
	if b.DataDir != "" {
		result.DataDir = b.DataDir
	}

	// Copy the dns recursors
	result.DNSRecursors = make([]string, 0, len(a.DNSRecursors)+len(b.DNSRecursors))
	result.DNSRecursors = append(result.DNSRecursors, a.DNSRecursors...)
	result.DNSRecursors = append(result.DNSRecursors, b.DNSRecursors...)

	if b.Domain != "" {
		result.Domain = b.Domain
	}
	if b.EncryptKey != "" {
		result.EncryptKey = b.EncryptKey
	}
	if b.DisableKeyringFile {
		result.DisableKeyringFile = true
	}
	if b.EncryptVerifyIncoming != nil {
		result.EncryptVerifyIncoming = b.EncryptVerifyIncoming
	}
	if b.EncryptVerifyOutgoing != nil {
		result.EncryptVerifyOutgoing = b.EncryptVerifyOutgoing
	}
	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.Protocol > 0 {
		result.Protocol = b.Protocol
	}
	if b.RaftProtocol > 0 {
		result.RaftProtocol = b.RaftProtocol
	}
	if b.NodeID != "" {
		result.NodeID = b.NodeID
	}
	if b.DisableHostNodeID != nil {
		result.DisableHostNodeID = b.DisableHostNodeID
	}
	if b.NodeName != "" {
		result.NodeName = b.NodeName
	}
	if b.ClientAddr != "" {
		result.ClientAddr = b.ClientAddr
	}
	if b.BindAddr != "" {
		result.BindAddr = b.BindAddr
	}
	if b.AdvertiseAddr != "" {
		result.AdvertiseAddr = b.AdvertiseAddr
	}
	if b.AdvertiseAddrWan != "" {
		result.AdvertiseAddrWan = b.AdvertiseAddrWan
	}
	if b.SerfWanBindAddr != "" {
		result.SerfWanBindAddr = b.SerfWanBindAddr
	}
	if b.SerfLanBindAddr != "" {
		result.SerfLanBindAddr = b.SerfLanBindAddr
	}
	if b.TranslateWanAddrs == true {
		result.TranslateWanAddrs = true
	}
	if b.AdvertiseAddrs.SerfLan != nil {
		result.AdvertiseAddrs.SerfLan = b.AdvertiseAddrs.SerfLan
		result.AdvertiseAddrs.SerfLanRaw = b.AdvertiseAddrs.SerfLanRaw
	}
	if b.AdvertiseAddrs.SerfWan != nil {
		result.AdvertiseAddrs.SerfWan = b.AdvertiseAddrs.SerfWan
		result.AdvertiseAddrs.SerfWanRaw = b.AdvertiseAddrs.SerfWanRaw
	}
	if b.AdvertiseAddrs.RPC != nil {
		result.AdvertiseAddrs.RPC = b.AdvertiseAddrs.RPC
		result.AdvertiseAddrs.RPCRaw = b.AdvertiseAddrs.RPCRaw
	}
	if b.Server == true {
		result.Server = b.Server
	}
	if b.NonVotingServer == true {
		result.NonVotingServer = b.NonVotingServer
	}
	if b.LeaveOnTerm != nil {
		result.LeaveOnTerm = b.LeaveOnTerm
	}
	if b.SkipLeaveOnInt != nil {
		result.SkipLeaveOnInt = b.SkipLeaveOnInt
	}
	if b.Autopilot.CleanupDeadServers != nil {
		result.Autopilot.CleanupDeadServers = b.Autopilot.CleanupDeadServers
	}
	if b.Autopilot.LastContactThreshold != nil {
		result.Autopilot.LastContactThreshold = b.Autopilot.LastContactThreshold
	}
	if b.Autopilot.MaxTrailingLogs != nil {
		result.Autopilot.MaxTrailingLogs = b.Autopilot.MaxTrailingLogs
	}
	if b.Autopilot.ServerStabilizationTime != nil {
		result.Autopilot.ServerStabilizationTime = b.Autopilot.ServerStabilizationTime
	}
	if b.Autopilot.RedundancyZoneTag != "" {
		result.Autopilot.RedundancyZoneTag = b.Autopilot.RedundancyZoneTag
	}
	if b.Autopilot.DisableUpgradeMigration != nil {
		result.Autopilot.DisableUpgradeMigration = b.Autopilot.DisableUpgradeMigration
	}
	if b.Autopilot.UpgradeVersionTag != "" {
		result.Autopilot.UpgradeVersionTag = b.Autopilot.UpgradeVersionTag
	}
	if b.Telemetry.DisableHostname == true {
		result.Telemetry.DisableHostname = true
	}
	if len(b.Telemetry.PrefixFilter) != 0 {
		result.Telemetry.PrefixFilter = append(result.Telemetry.PrefixFilter, b.Telemetry.PrefixFilter...)
	}
	if b.Telemetry.FilterDefault != nil {
		result.Telemetry.FilterDefault = b.Telemetry.FilterDefault
	}
	if b.Telemetry.StatsdAddr != "" {
		result.Telemetry.StatsdAddr = b.Telemetry.StatsdAddr
	}
	if b.Telemetry.StatsiteAddr != "" {
		result.Telemetry.StatsiteAddr = b.Telemetry.StatsiteAddr
	}
	if b.Telemetry.StatsitePrefix != "" {
		result.Telemetry.StatsitePrefix = b.Telemetry.StatsitePrefix
	}
	if b.Telemetry.DogStatsdAddr != "" {
		result.Telemetry.DogStatsdAddr = b.Telemetry.DogStatsdAddr
	}
	if b.Telemetry.DogStatsdTags != nil {
		result.Telemetry.DogStatsdTags = b.Telemetry.DogStatsdTags
	}
	if b.Telemetry.CirconusAPIToken != "" {
		result.Telemetry.CirconusAPIToken = b.Telemetry.CirconusAPIToken
	}
	if b.Telemetry.CirconusAPIApp != "" {
		result.Telemetry.CirconusAPIApp = b.Telemetry.CirconusAPIApp
	}
	if b.Telemetry.CirconusAPIURL != "" {
		result.Telemetry.CirconusAPIURL = b.Telemetry.CirconusAPIURL
	}
	if b.Telemetry.CirconusCheckSubmissionURL != "" {
		result.Telemetry.CirconusCheckSubmissionURL = b.Telemetry.CirconusCheckSubmissionURL
	}
	if b.Telemetry.CirconusSubmissionInterval != "" {
		result.Telemetry.CirconusSubmissionInterval = b.Telemetry.CirconusSubmissionInterval
	}
	if b.Telemetry.CirconusCheckID != "" {
		result.Telemetry.CirconusCheckID = b.Telemetry.CirconusCheckID
	}
	if b.Telemetry.CirconusCheckForceMetricActivation != "" {
		result.Telemetry.CirconusCheckForceMetricActivation = b.Telemetry.CirconusCheckForceMetricActivation
	}
	if b.Telemetry.CirconusCheckInstanceID != "" {
		result.Telemetry.CirconusCheckInstanceID = b.Telemetry.CirconusCheckInstanceID
	}
	if b.Telemetry.CirconusCheckSearchTag != "" {
		result.Telemetry.CirconusCheckSearchTag = b.Telemetry.CirconusCheckSearchTag
	}
	if b.Telemetry.CirconusCheckDisplayName != "" {
		result.Telemetry.CirconusCheckDisplayName = b.Telemetry.CirconusCheckDisplayName
	}
	if b.Telemetry.CirconusCheckTags != "" {
		result.Telemetry.CirconusCheckTags = b.Telemetry.CirconusCheckTags
	}
	if b.Telemetry.CirconusBrokerID != "" {
		result.Telemetry.CirconusBrokerID = b.Telemetry.CirconusBrokerID
	}
	if b.Telemetry.CirconusBrokerSelectTag != "" {
		result.Telemetry.CirconusBrokerSelectTag = b.Telemetry.CirconusBrokerSelectTag
	}
	if b.EnableDebug {
		result.EnableDebug = true
	}
	if b.VerifyIncoming {
		result.VerifyIncoming = true
	}
	if b.VerifyIncomingRPC {
		result.VerifyIncomingRPC = true
	}
	if b.VerifyIncomingHTTPS {
		result.VerifyIncomingHTTPS = true
	}
	if b.VerifyOutgoing {
		result.VerifyOutgoing = true
	}
	if b.VerifyServerHostname {
		result.VerifyServerHostname = true
	}
	if b.CAFile != "" {
		result.CAFile = b.CAFile
	}
	if b.CAPath != "" {
		result.CAPath = b.CAPath
	}
	if b.CertFile != "" {
		result.CertFile = b.CertFile
	}
	if b.KeyFile != "" {
		result.KeyFile = b.KeyFile
	}
	if b.ServerName != "" {
		result.ServerName = b.ServerName
	}
	if b.TLSMinVersion != "" {
		result.TLSMinVersion = b.TLSMinVersion
	}
	if len(b.TLSCipherSuites) != 0 {
		result.TLSCipherSuites = append(result.TLSCipherSuites, b.TLSCipherSuites...)
	}
	if b.TLSPreferServerCipherSuites {
		result.TLSPreferServerCipherSuites = true
	}
	if b.Checks != nil {
		result.Checks = append(result.Checks, b.Checks...)
	}
	if b.Services != nil {
		result.Services = append(result.Services, b.Services...)
	}
	if b.Ports.DNS != 0 {
		result.Ports.DNS = b.Ports.DNS
	}
	if b.Ports.HTTP != 0 {
		result.Ports.HTTP = b.Ports.HTTP
	}
	if b.Ports.HTTPS != 0 {
		result.Ports.HTTPS = b.Ports.HTTPS
	}
	if b.Ports.RPC != 0 {
		result.Ports.RPC = b.Ports.RPC
	}
	if b.Ports.SerfLan != 0 {
		result.Ports.SerfLan = b.Ports.SerfLan
	}
	if b.Ports.SerfWan != 0 {
		result.Ports.SerfWan = b.Ports.SerfWan
	}
	if b.Ports.Server != 0 {
		result.Ports.Server = b.Ports.Server
	}
	if b.Addresses.DNS != "" {
		result.Addresses.DNS = b.Addresses.DNS
	}
	if b.Addresses.HTTP != "" {
		result.Addresses.HTTP = b.Addresses.HTTP
	}
	if b.Addresses.HTTPS != "" {
		result.Addresses.HTTPS = b.Addresses.HTTPS
	}
	if b.Addresses.RPC != "" {
		result.Addresses.RPC = b.Addresses.RPC
	}
	if b.EnableUI {
		result.EnableUI = true
	}
	if b.UIDir != "" {
		result.UIDir = b.UIDir
	}
	if b.PidFile != "" {
		result.PidFile = b.PidFile
	}
	if b.EnableSyslog {
		result.EnableSyslog = true
	}
	if b.RejoinAfterLeave {
		result.RejoinAfterLeave = true
	}
	if b.RetryMaxAttempts != 0 {
		result.RetryMaxAttempts = b.RetryMaxAttempts
	}
	if b.RetryInterval != 0 {
		result.RetryInterval = b.RetryInterval
	}
	if b.DeprecatedRetryJoinEC2.AccessKeyID != "" {
		result.DeprecatedRetryJoinEC2.AccessKeyID = b.DeprecatedRetryJoinEC2.AccessKeyID
	}
	if b.DeprecatedRetryJoinEC2.SecretAccessKey != "" {
		result.DeprecatedRetryJoinEC2.SecretAccessKey = b.DeprecatedRetryJoinEC2.SecretAccessKey
	}
	if b.DeprecatedRetryJoinEC2.Region != "" {
		result.DeprecatedRetryJoinEC2.Region = b.DeprecatedRetryJoinEC2.Region
	}
	if b.DeprecatedRetryJoinEC2.TagKey != "" {
		result.DeprecatedRetryJoinEC2.TagKey = b.DeprecatedRetryJoinEC2.TagKey
	}
	if b.DeprecatedRetryJoinEC2.TagValue != "" {
		result.DeprecatedRetryJoinEC2.TagValue = b.DeprecatedRetryJoinEC2.TagValue
	}
	if b.DeprecatedRetryJoinGCE.ProjectName != "" {
		result.DeprecatedRetryJoinGCE.ProjectName = b.DeprecatedRetryJoinGCE.ProjectName
	}
	if b.DeprecatedRetryJoinGCE.ZonePattern != "" {
		result.DeprecatedRetryJoinGCE.ZonePattern = b.DeprecatedRetryJoinGCE.ZonePattern
	}
	if b.DeprecatedRetryJoinGCE.TagValue != "" {
		result.DeprecatedRetryJoinGCE.TagValue = b.DeprecatedRetryJoinGCE.TagValue
	}
	if b.DeprecatedRetryJoinGCE.CredentialsFile != "" {
		result.DeprecatedRetryJoinGCE.CredentialsFile = b.DeprecatedRetryJoinGCE.CredentialsFile
	}
	if b.DeprecatedRetryJoinAzure.TagName != "" {
		result.DeprecatedRetryJoinAzure.TagName = b.DeprecatedRetryJoinAzure.TagName
	}
	if b.DeprecatedRetryJoinAzure.TagValue != "" {
		result.DeprecatedRetryJoinAzure.TagValue = b.DeprecatedRetryJoinAzure.TagValue
	}
	if b.DeprecatedRetryJoinAzure.SubscriptionID != "" {
		result.DeprecatedRetryJoinAzure.SubscriptionID = b.DeprecatedRetryJoinAzure.SubscriptionID
	}
	if b.DeprecatedRetryJoinAzure.TenantID != "" {
		result.DeprecatedRetryJoinAzure.TenantID = b.DeprecatedRetryJoinAzure.TenantID
	}
	if b.DeprecatedRetryJoinAzure.ClientID != "" {
		result.DeprecatedRetryJoinAzure.ClientID = b.DeprecatedRetryJoinAzure.ClientID
	}
	if b.DeprecatedRetryJoinAzure.SecretAccessKey != "" {
		result.DeprecatedRetryJoinAzure.SecretAccessKey = b.DeprecatedRetryJoinAzure.SecretAccessKey
	}
	if b.RetryMaxAttemptsWan != 0 {
		result.RetryMaxAttemptsWan = b.RetryMaxAttemptsWan
	}
	if b.RetryIntervalWan != 0 {
		result.RetryIntervalWan = b.RetryIntervalWan
	}
	if b.ReconnectTimeoutLan != 0 {
		result.ReconnectTimeoutLan = b.ReconnectTimeoutLan
		result.ReconnectTimeoutLanRaw = b.ReconnectTimeoutLanRaw
	}
	if b.ReconnectTimeoutWan != 0 {
		result.ReconnectTimeoutWan = b.ReconnectTimeoutWan
		result.ReconnectTimeoutWanRaw = b.ReconnectTimeoutWanRaw
	}
	if b.DNSConfig.NodeTTL != 0 {
		result.DNSConfig.NodeTTL = b.DNSConfig.NodeTTL
	}
	if len(b.DNSConfig.ServiceTTL) != 0 {
		if result.DNSConfig.ServiceTTL == nil {
			result.DNSConfig.ServiceTTL = make(map[string]time.Duration)
		}
		for service, dur := range b.DNSConfig.ServiceTTL {
			result.DNSConfig.ServiceTTL[service] = dur
		}
	}
	if b.DNSConfig.AllowStale != nil {
		result.DNSConfig.AllowStale = b.DNSConfig.AllowStale
	}
	if b.DNSConfig.UDPAnswerLimit != 0 {
		result.DNSConfig.UDPAnswerLimit = b.DNSConfig.UDPAnswerLimit
	}
	if b.DNSConfig.EnableTruncate {
		result.DNSConfig.EnableTruncate = true
	}
	if b.DNSConfig.MaxStale != 0 {
		result.DNSConfig.MaxStale = b.DNSConfig.MaxStale
	}
	if b.DNSConfig.OnlyPassing {
		result.DNSConfig.OnlyPassing = true
	}
	if b.DNSConfig.DisableCompression {
		result.DNSConfig.DisableCompression = true
	}
	if b.DNSConfig.RecursorTimeout != 0 {
		result.DNSConfig.RecursorTimeout = b.DNSConfig.RecursorTimeout
	}
	if b.EnableScriptChecks {
		result.EnableScriptChecks = true
	}
	if b.CheckUpdateIntervalRaw != "" || b.CheckUpdateInterval != 0 {
		result.CheckUpdateInterval = b.CheckUpdateInterval
	}
	if b.SyslogFacility != "" {
		result.SyslogFacility = b.SyslogFacility
	}
	if b.ACLToken != "" {
		result.ACLToken = b.ACLToken
	}
	if b.ACLAgentMasterToken != "" {
		result.ACLAgentMasterToken = b.ACLAgentMasterToken
	}
	if b.ACLAgentToken != "" {
		result.ACLAgentToken = b.ACLAgentToken
	}
	if b.ACLMasterToken != "" {
		result.ACLMasterToken = b.ACLMasterToken
	}
	if b.ACLDatacenter != "" {
		result.ACLDatacenter = b.ACLDatacenter
	}
	if b.ACLTTLRaw != "" {
		result.ACLTTL = b.ACLTTL
		result.ACLTTLRaw = b.ACLTTLRaw
	}
	if b.ACLDownPolicy != "" {
		result.ACLDownPolicy = b.ACLDownPolicy
	}
	if b.ACLDefaultPolicy != "" {
		result.ACLDefaultPolicy = b.ACLDefaultPolicy
	}
	if b.EnableACLReplication {
		result.EnableACLReplication = true
	}
	if b.ACLReplicationToken != "" {
		result.ACLReplicationToken = b.ACLReplicationToken
	}
	if b.ACLEnforceVersion8 != nil {
		result.ACLEnforceVersion8 = b.ACLEnforceVersion8
	}
	if len(b.Watches) != 0 {
		result.Watches = append(result.Watches, b.Watches...)
	}
	if len(b.WatchPlans) != 0 {
		result.WatchPlans = append(result.WatchPlans, b.WatchPlans...)
	}
	if b.DisableRemoteExec != nil {
		result.DisableRemoteExec = b.DisableRemoteExec
	}
	if b.DisableUpdateCheck {
		result.DisableUpdateCheck = true
	}
	if b.DisableAnonymousSignature {
		result.DisableAnonymousSignature = true
	}
	if b.UnixSockets.Usr != "" {
		result.UnixSockets.Usr = b.UnixSockets.Usr
	}
	if b.UnixSockets.Grp != "" {
		result.UnixSockets.Grp = b.UnixSockets.Grp
	}
	if b.UnixSockets.Perms != "" {
		result.UnixSockets.Perms = b.UnixSockets.Perms
	}
	if b.DisableCoordinates {
		result.DisableCoordinates = true
	}
	if b.SessionTTLMinRaw != "" {
		result.SessionTTLMin = b.SessionTTLMin
		result.SessionTTLMinRaw = b.SessionTTLMinRaw
	}

	result.HTTPConfig.BlockEndpoints = append(a.HTTPConfig.BlockEndpoints,
		b.HTTPConfig.BlockEndpoints...)
	if len(b.HTTPConfig.ResponseHeaders) > 0 {
		if result.HTTPConfig.ResponseHeaders == nil {
			result.HTTPConfig.ResponseHeaders = make(map[string]string)
		}
		for field, value := range b.HTTPConfig.ResponseHeaders {
			result.HTTPConfig.ResponseHeaders[field] = value
		}
	}

	if len(b.Meta) != 0 {
		if result.Meta == nil {
			result.Meta = make(map[string]string)
		}
		for field, value := range b.Meta {
			result.Meta[field] = value
		}
	}

	// Copy the start join addresses
	result.StartJoin = make([]string, 0, len(a.StartJoin)+len(b.StartJoin))
	result.StartJoin = append(result.StartJoin, a.StartJoin...)
	result.StartJoin = append(result.StartJoin, b.StartJoin...)

	// Copy the start join addresses
	result.StartJoinWan = make([]string, 0, len(a.StartJoinWan)+len(b.StartJoinWan))
	result.StartJoinWan = append(result.StartJoinWan, a.StartJoinWan...)
	result.StartJoinWan = append(result.StartJoinWan, b.StartJoinWan...)

	// Copy the retry join addresses
	result.RetryJoin = make([]string, 0, len(a.RetryJoin)+len(b.RetryJoin))
	result.RetryJoin = append(result.RetryJoin, a.RetryJoin...)
	result.RetryJoin = append(result.RetryJoin, b.RetryJoin...)

	// Copy the retry join -wan addresses
	result.RetryJoinWan = make([]string, 0, len(a.RetryJoinWan)+len(b.RetryJoinWan))
	result.RetryJoinWan = append(result.RetryJoinWan, a.RetryJoinWan...)
	result.RetryJoinWan = append(result.RetryJoinWan, b.RetryJoinWan...)

	return &result
}

// ReadConfigPaths reads the paths in the given order to load configurations.
// The paths can be to files or directories. If the path is a directory,
// we read one directory deep and read any files ending in ".json" as
// configuration files.
func ReadConfigPaths(paths []string) (*Config, error) {
	result := new(Config)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		if !fi.IsDir() {
			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", path, err)
			}

			result = MergeConfig(result, config)
			continue
		}

		contents, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		// Sort the contents, ensures lexical order
		sort.Sort(dirEnts(contents))

		for _, fi := range contents {
			// Don't recursively read contents
			if fi.IsDir() {
				continue
			}

			// If it isn't a JSON file, ignore it
			if !strings.HasSuffix(fi.Name(), ".json") {
				continue
			}
			// If the config file is empty, ignore it
			if fi.Size() == 0 {
				continue
			}

			subpath := filepath.Join(path, fi.Name())
			f, err := os.Open(subpath)
			if err != nil {
				return nil, fmt.Errorf("Error reading '%s': %s", subpath, err)
			}

			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", subpath, err)
			}

			result = MergeConfig(result, config)
		}
	}

	return result, nil
}

// ResolveTmplAddrs iterates over the myriad of addresses in the agent's config
// and performs go-sockaddr/template Parse on each known address in case the
// user specified a template config for any of their values.
func (c *Config) ResolveTmplAddrs() (err error) {
	parse := func(addr *string, socketAllowed bool, name string) {
		if *addr == "" || err != nil {
			return
		}
		var ip string
		ip, err = parseSingleIPTemplate(*addr)
		if err != nil {
			err = fmt.Errorf("Resolution of %s failed: %v", name, err)
			return
		}
		ipAddr := net.ParseIP(ip)
		if !socketAllowed && ipAddr == nil {
			err = fmt.Errorf("Failed to parse %s: %v", name, ip)
			return
		}
		if socketAllowed && socketPath(ip) == "" && ipAddr == nil {
			err = fmt.Errorf("Failed to parse %s, %q is not a valid IP address or socket", name, ip)
			return
		}

		*addr = ip
	}

	if c == nil {
		return
	}
	parse(&c.Addresses.DNS, true, "DNS address")
	parse(&c.Addresses.HTTP, true, "HTTP address")
	parse(&c.Addresses.HTTPS, true, "HTTPS address")
	parse(&c.AdvertiseAddr, false, "Advertise address")
	parse(&c.AdvertiseAddrWan, false, "Advertise WAN address")
	parse(&c.BindAddr, true, "Bind address")
	parse(&c.ClientAddr, true, "Client address")
	parse(&c.SerfLanBindAddr, false, "Serf LAN address")
	parse(&c.SerfWanBindAddr, false, "Serf WAN address")

	return
}

// SetupTaggedAndAdvertiseAddrs configures advertise addresses and sets up a map of tagged addresses
func (cfg *Config) SetupTaggedAndAdvertiseAddrs() error {
	if cfg.AdvertiseAddr == "" {
		switch {

		case cfg.BindAddr != "" && !ipaddr.IsAny(cfg.BindAddr):
			cfg.AdvertiseAddr = cfg.BindAddr

		default:
			ip, err := consul.GetPrivateIP()
			if ipaddr.IsAnyV6(cfg.BindAddr) {
				ip, err = consul.GetPublicIPv6()
			}
			if err != nil {
				return fmt.Errorf("Failed to get advertise address: %v", err)
			}
			cfg.AdvertiseAddr = ip.String()
		}
	}

	// Try to get an advertise address for the wan
	if cfg.AdvertiseAddrWan == "" {
		cfg.AdvertiseAddrWan = cfg.AdvertiseAddr
	}

	// Create the default set of tagged addresses.
	cfg.TaggedAddresses = map[string]string{
		"lan": cfg.AdvertiseAddr,
		"wan": cfg.AdvertiseAddrWan,
	}
	return nil
}

// parseSingleIPTemplate is used as a helper function to parse out a single IP
// address from a config parameter.
func parseSingleIPTemplate(ipTmpl string) (string, error) {
	out, err := template.Parse(ipTmpl)
	if err != nil {
		return "", fmt.Errorf("Unable to parse address template %q: %v", ipTmpl, err)
	}

	ips := strings.Split(out, " ")
	switch len(ips) {
	case 0:
		return "", errors.New("No addresses found, please configure one.")
	case 1:
		return ips[0], nil
	default:
		return "", fmt.Errorf("Multiple addresses found (%q), please configure one.", out)
	}
}

// Implement the sort interface for dirEnts
func (d dirEnts) Len() int {
	return len(d)
}

func (d dirEnts) Less(i, j int) bool {
	return d[i].Name() < d[j].Name()
}

func (d dirEnts) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

// ParseMetaPair parses a key/value pair of the form key:value
func ParseMetaPair(raw string) (string, string) {
	pair := strings.SplitN(raw, ":", 2)
	if len(pair) == 2 {
		return pair[0], pair[1]
	}
	return pair[0], ""
}
