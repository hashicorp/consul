package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
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
	//   * async-cache - Same behaviour as extend-cache, but perform ACL
	//                   Lookups asynchronously when cache TTL is expired.
	//
	// hcl: acl_down_policy = ("allow"|"deny"|"extend-cache"|"async-cache")
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
	// (Enterprise-only)
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

	// DNSARecordLimit is used to limit the maximum number of DNS Resource
	// Records returned in the ANSWER section of a DNS response for A or AAAA
	// records for both UDP and TCP queries.
	//
	// This is not normally useful and will be limited based on the querying
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
	// hcl: dns_config { a_record_limit = int }
	DNSARecordLimit int

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
	// Records returned in the ANSWER section of a DNS response for UDP
	// responses without EDNS support (limited to 512 bytes).
	// This parameter is deprecated, if you want to limit the number of
	// records returned by A or AAAA questions, please use DNSARecordLimit
	// instead.
	//
	// hcl: dns_config { udp_answer_limit = int }
	DNSUDPAnswerLimit int

	// DNSNodeMetaTXT controls whether DNS queries will synthesize
	// TXT records for the node metadata and add them when not specifically
	// request (query type = TXT). If unset this will default to true
	DNSNodeMetaTXT bool

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

	// Embed Telemetry Config
	Telemetry lib.TelemetryConfig

	// Datacenter is the datacenter this node is in. Defaults to "dc1".
	//
	// Datacenter is exposed via /v1/agent/self from here and
	// used in lots of places like CLI commands. Treat this as an interface
	// that must be stable.
	//
	// hcl: datacenter = string
	// flag: -datacenter string
	Datacenter string

	// Defines the maximum stale value for discovery path. Defauls to "0s".
	// Discovery paths are /v1/heath/ paths
	//
	// If not set to 0, it will try to perform stale read and perform only a
	// consistent read whenever the value is too old.
	// hcl: discovery_max_stale = "duration"
	DiscoveryMaxStale time.Duration

	// Node name is the name we use to advertise. Defaults to hostname.
	//
	// NodeName is exposed via /v1/agent/self from here and
	// used in lots of places like CLI commands. Treat this as an interface
	// that must be stable.
	//
	// hcl: node_name = string
	// flag: -node string
	NodeName string

	// AdvertiseAddrLAN is the address we use for advertising our Serf, and
	// Consul RPC IP. The address can be specified as an ip address or as a
	// go-sockaddr template which resolves to a single ip address. If not
	// specified, the bind address is used.
	//
	// hcl: advertise_addr = string
	AdvertiseAddrLAN *net.IPAddr

	// AdvertiseAddrWAN is the address we use for advertising our Serf, and
	// Consul RPC IP. The address can be specified as an ip address or as a
	// go-sockaddr template which resolves to a single ip address. If not
	// specified, the bind address is used.
	//
	// hcl: advertise_addr_wan = string
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

	// BootstrapExpect tries to automatically bootstrap the Consul cluster, by
	// having servers wait to bootstrap until enough servers join, and then
	// performing the bootstrap process automatically. They will disable their
	// automatic bootstrap process if they detect any servers that are part of
	// an existing cluster, so it's safe to leave this set to a non-zero value.
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

	// CheckUpdateInterval controls the interval on which the output of a health check
	// is updated if there is no change to the state. For example, a check in a steady
	// state may run every 5 second generating a unique output (timestamp, etc), forcing
	// constant writes. This allows Consul to defer the write for some period of time,
	// reducing the write pressure when the state is steady.
	//
	// See also: DiscardCheckOutput
	//
	// hcl: check_update_interval = "duration"
	CheckUpdateInterval time.Duration

	// Checks contains the provided check definitions.
	//
	// hcl: checks = [
	//   {
	//     id = string
	//     name = string
	//     notes = string
	//     service_id = string
	//     token = string
	//     status = string
	//     script = string
	//     args = string
	//     http = string
	//     header = map[string][]string
	//     method = string
	//     tcp = string
	//     interval = string
	//     docker_container_id = string
	//     shell = string
	//     tls_skip_verify = (true|false)
	//     timeout = "duration"
	//     ttl = "duration"
	//     deregister_critical_service_after = "duration"
	//   },
	//   ...
	// ]
	Checks []*structs.CheckDefinition

	// ClientAddrs contains the list of ip addresses the DNS, HTTP and HTTPS
	// endpoints will bind to if the endpoints are enabled (ports > 0) and the
	// addresses are not overwritten.
	//
	// The ip addresses must be provided as a space separated list of ip
	// addresses and go-sockaddr templates.
	//
	// Client addresses cannot contain UNIX socket addresses since a socket
	// cannot be shared across multiple endpoints (no ports). To use UNIX
	// sockets configure it in 'addresses'.
	//
	// hcl: client_addr = string
	// flag: -client string
	ClientAddrs []*net.IPAddr

	// ConnectEnabled opts the agent into connect. It should be set on all clients
	// and servers in a cluster for correct connect operation.
	ConnectEnabled bool

	// ConnectProxyBindMinPort is the inclusive start of the range of ports
	// allocated to the agent for starting proxy listeners on where no explicit
	// port is specified.
	ConnectProxyBindMinPort int

	// ConnectProxyBindMaxPort is the inclusive end of the range of ports
	// allocated to the agent for starting proxy listeners on where no explicit
	// port is specified.
	ConnectProxyBindMaxPort int

	// ConnectProxyAllowManagedRoot is true if Consul can execute managed
	// proxies when running as root (EUID == 0).
	ConnectProxyAllowManagedRoot bool

	// ConnectProxyAllowManagedAPIRegistration enables managed proxy registration
	// via the agent HTTP API. If this is false, only file configurations
	// can be used.
	ConnectProxyAllowManagedAPIRegistration bool

	// ConnectProxyDefaultExecMode is used where a registration doesn't include an
	// exec_mode. Defaults to daemon.
	ConnectProxyDefaultExecMode string

	// ConnectProxyDefaultDaemonCommand is used to start proxy in exec_mode =
	// daemon if not specified at registration time.
	ConnectProxyDefaultDaemonCommand []string

	// ConnectProxyDefaultScriptCommand is used to start proxy in exec_mode =
	// script if not specified at registration time.
	ConnectProxyDefaultScriptCommand []string

	// ConnectProxyDefaultConfig is merged with any config specified at
	// registration time to allow global control of defaults.
	ConnectProxyDefaultConfig map[string]interface{}

	// ConnectCAProvider is the type of CA provider to use with Connect.
	ConnectCAProvider string

	// ConnectCAConfig is the config to use for the CA provider.
	ConnectCAConfig map[string]interface{}

	// ConnectTestDisableManagedProxies is not exposed to public config but us
	// used by TestAgent to prevent self-executing the test binary in the
	// background if a managed proxy is created for a test. The only place we
	// actually want to test processes really being spun up and managed is in
	// `agent/proxy` and it does it at a lower level. Note that this still allows
	// registering managed proxies via API and other methods, and still creates
	// all the agent state for them, just doesn't actually start external
	// processes up.
	ConnectTestDisableManagedProxies bool

	// DNSAddrs contains the list of TCP and UDP addresses the DNS server will
	// bind to. If the DNS endpoint is disabled (ports.dns <= 0) the list is
	// empty.
	//
	// The ip addresses are taken from 'addresses.dns' which should contain a
	// space separated list of ip addresses and/or go-sockaddr templates.
	//
	// If 'addresses.dns' was not provided the 'client_addr' addresses are
	// used.
	//
	// The DNS server cannot be bound to UNIX sockets.
	//
	// hcl: client_addr = string addresses { dns = string } ports { dns = int }
	DNSAddrs []net.Addr

	// DNSPort is the port the DNS server listens on. The default is 8600.
	// Setting this to a value <= 0 disables the endpoint.
	//
	// hcl: ports { dns = int }
	// flags: -dns-port int
	DNSPort int

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

	// DisableAnonymousSignature is used to turn off the anonymous signature
	// send with the update check. This is used to deduplicate messages.
	//
	// hcl: disable_anonymous_signature = (true|false)
	DisableAnonymousSignature bool

	// DisableCoordinates controls features related to network coordinates.
	//
	// hcl: disable_coordinates = (true|false)
	DisableCoordinates bool

	// DisableHostNodeID will prevent Consul from using information from the
	// host to generate a node ID, and will cause Consul to generate a
	// random ID instead.
	//
	// hcl: disable_host_node_id = (true|false)
	// flag: -disable-host-node-id
	DisableHostNodeID bool

	// DisableHTTPUnprintableCharFilter will bypass the filter preventing HTTP
	// URLs from containing unprintable chars. This filter was added in 1.0.3 as a
	// response to a vulnerability report. Disabling this is never recommended in
	// general however some users who have keys written in older versions of
	// Consul may use this to temporarily disable the filter such that they can
	// delete those keys again! We do not recommend leaving it disabled long term.
	//
	// hcl: disable_http_unprintable_char_filter
	DisableHTTPUnprintableCharFilter bool

	// DisableKeyringFile disables writing the keyring to a file.
	//
	// hcl: disable_keyring_file = (true|false)
	// flag: -disable-keyring-file
	DisableKeyringFile bool

	// DisableRemoteExec is used to turn off the remote execution
	// feature. This is for security to prevent unknown scripts from running.
	//
	// hcl: disable_remote_exec = (true|false)
	DisableRemoteExec bool

	// DisableUpdateCheck is used to turn off the automatic update and
	// security bulletin checking.
	//
	// hcl: disable_update_check = (true|false)
	DisableUpdateCheck bool

	// DiscardCheckOutput is used to turn off storing and comparing the
	// output of health checks. This reduces the write rate on the server
	// for checks with highly volatile output. (reloadable)
	//
	// See also: CheckUpdateInterval
	//
	// hcl: discard_check_output = (true|false)
	DiscardCheckOutput bool

	// EnableACLReplication is used to turn on ACL replication when using
	// /v1/agent/token/acl_replication_token to introduce the token, instead
	// of setting acl_replication_token in the config. Setting the token via
	// config will also set this to true for backward compatibility.
	//
	// hcl: enable_acl_replication = (true|false)
	// todo(fs): rename to ACLEnableReplication
	EnableACLReplication bool

	// EnableAgentTLSForChecks is used to apply the agent's TLS settings in
	// order to configure the HTTP client used for health checks. Enabling
	// this allows HTTP checks to present a client certificate and verify
	// the server using the same TLS configuration as the agent (CA, cert,
	// and key).
	EnableAgentTLSForChecks bool

	// EnableDebug is used to enable various debugging features.
	//
	// hcl: enable_debug = (true|false)
	EnableDebug bool

	// EnableScriptChecks controls whether health checks which execute
	// scripts are enabled. This includes regular script checks and Docker
	// checks.
	//
	// hcl: enable_script_checks = (true|false)
	// flag: -enable-script-checks
	EnableScriptChecks bool

	// EnableSyslog is used to also tee all the logs over to syslog. Only supported
	// on linux and OSX. Other platforms will generate an error.
	//
	// hcl: enable_syslog = (true|false)
	// flag: -syslog
	EnableSyslog bool

	// EnableUI enables the statically-compiled assets for the Consul web UI and
	// serves them at the default /ui/ endpoint automatically.
	//
	// hcl: enable_ui = (true|false)
	// flag: -ui
	EnableUI bool

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

	// HTTPAddrs contains the list of TCP addresses and UNIX sockets the HTTP
	// server will bind to. If the HTTP endpoint is disabled (ports.http <= 0)
	// the list is empty.
	//
	// The addresses are taken from 'addresses.http' which should contain a
	// space separated list of ip addresses, UNIX socket paths and/or
	// go-sockaddr templates. UNIX socket paths must be written as
	// 'unix://<full path>', e.g. 'unix:///var/run/consul-http.sock'.
	//
	// If 'addresses.http' was not provided the 'client_addr' addresses are
	// used.
	//
	// hcl: client_addr = string addresses { http = string } ports { http = int }
	HTTPAddrs []net.Addr

	// HTTPPort is the port the HTTP server listens on. The default is 8500.
	// Setting this to a value <= 0 disables the endpoint.
	//
	// hcl: ports { http = int }
	// flags: -http-port int
	HTTPPort int

	// HTTPSAddrs contains the list of TCP addresses and UNIX sockets the HTTPS
	// server will bind to. If the HTTPS endpoint is disabled (ports.https <=
	// 0) the list is empty.
	//
	// The addresses are taken from 'addresses.https' which should contain a
	// space separated list of ip addresses, UNIX socket paths and/or
	// go-sockaddr templates. UNIX socket paths must be written as
	// 'unix://<full path>', e.g. 'unix:///var/run/consul-https.sock'.
	//
	// If 'addresses.https' was not provided the 'client_addr' addresses are
	// used.
	//
	// hcl: client_addr = string addresses { https = string } ports { https = int }
	HTTPSAddrs []net.Addr

	// HTTPSPort is the port the HTTP server listens on. The default is -1.
	// Setting this to a value <= 0 disables the endpoint.
	//
	// hcl: ports { https = int }
	HTTPSPort int

	// KeyFile is used to provide a TLS key that is used for serving TLS
	// connections. Must be provided to serve TLS connections.
	//
	// hcl: key_file = string
	KeyFile string

	// LeaveDrainTime is used to wait after a server has left the LAN Serf
	// pool for RPCs to drain and new requests to be sent to other servers.
	//
	// hcl: performance { leave_drain_time = "duration" }
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

	// LogFile is the path to the file where the logs get written to. Defaults to empty string.
	//
	// hcl: log_file = string
	// flags: -log-file string
	LogFile string

	// LogRotateDuration is the time configured to rotate logs based on time
	//
	// hcl: log_rotate_duration = string
	// flags: -log-rotate-duration string
	LogRotateDuration time.Duration

	// LogRotateBytes is the time configured to rotate logs based on bytes written
	//
	// hcl: log_rotate_bytes = int
	// flags: -log-rotate-bytes int
	LogRotateBytes int

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

	// PidFile is the file to store our PID in.
	//
	// hcl: pid_file = string
	PidFile string

	// RPCAdvertiseAddr is the TCP address Consul advertises for its RPC endpoint.
	// By default this is the bind address on the default RPC Server port. If the
	// advertise address is specified then it is used.
	//
	// hcl: bind_addr = string advertise_addr = string ports { server = int }
	RPCAdvertiseAddr *net.TCPAddr

	// RPCBindAddr is the TCP address Consul will bind to for its RPC endpoint.
	// By default this is the bind address on the default RPC Server port.
	//
	// hcl: bind_addr = string ports { server = int }
	RPCBindAddr *net.TCPAddr

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	//
	// hcl: performance { rpc_hold_timeout = "duration" }
	RPCHoldTimeout time.Duration

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

	// RaftSnapshotThreshold sets the minimum threshold of raft commits after which
	// a snapshot is created. Defaults to 8192
	//
	// hcl: raft_snapshot_threshold = int
	RaftSnapshotThreshold int

	// RaftSnapshotInterval sets the interval to use when checking whether to create
	// a new snapshot. Defaults to 5 seconds.
	// hcl: raft_snapshot_threshold = int
	RaftSnapshotInterval time.Duration

	// ReconnectTimeoutLAN specifies the amount of time to wait to reconnect with
	// another agent before deciding it's permanently gone. This can be used to
	// control the time it takes to reap failed nodes from the cluster.
	//
	// hcl: reconnect_timeout = "duration"
	ReconnectTimeoutLAN time.Duration

	// ReconnectTimeoutWAN specifies the amount of time to wait to reconnect with
	// another agent before deciding it's permanently gone. This can be used to
	// control the time it takes to reap failed nodes from the cluster.
	//
	// hcl: reconnect_timeout = "duration"
	ReconnectTimeoutWAN time.Duration

	// RejoinAfterLeave controls our interaction with the cluster after leave.
	// When set to false (default), a leave causes Consul to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	//
	// hcl: rejoin_after_leave = (true|false)
	// flag: -rejoin
	RejoinAfterLeave bool

	// RetryJoinIntervalLAN specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	//
	// hcl: retry_join = "duration"
	RetryJoinIntervalLAN time.Duration

	// RetryJoinIntervalWAN specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	//
	// hcl: retry_join_wan = "duration"
	RetryJoinIntervalWAN time.Duration

	// RetryJoinLAN is a list of addresses and/or go-discover expressions to
	// join with retry enabled. See
	// https://www.consul.io/docs/agent/options.html#cloud-auto-joining for
	// details.
	//
	// hcl: retry_join = []string
	// flag: -retry-join string -retry-join string
	RetryJoinLAN []string

	// RetryJoinMaxAttemptsLAN specifies the maximum number of times to retry
	// joining a host on startup. This is useful for cases where we know the
	// node will be online eventually.
	//
	// hcl: retry_max = int
	// flag: -retry-max int
	RetryJoinMaxAttemptsLAN int

	// RetryJoinMaxAttemptsWAN specifies the maximum number of times to retry
	// joining a host on startup. This is useful for cases where we know the
	// node will be online eventually.
	//
	// hcl: retry_max_wan = int
	// flag: -retry-max-wan int
	RetryJoinMaxAttemptsWAN int

	// RetryJoinWAN is a list of addresses and/or go-discover expressions to
	// join -wan with retry enabled. See
	// https://www.consul.io/docs/agent/options.html#cloud-auto-joining for
	// details.
	//
	// hcl: retry_join_wan = []string
	// flag: -retry-join-wan string -retry-join-wan string
	RetryJoinWAN []string

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

	// SerfAdvertiseAddrLAN is the TCP address which is used for advertising
	// the LAN Gossip pool for both client and server. The address is the
	// combination of AdvertiseAddrLAN and the SerfPortLAN. If the advertise
	// address is not given the bind address is used.
	//
	// hcl: bind_addr = string advertise_addr = string ports { serf_lan = int }
	SerfAdvertiseAddrLAN *net.TCPAddr

	// SerfAdvertiseAddrWAN is the TCP address which is used for advertising
	// the WAN Gossip pool on the server only. The address is the combination
	// of AdvertiseAddrWAN and the SerfPortWAN. If the advertise address is not
	// given the bind address is used.
	//
	// hcl: bind_addr = string advertise_addr_wan = string ports { serf_wan = int }
	SerfAdvertiseAddrWAN *net.TCPAddr

	// SerfBindAddrLAN is the address to bind the Serf LAN TCP and UDP
	// listeners to. The ip address is either the default bind address or the
	// 'serf_lan' address which can be either an ip address or a go-sockaddr
	// template which resolves to a single ip address.
	//
	// hcl: bind_addr = string serf_lan = string ports { serf_lan = int }
	// flag: -serf-lan string
	SerfBindAddrLAN *net.TCPAddr

	// SerfBindAddrWAN is the address to bind the Serf WAN TCP and UDP
	// listeners to. The ip address is either the default bind address or the
	// 'serf_wan' address which can be either an ip address or a go-sockaddr
	// template which resolves to a single ip address.
	//
	// hcl: bind_addr = string serf_wan = string ports { serf_wan = int }
	// flag: -serf-wan string
	SerfBindAddrWAN *net.TCPAddr

	// SerfPortLAN is the port used for the LAN Gossip pool for both client and server.
	// The default is 8301.
	//
	// hcl: ports { serf_lan = int }
	SerfPortLAN int

	// SerfPortWAN is the port used for the WAN Gossip pool for the server only.
	// The default is 8302.
	//
	// hcl: ports { serf_wan = int }
	SerfPortWAN int

	// GossipLANGossipInterval is the interval between sending messages that need
	// to be gossiped that haven't been able to piggyback on probing messages.
	// If this is set to zero, non-piggyback gossip is disabled. By lowering
	// this value (more frequent) gossip messages are propagated across
	// the cluster more quickly at the expense of increased bandwidth. This
	// configuration only applies to LAN gossip communications
	//
	// The default is: 200ms
	//
	// hcl: gossip_lan { gossip_interval = duration}
	GossipLANGossipInterval time.Duration

	// GossipLANGossipNodes is the number of random nodes to send gossip messages to
	// per GossipInterval. Increasing this number causes the gossip messages to
	// propagate across the cluster more quickly at the expense of increased
	// bandwidth. This configuration only applies to LAN gossip communications
	//
	// The default is: 3
	//
	// hcl: gossip_lan { gossip_nodes = int }
	GossipLANGossipNodes int

	// GossipLANProbeInterval is the interval between random node probes. Setting
	// this lower (more frequent) will cause the memberlist cluster to detect
	// failed nodes more quickly at the expense of increased bandwidth usage.
	// This configuration only applies to LAN gossip communications
	//
	// The default is: 1s
	//
	// hcl: gossip_lan { probe_interval = duration }
	GossipLANProbeInterval time.Duration

	// GossipLANProbeTimeout is the timeout to wait for an ack from a probed node
	// before assuming it is unhealthy. This should be set to 99-percentile
	// of RTT (round-trip time) on your network. This configuration
	// only applies to the LAN gossip communications
	//
	// The default is: 500ms
	//
	// hcl: gossip_lan { probe_timeout = duration }
	GossipLANProbeTimeout time.Duration

	// GossipLANSuspicionMult is the multiplier for determining the time an
	// inaccessible node is considered suspect before declaring it dead. This
	// configuration only applies to LAN gossip communications
	//
	// The actual timeout is calculated using the formula:
	//
	//   SuspicionTimeout = SuspicionMult * log(N+1) * ProbeInterval
	//
	// This allows the timeout to scale properly with expected propagation
	// delay with a larger cluster size. The higher the multiplier, the longer
	// an inaccessible node is considered part of the cluster before declaring
	// it dead, giving that suspect node more time to refute if it is indeed
	// still alive.
	//
	// The default is: 4
	//
	// hcl: gossip_lan { suspicion_mult = int }
	GossipLANSuspicionMult int

	// GossipLANRetransmitMult is the multiplier for the number of retransmissions
	// that are attempted for messages broadcasted over gossip. This
	// configuration only applies to LAN gossip communications. The actual
	// count of retransmissions is calculated using the formula:
	//
	//   Retransmits = RetransmitMult * log(N+1)
	//
	// This allows the retransmits to scale properly with cluster size. The
	// higher the multiplier, the more likely a failed broadcast is to converge
	// at the expense of increased bandwidth.
	//
	// The default is: 4
	//
	// hcl: gossip_lan { retransmit_mult = int }
	GossipLANRetransmitMult int

	// GossipWANGossipInterval  is the interval between sending messages that need
	// to be gossiped that haven't been able to piggyback on probing messages.
	// If this is set to zero, non-piggyback gossip is disabled. By lowering
	// this value (more frequent) gossip messages are propagated across
	// the cluster more quickly at the expense of increased bandwidth. This
	// configuration only applies to WAN gossip communications
	//
	// The default is: 200ms
	//
	// hcl: gossip_wan { gossip_interval = duration}
	GossipWANGossipInterval time.Duration

	// GossipWANGossipNodes is the number of random nodes to send gossip messages to
	// per GossipInterval. Increasing this number causes the gossip messages to
	// propagate across the cluster more quickly at the expense of increased
	// bandwidth. This configuration only applies to WAN gossip communications
	//
	// The default is: 3
	//
	// hcl: gossip_wan { gossip_nodes = int }
	GossipWANGossipNodes int

	// GossipWANProbeInterval is the interval between random node probes. Setting
	// this lower (more frequent) will cause the memberlist cluster to detect
	// failed nodes more quickly at the expense of increased bandwidth usage.
	// This configuration only applies to WAN gossip communications
	//
	// The default is: 1s
	//
	// hcl: gossip_wan { probe_interval = duration }
	GossipWANProbeInterval time.Duration

	// GossipWANProbeTimeout is the timeout to wait for an ack from a probed node
	// before assuming it is unhealthy. This should be set to 99-percentile
	// of RTT (round-trip time) on your network. This configuration
	// only applies to the WAN gossip communications
	//
	// The default is: 500ms
	//
	// hcl: gossip_wan { probe_timeout = duration }
	GossipWANProbeTimeout time.Duration

	// GossipWANSuspicionMult is the multiplier for determining the time an
	// inaccessible node is considered suspect before declaring it dead. This
	// configuration only applies to WAN gossip communications
	//
	// The actual timeout is calculated using the formula:
	//
	//   SuspicionTimeout = SuspicionMult * log(N+1) * ProbeInterval
	//
	// This allows the timeout to scale properly with expected propagation
	// delay with a larger cluster size. The higher the multiplier, the longer
	// an inaccessible node is considered part of the cluster before declaring
	// it dead, giving that suspect node more time to refute if it is indeed
	// still alive.
	//
	// The default is: 4
	//
	// hcl: gossip_wan { suspicion_mult = int }
	GossipWANSuspicionMult int

	// GossipWANRetransmitMult is the multiplier for the number of retransmissions
	// that are attempted for messages broadcasted over gossip. This
	// configuration only applies to WAN gossip communications. The actual
	// count of retransmissions is calculated using the formula:
	//
	//   Retransmits = RetransmitMult * log(N+1)
	//
	// This allows the retransmits to scale properly with cluster size. The
	// higher the multiplier, the more likely a failed broadcast is to converge
	// at the expense of increased bandwidth.
	//
	// The default is: 4
	//
	// hcl: gossip_wan { retransmit_mult = int }
	GossipWANRetransmitMult int

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

	// ServerPort is the port the RPC server will bind to.
	// The default is 8300.
	//
	// hcl: ports { server = int }
	ServerPort int

	// Services contains the provided service definitions:
	//
	// hcl: services = [
	//   {
	//     id = string
	//     name = string
	//     tags = []string
	//     address = string
	//     check = { check definition }
	//     checks = [ { check definition}, ... ]
	//     token = string
	//     enable_tag_override = (true|false)
	//   },
	//   ...
	// ]
	Services []*structs.ServiceDefinition

	// Minimum Session TTL.
	//
	// hcl: session_ttl_min = "duration"
	SessionTTLMin time.Duration

	// SkipLeaveOnInt controls if Serf skips a graceful leave when
	// receiving the INT signal. Defaults false on clients, true on
	// servers. (reloadable)
	//
	// hcl: skip_leave_on_interrupt = (true|false)
	SkipLeaveOnInt bool

	// StartJoinLAN is a list of addresses to attempt to join -wan when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	//
	// hcl: start_join = []string
	// flag: -join string -join string
	StartJoinAddrsLAN []string

	// StartJoinWAN is a list of addresses to attempt to join -wan when the
	// agent starts. If Serf is unable to communicate with any of these
	// addresses, then the agent will error and exit.
	//
	// hcl: start_join_wan = []string
	// flag: -join-wan string -join-wan string
	StartJoinAddrsWAN []string

	// SyslogFacility is used to control where the syslog messages go
	// By default, goes to LOCAL0
	//
	// hcl: syslog_facility = string
	SyslogFacility string

	// TLSCipherSuites is used to specify the list of supported ciphersuites.
	//
	// The values should be a list of the following values:
	//
	//   TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
	//   TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
	//   TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	//   TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
	//   TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
	//   TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
	//   TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256
	//   TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
	//   TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256
	//   TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
	//   TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
	//   TLS_RSA_WITH_AES_128_GCM_SHA256
	//   TLS_RSA_WITH_AES_256_GCM_SHA384
	//   TLS_RSA_WITH_AES_128_CBC_SHA256
	//   TLS_RSA_WITH_AES_128_CBC_SHA
	//   TLS_RSA_WITH_AES_256_CBC_SHA
	//   TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
	//   TLS_RSA_WITH_3DES_EDE_CBC_SHA
	//   TLS_RSA_WITH_RC4_128_SHA
	//   TLS_ECDHE_RSA_WITH_RC4_128_SHA
	//   TLS_ECDHE_ECDSA_WITH_RC4_128_SHA
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

	// UIDir is the directory containing the Web UI resources.
	// If provided, the UI endpoints will be enabled.
	//
	// hcl: ui_dir = string
	// flag: -ui-dir string
	UIDir string

	// UnixSocketGroup contains the group of the file permissions when
	// Consul binds to UNIX sockets.
	//
	// hcl: unix_sockets { group = string }
	UnixSocketGroup string

	// UnixSocketMode contains the mode of the file permissions when
	// Consul binds to UNIX sockets.
	//
	// hcl: unix_sockets { mode = string }
	UnixSocketMode string

	// UnixSocketUser contains the user of the file permissions when
	// Consul binds to UNIX sockets.
	//
	// hcl: unix_sockets { user = string }
	UnixSocketUser string

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

	// Watches are used to monitor various endpoints and to invoke a
	// handler to act appropriately. These are managed entirely in the
	// agent layer using the standard APIs.
	//
	// See https://www.consul.io/docs/agent/watches.html for details.
	//
	// hcl: watches = [
	//   { type=string ... },
	//   { type=string ... },
	//   ...
	// ]
	//
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

func (c *RuntimeConfig) apiAddresses(maxPerType int) (unixAddrs, httpAddrs, httpsAddrs []string) {
	if len(c.HTTPSAddrs) > 0 {
		for i, addr := range c.HTTPSAddrs {
			if maxPerType < 1 || i < maxPerType {
				httpsAddrs = append(httpsAddrs, addr.String())
			} else {
				break
			}
		}
	}
	if len(c.HTTPAddrs) > 0 {
		unix_count := 0
		http_count := 0
		for _, addr := range c.HTTPAddrs {
			switch addr.(type) {
			case *net.UnixAddr:
				if maxPerType < 1 || unix_count < maxPerType {
					unixAddrs = append(unixAddrs, addr.String())
					unix_count += 1
				}
			default:
				if maxPerType < 1 || http_count < maxPerType {
					httpAddrs = append(httpAddrs, addr.String())
					http_count += 1
				}
			}
		}
	}

	return
}

func (c *RuntimeConfig) ClientAddress() (unixAddr, httpAddr, httpsAddr string) {
	unixAddrs, httpAddrs, httpsAddrs := c.apiAddresses(0)

	if len(unixAddrs) > 0 {
		unixAddr = "unix://" + unixAddrs[0]
	}

	http_any := ""
	if len(httpAddrs) > 0 {
		for _, addr := range httpAddrs {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				continue
			}

			if host == "0.0.0.0" || host == "::" {
				if http_any == "" {
					if host == "0.0.0.0" {
						http_any = net.JoinHostPort("127.0.0.1", port)
					} else {
						http_any = net.JoinHostPort("::1", port)
					}
				}
				continue
			}

			httpAddr = addr
			break
		}

		if httpAddr == "" && http_any != "" {
			httpAddr = http_any
		}
	}

	https_any := ""
	if len(httpsAddrs) > 0 {
		for _, addr := range httpsAddrs {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				continue
			}

			if host == "0.0.0.0" || host == "::" {
				if https_any == "" {
					if host == "0.0.0.0" {
						https_any = net.JoinHostPort("127.0.0.1", port)
					} else {
						https_any = net.JoinHostPort("::1", port)
					}
				}
				continue
			}

			httpsAddr = addr
			break
		}

		if httpsAddr == "" && https_any != "" {
			httpsAddr = https_any
		}
	}

	return
}

func (c *RuntimeConfig) APIConfig(includeClientCerts bool) (*api.Config, error) {
	cfg := &api.Config{
		Datacenter: c.Datacenter,
		TLSConfig:  api.TLSConfig{InsecureSkipVerify: !c.VerifyOutgoing},
	}

	unixAddr, httpAddr, httpsAddr := c.ClientAddress()

	if httpsAddr != "" {
		cfg.Address = httpsAddr
		cfg.Scheme = "https"
		cfg.TLSConfig.CAFile = c.CAFile
		cfg.TLSConfig.CAPath = c.CAPath
		cfg.TLSConfig.Address = httpsAddr
		if includeClientCerts {
			cfg.TLSConfig.CertFile = c.CertFile
			cfg.TLSConfig.KeyFile = c.KeyFile
		}
	} else if httpAddr != "" {
		cfg.Address = httpAddr
		cfg.Scheme = "http"
	} else if unixAddr != "" {
		cfg.Address = unixAddr
		// this should be ignored - however we are still talking http over a unix socket
		// so it makes sense to set it like this
		cfg.Scheme = "http"
	} else {
		return nil, fmt.Errorf("No suitable client address can be found")
	}

	return cfg, nil
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
