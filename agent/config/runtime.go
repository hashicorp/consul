// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul"
	consulrate "github.com/hashicorp/consul/agent/consul/rate"
	"github.com/hashicorp/consul/agent/dns"
	hcpconfig "github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

type RuntimeSOAConfig struct {
	Refresh uint32 // 3600 by default
	Retry   uint32 // 600
	Expire  uint32 // 86400
	Minttl  uint32 // 0,
}

// StaticRuntimeConfig specifies the subset of configuration the consul agent actually
// uses and that are not reloadable by configuration auto reload.
type StaticRuntimeConfig struct {
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
}

// RuntimeConfig specifies the configuration the consul agent actually
// uses. Is is derived from one or more Config structures which can come
// from files, flags and/or environment variables.
type RuntimeConfig struct {
	// non-user configurable values
	AEInterval time.Duration

	CheckDeregisterIntervalMin time.Duration
	CheckReapInterval          time.Duration
	SegmentLimit               int
	SegmentNameLimit           int
	SyncCoordinateRateTarget   float64
	SyncCoordinateIntervalMin  time.Duration
	Revision                   string
	Version                    string
	VersionPrerelease          string
	VersionMetadata            string
	BuildDate                  time.Time

	// consul config
	ConsulCoordinateUpdateMaxBatches int
	ConsulCoordinateUpdateBatchSize  int
	ConsulCoordinateUpdatePeriod     time.Duration
	ConsulRaftElectionTimeout        time.Duration
	ConsulRaftHeartbeatTimeout       time.Duration
	ConsulRaftLeaderLeaseTimeout     time.Duration
	ConsulServerHealthInterval       time.Duration

	// ACLsEnabled is used to determine whether ACLs should be enabled
	//
	// hcl: acl.enabled = boolean
	ACLsEnabled bool

	ACLTokens token.Config

	ACLResolverSettings consul.ACLResolverSettings

	// ACLEnableKeyListPolicy is used to opt-in to the "list" policy added to
	// KV ACLs in Consul 1.0.
	//
	// See https://www.consul.io/docs/guides/acl.html#list-policy-for-keys for
	// more details.
	//
	// hcl: acl.enable_key_list_policy = (true|false)
	ACLEnableKeyListPolicy bool

	// ACLInitialManagementToken is used to bootstrap the ACL system. It should be specified
	// on the servers in the PrimaryDatacenter. When the leader comes online, it ensures
	// that the initial management token is available. This provides the initial token.
	//
	// hcl: acl.tokens.initial_management = string
	ACLInitialManagementToken string

	// ACLtokenReplication is used to indicate that both tokens and policies
	// should be replicated instead of just policies
	//
	// hcl: acl.token_replication = boolean
	ACLTokenReplication bool

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

	// AutopilotMinQuorum sets the minimum number of servers required in a cluster
	// before autopilot can prune dead servers.
	//
	// hcl: autopilot { min_quorum = int }
	AutopilotMinQuorum uint

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

	// Cloud contains configuration for agents to connect to HCP.
	//
	// hcl: cloud { ... }
	Cloud hcpconfig.CloudConfig

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

	// DNSAltDomain can be set to support resolution on an additional
	// consul domain. Should end with a dot.
	// If left blank, only the primary domain will be used.
	//
	// hcl: alt_domain = string
	// flag: -alt-domain string
	DNSAltDomain string

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
	// hcl: dns_config { only_passing = (true|false) }
	DNSOnlyPassing bool

	// DNSRecursorStrategy controls the order in which DNS recursors are queried.
	// 'sequential' queries recursors in the order they are listed under `recursors`.
	// 'random' causes random selection of recursors which has the effect of
	// spreading the query load among all listed servers, rather than having
	// client agents try the first server in the list every time.
	//
	// hcl: dns_config { recursor_strategy = "(random|sequential)" }
	DNSRecursorStrategy dns.RecursorStrategy

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

	// DNSUseCache whether or not to use cache for dns queries
	//
	// hcl: dns_config { use_cache = (true|false) }
	DNSUseCache bool

	// DNSUseCache whether or not to use cache for dns queries
	//
	// hcl: dns_config { cache_max_age = "duration" }
	DNSCacheMaxAge time.Duration

	// HTTPUseCache whether or not to use cache for http queries. Defaults
	// to true.
	//
	// hcl: http_config { use_cache = (true|false) }
	HTTPUseCache bool

	// HTTPBlockEndpoints is a list of endpoint prefixes to block in the
	// HTTP API. Any requests to these will get a 403 response.
	//
	// hcl: http_config { block_endpoints = []string }
	HTTPBlockEndpoints []string

	// AllowWriteHTTPFrom restricts the agent write endpoints to the given
	// networks. Any request to a protected endpoint that is not mactched
	// by one of these networks will get a 403 response.
	// An empty slice means no restriction.
	//
	// hcl: http_config { allow_write_http_from = []string }
	AllowWriteHTTPFrom []*net.IPNet

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

	// Defines the maximum stale value for discovery path. Defaults to "0s".
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

	// Cache represent cache configuration of agent
	Cache cache.Options

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

	// Maximum size for the output of a healtcheck
	// hcl check_output_max_size int
	// flag: -check_output_max_size int
	CheckOutputMaxSize int

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
	//     disable_redirects = (true|false)
	//     tcp = string
	//     h2ping = string
	//     interval = string
	//     docker_container_id = string
	//     shell = string
	//     tls_skip_verify = (true|false)
	//     timeout = "duration"
	//     ttl = "duration"
	//     os_service = string
	//     success_before_passing = int
	//     failures_before_warning = int
	//     failures_before_critical = int
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

	// ConfigEntryBootstrap contains a list of ConfigEntries to ensure are created
	// If entries of the same Kind/Name exist already these will not update them.
	ConfigEntryBootstrap []structs.ConfigEntry

	// AutoEncryptTLS requires the client to acquire TLS certificates from
	// servers.
	AutoEncryptTLS bool

	// Additional DNS SAN entries that clients request during auto_encrypt
	// flow for their certificates.
	AutoEncryptDNSSAN []string

	// Additional IP SAN entries that clients request during auto_encrypt
	// flow for their certificates.
	AutoEncryptIPSAN []net.IP

	// AutoEncryptAllowTLS enables the server to respond to
	// AutoEncrypt.Sign requests.
	AutoEncryptAllowTLS bool

	// AutoConfig is a grouping of the configurations around the agent auto configuration
	// process including how servers can authorize requests.
	AutoConfig AutoConfig

	// ConnectEnabled opts the agent into connect. It should be set on all clients
	// and servers in a cluster for correct connect operation.
	ConnectEnabled bool

	// ConnectSidecarMinPort is the inclusive start of the range of ports
	// allocated to the agent for asigning to sidecar services where no port is
	// specified.
	ConnectSidecarMinPort int

	// ConnectSidecarMaxPort is the inclusive end of the range of ports
	// allocated to the agent for asigning to sidecar services where no port is
	// specified
	ConnectSidecarMaxPort int

	// ExposeMinPort is the inclusive start of the range of ports
	// allocated to the agent for exposing checks through a proxy
	ExposeMinPort int

	// ExposeMinPort is the inclusive start of the range of ports
	// allocated to the agent for exposing checks through a proxy
	ExposeMaxPort int

	// ConnectCAProvider is the type of CA provider to use with Connect.
	ConnectCAProvider string

	// ConnectCAConfig is the config to use for the CA provider.
	ConnectCAConfig map[string]interface{}

	// ConnectMeshGatewayWANFederationEnabled determines if wan federation of
	// datacenters should exclusively traverse mesh gateways.
	ConnectMeshGatewayWANFederationEnabled bool

	// ConnectTestCALeafRootChangeSpread is used to control how long the CA leaf
	// cache with spread CSRs over when a root change occurs. For now we don't
	// expose this in public config intentionally but could later with a rename.
	// We only set this from during tests to effectively make CA rotation tests
	// deterministic again.
	ConnectTestCALeafRootChangeSpread time.Duration

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

	// DNSSOA is the settings applied for DNS SOA
	// hcl: soa {}
	DNSSOA RuntimeSOAConfig

	// DataDir is the path to the directory where the local state is stored.
	//
	// hcl: data_dir = string
	// flag: -data-dir string
	DataDir string

	// DefaultQueryTime is the amount of time a blocking query will wait before
	// Consul will force a response. This value can be overridden by the 'wait'
	// query parameter.
	//
	// hcl: default_query_time = "duration"
	// flag: -default-query-time string
	DefaultQueryTime time.Duration

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

	// EnableAgentTLSForChecks is used to apply the agent's TLS settings in
	// order to configure the HTTP client used for health checks. Enabling
	// this allows HTTP checks to present a client certificate and verify
	// the server using the same TLS configuration as the agent (CA, cert,
	// and key).
	EnableAgentTLSForChecks bool

	// EnableCentralServiceConfig controls whether the agent should incorporate
	// centralized config such as service-defaults into local service registrations.
	//
	// hcl: enable_central_service_config = (true|false)
	EnableCentralServiceConfig bool

	// EnableDebug is used to enable various debugging features.
	//
	// hcl: enable_debug = (true|false)
	EnableDebug bool

	// EnableLocalScriptChecks controls whether health checks declared from the local
	// config file which execute scripts are enabled. This includes regular script
	// checks and Docker checks.
	//
	// hcl: (enable_script_checks|enable_local_script_checks) = (true|false)
	// flag: -enable-script-checks, -enable-local-script-checks
	EnableLocalScriptChecks bool

	// EnableRemoeScriptChecks controls whether health checks declared from the http API
	// which execute scripts are enabled. This includes regular script checks and Docker
	// checks.
	//
	// hcl: enable_script_checks = (true|false)
	// flag: -enable-script-checks
	EnableRemoteScriptChecks bool

	// EncryptKey contains the encryption key to use for the Serf communication.
	//
	// hcl: encrypt = string
	// flag: -encrypt string
	EncryptKey string

	// GRPCPort is the port the gRPC server listens on. It is disabled by default.
	//
	// hcl: ports { grpc = int }
	// flags: -grpc-port int
	GRPCPort int

	// GRPCTLSPort is the port the gRPC server listens on. It is disabled by default.
	//
	// hcl: ports { grpc_tls = int }
	// flags: -grpc-tls-port int
	GRPCTLSPort int

	// GRPCAddrs contains the list of TCP addresses and UNIX sockets the gRPC
	// server will bind to. If the gRPC endpoint is disabled (ports.grpc <= 0)
	// the list is empty.
	//
	// The addresses are taken from 'addresses.grpc' which should contain a
	// space separated list of ip addresses, UNIX socket paths and/or
	// go-sockaddr templates. UNIX socket paths must be written as
	// 'unix://<full path>', e.g. 'unix:///var/run/consul-grpc.sock'.
	//
	// If 'addresses.grpc' was not provided the 'client_addr' addresses are
	// used.
	//
	// hcl: client_addr = string addresses { grpc = string } ports { grpc = int }
	GRPCAddrs []net.Addr

	// GRPCTLSAddrs contains the list of TCP addresses and UNIX sockets the gRPC
	// server will bind to. If the gRPC endpoint is disabled (ports.grpc <= 0)
	// the list is empty.
	//
	// The addresses are taken from 'addresses.grpc_tls' which should contain a
	// space separated list of ip addresses, UNIX socket paths and/or
	// go-sockaddr templates. UNIX socket paths must be written as
	// 'unix://<full path>', e.g. 'unix:///var/run/consul-grpc.sock'.
	//
	// If 'addresses.grpc_tls' was not provided the 'client_addr' addresses are
	// used.
	//
	// hcl: client_addr = string addresses { grpc_tls = string } ports { grpc_tls = int }
	GRPCTLSAddrs []net.Addr

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

	// HTTPMaxConnsPerClient limits the number of concurrent TCP connections the
	// HTTP(S) server will accept from any single source IP address.
	//
	// hcl: limits{ http_max_conns_per_client = 200 }
	HTTPMaxConnsPerClient int

	// HTTPMaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line. It does not limit the
	// size of the request body.
	//
	// If zero, or negative, http.DefaultMaxHeaderBytes is used.
	HTTPMaxHeaderBytes int

	// HTTPSHandshakeTimeout is the time allowed for HTTPS client to complete the
	// TLS handshake and send first bytes of the request.
	//
	// hcl: limits{ https_handshake_timeout = "5s" }
	HTTPSHandshakeTimeout time.Duration

	// HTTPSPort is the port the HTTP server listens on. The default is -1.
	// Setting this to a value <= 0 disables the endpoint.
	//
	// hcl: ports { https = int }
	// flags: -https-port int
	HTTPSPort int

	// KVMaxValueSize controls the max allowed value size. If not set defaults
	// to raft's suggested max value size.
	//
	// hcl: limits { kv_max_value_size = uint64 }
	KVMaxValueSize uint64

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

	Locality *Locality

	// Logging configuration used to initialize agent logging.
	Logging logging.Config

	// MaxQueryTime is the maximum amount of time a blocking query can wait
	// before Consul will force a response. Consul applies jitter to the wait
	// time. The jittered time will be capped to MaxQueryTime.
	//
	// hcl: max_query_time = "duration"
	// flags: -max-query-time string
	MaxQueryTime time.Duration

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

	// ReadReplica is whether this server will act as a non-voting member
	// of the cluster to help provide read scalability. (Enterprise-only)
	//
	// hcl: non_voting_server = (true|false)
	// flag: -non-voting-server
	ReadReplica bool

	// PeeringEnabled enables cluster peering. This setting only applies for servers.
	// When disabled, all peering RPC endpoints will return errors,
	// peering requests from other clusters will receive errors, and any peerings already stored in this server's
	// state will be ignored.
	//
	// hcl: peering { enabled = (true|false) }
	PeeringEnabled bool

	// TestAllowPeerRegistrations controls whether CatalogRegister endpoints allow
	// registrations for objects with `PeerName`
	PeeringTestAllowPeerRegistrations bool

	// PidFile is the file to store our PID in.
	//
	// hcl: pid_file = string
	PidFile string

	// PrimaryDatacenter is the central datacenter that holds authoritative
	// ACL records, replicates intentions and holds the root CA for Connect.
	// This must be the same for the entire cluster. Off by default.
	//
	// hcl: primary_datacenter = string
	PrimaryDatacenter string

	// PrimaryGateways is a list of addresses and/or go-discover expressions to
	// discovery the mesh gateways in the primary datacenter. See
	// https://www.consul.io/docs/agent/config/cli-flags#cloud-auto-joining for
	// details.
	//
	// hcl: primary_gateways = []string
	// flag: -primary-gateway string -primary-gateway string
	PrimaryGateways []string

	// PrimaryGatewaysInterval specifies the amount of time to wait in between discovery
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	//
	// hcl: primary_gateways_interval = "duration"
	PrimaryGatewaysInterval time.Duration

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

	// RPCHandshakeTimeout is the timeout for reading the initial magic byte on a
	// new RPC connection. If this is set high it may allow unauthenticated users
	// to hold connections open arbitrarily long, even when mutual TLS is being
	// enforced. It may be set to 0 explicitly to disable the timeout but this
	// should never be used in production. Default is 5 seconds.
	//
	// hcl: limits { rpc_handshake_timeout = "duration" }
	RPCHandshakeTimeout time.Duration

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	//
	// hcl: performance { rpc_hold_timeout = "duration" }
	RPCHoldTimeout time.Duration

	// RPCClientTimeout limits how long a client is allowed to read from an RPC
	// connection. This is used to set an upper bound for requests to eventually
	// terminate so that RPC connections are not held indefinitely.
	// It may be set to 0 explicitly to disable the timeout but this should never
	// be used in production. Default is 60 seconds.
	//
	// Note: Blocking queries use MaxQueryTime and DefaultQueryTime to calculate
	// timeouts.
	//
	// hcl: limits { rpc_client_timeout = "duration" }
	RPCClientTimeout time.Duration

	// RPCRateLimit and RPCMaxBurst control how frequently RPC calls are allowed
	// to happen. In any large enough time interval, rate limiter limits the
	// rate to RPCRateLimit tokens per second, with a maximum burst size of
	// RPCMaxBurst events. As a special case, if RPCRateLimit == Inf (the infinite
	// rate), RPCMaxBurst is ignored.
	//
	// See https://en.wikipedia.org/wiki/Token_bucket for more about token
	// buckets.
	//
	// hcl: limits { rpc_rate = (float64|MaxFloat64) rpc_max_burst = int }
	RPCRateLimit rate.Limit
	RPCMaxBurst  int

	// RPCMaxConnsPerClient limits the number of concurrent TCP connections the
	// RPC server will accept from any single source IP address.
	//
	// hcl: limits { rpc_max_conns_per_client = 100 }
	RPCMaxConnsPerClient int

	// RPCProtocol is the Consul protocol version to use.
	//
	// hcl: protocol = int
	RPCProtocol int

	RPCConfig consul.RPCConfig

	// UseStreamingBackend enables streaming as a replacement for agent/cache
	// in the client agent for endpoints which support streaming.
	UseStreamingBackend bool

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

	// RaftTrailingLogs sets the number of log entries that will be left in the
	// log store after a snapshot. This must be large enough that a follower can
	// transfer and restore an entire snapshot of the state before this many new
	// entries have been appended. In vast majority of cases the default is plenty
	// but if there is a sustained high write throughput coupled with a huge
	// multi-gigabyte snapshot setting this higher may be necessary to allow
	// followers time to reload from snapshot without becoming unhealthy. If it's
	// too low then followers are unable to ever recover from a restart and will
	// enter a loop of constantly downloading full snapshots and never catching
	// up. If you need to change this you should reconsider your usage of Consul
	// as it is not designed to store multiple-gigabyte data sets with high write
	// throughput. Defaults to 10000.
	//
	// hcl: raft_trailing_logs = int
	RaftTrailingLogs int

	RaftLogStoreConfig consul.RaftLogStoreConfig

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

	// AdvertiseReconnectTimeout specifies the amount of time other agents should
	// wait for us to reconnect before deciding we are permanently gone. This
	// should only be set for client agents that are run in a stateless or
	// ephemeral manner in order to realize their deletion sooner than we
	// would otherwise.
	AdvertiseReconnectTimeout time.Duration

	// RejoinAfterLeave controls our interaction with the cluster after leave.
	// When set to false (default), a leave causes Consul to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	//
	// hcl: rejoin_after_leave = (true|false)
	// flag: -rejoin
	RejoinAfterLeave bool

	// RequestLimitsMode will disable or enable rate limiting.  If not disabled, it
	// enforces the action that will occur when RequestLimitsReadRate
	// or RequestLimitsWriteRate is exceeded.  The default value of "disabled" will
	// prevent any rate limiting from occuring.  A value of "enforce" will block
	// the request from processings by returning an error.  A value of
	// "permissive" will not block the request and will allow the request to
	// continue processing.
	//
	// hcl: limits { request_limits { mode = "permissive" } }
	RequestLimitsMode consulrate.Mode

	// RequestLimitsReadRate controls how frequently RPC, gRPC, and HTTP
	// queries are allowed to happen. In any large enough time interval, rate
	// limiter limits the rate to RequestLimitsReadRate tokens per second.
	//
	// See https://en.wikipedia.org/wiki/Token_bucket for more about token
	// buckets.
	//
	// hcl: limits { request_limits { read_rate = (float64|MaxFloat64) } }
	RequestLimitsReadRate rate.Limit

	// RequestLimitsWriteRate controls how frequently RPC, gRPC, and HTTP
	// writes are allowed to happen. In any large enough time interval, rate
	// limiter limits the rate to RequestLimitsWriteRate tokens per second.
	//
	// See https://en.wikipedia.org/wiki/Token_bucket for more about token
	// buckets.
	//
	// hcl: limits { request_limits { write_rate = (float64|MaxFloat64) } }
	RequestLimitsWriteRate rate.Limit

	// RetryJoinIntervalLAN specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	//
	// hcl: retry_interval = "duration"
	RetryJoinIntervalLAN time.Duration

	// RetryJoinIntervalWAN specifies the amount of time to wait in between join
	// attempts on agent start. The minimum allowed value is 1 second and
	// the default is 30s.
	//
	// hcl: retry_interval_wan = "duration"
	RetryJoinIntervalWAN time.Duration

	// RetryJoinLAN is a list of addresses and/or go-discover expressions to
	// join with retry enabled. See
	// https://www.consul.io/docs/agent/config/cli-flags#cloud-auto-joining for
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
	// https://www.consul.io/docs/agent/config/cli-flags#cloud-auto-joining for
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

	// SerfAllowedCIDRsLAN if set to a non-empty value, will restrict which networks
	// are allowed to connect to Serf on the LAN.
	// hcl: serf_lan_allowed_cidrs = []string
	// flag: serf-lan-allowed-cidrs string (can be specified multiple times)
	SerfAllowedCIDRsLAN []net.IPNet

	// SerfAllowedCIDRsWAN if set to a non-empty value, will restrict which networks
	// are allowed to connect to Serf on the WAN.
	// hcl: serf_wan_allowed_cidrs = []string
	// flag: serf-wan-allowed-cidrs string (can be specified multiple times)
	SerfAllowedCIDRsWAN []net.IPNet

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
	// The default is: 500ms
	//
	// hcl: gossip_wan { gossip_interval = duration}
	GossipWANGossipInterval time.Duration

	// GossipWANGossipNodes is the number of random nodes to send gossip messages to
	// per GossipInterval. Increasing this number causes the gossip messages to
	// propagate across the cluster more quickly at the expense of increased
	// bandwidth. This configuration only applies to WAN gossip communications
	//
	// The default is: 4
	//
	// hcl: gossip_wan { gossip_nodes = int }
	GossipWANGossipNodes int

	// GossipWANProbeInterval is the interval between random node probes. Setting
	// this lower (more frequent) will cause the memberlist cluster to detect
	// failed nodes more quickly at the expense of increased bandwidth usage.
	// This configuration only applies to WAN gossip communications
	//
	// The default is: 5s
	//
	// hcl: gossip_wan { probe_interval = duration }
	GossipWANProbeInterval time.Duration

	// GossipWANProbeTimeout is the timeout to wait for an ack from a probed node
	// before assuming it is unhealthy. This should be set to 99-percentile
	// of RTT (round-trip time) on your network. This configuration
	// only applies to the WAN gossip communications
	//
	// The default is: 3s
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
	// The default is: 6
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

	// ServerRejoinAgeMax is used to specify the duration of time a server
	// is allowed to be down/offline before a startup operation is refused.
	//
	// For example: if a server has been offline for 5 days, and this option
	// is configured to 3 days, then any subsequent startup operation will fail
	// and require an operator to manually intervene.
	//
	// The default is: 7 days
	//
	// hcl: server_rejoin_age_max = "duration"
	ServerRejoinAgeMax time.Duration

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

	// AutoReloadConfig indicate if the config will be
	// auto reloaded bases on config file modification
	// hcl: auto_reload_config = (true|false)
	AutoReloadConfig bool

	// TLS configures certificates, CA, cipher suites, and other TLS settings
	// on Consul's listeners (i.e. Internal multiplexed RPC, HTTPS and gRPC).
	//
	// hcl: tls { ... }
	TLS tlsutil.Config

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

	// TxnMaxReqLen configures the upper limit for the size (in bytes) of the
	// incoming request bodies for transactions to the /txn endpoint.
	//
	// hcl: limits { txn_max_req_len = uint64 }
	TxnMaxReqLen uint64

	// UIConfig holds various runtime options that control both the agent's
	// behavior while serving the UI (e.g. whether it's enabled, what path it's
	// mounted on) as well as options that enable or disable features within the
	// UI.
	//
	// NOTE: Never read from this field directly once the agent has started up
	// since the UI config is reloadable. The on in the agent's config field may
	// be out of date. Use the agent.getUIConfig() method to get the latest config
	// in a thread-safe way.
	//
	// hcl: ui_config { ... }
	UIConfig UIConfig

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

	StaticRuntimeConfig StaticRuntimeConfig

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

	// XDSUpdateRateLimit controls the maximum rate at which proxy config updates
	// will be delivered, across all connected xDS streams. This is used to stop
	// updates to "global" resources (e.g. wildcard intentions) from saturating
	// system resources at the expense of other work, such as raft and gossip,
	// which could cause general cluster instability.
	//
	// hcl: xds { update_max_per_second = (float64|MaxFloat64) }
	XDSUpdateRateLimit rate.Limit

	// AutoReloadConfigCoalesceInterval Coalesce Interval for auto reload config
	AutoReloadConfigCoalesceInterval time.Duration

	// LocalProxyConfigResyncInterval is not a user-configurable value and exists
	// here so that tests can use a smaller value.
	LocalProxyConfigResyncInterval time.Duration

	Reporting ReportingConfig

	// List of experiments to enable
	Experiments []string

	EnterpriseRuntimeConfig
}

type LicenseConfig struct {
	Enabled bool
}

type ReportingConfig struct {
	License LicenseConfig
}

type AutoConfig struct {
	Enabled         bool
	IntroToken      string
	IntroTokenFile  string
	ServerAddresses []string
	DNSSANs         []string
	IPSANs          []net.IP
	Authorizer      AutoConfigAuthorizer
}

type AutoConfigAuthorizer struct {
	Enabled    bool
	AuthMethod structs.ACLAuthMethod
	// AuthMethodConfig ssoauth.Config
	ClaimAssertions []string
	AllowReuse      bool
}

type UIConfig struct {
	Enabled                    bool
	Dir                        string
	ContentPath                string
	MetricsProvider            string
	MetricsProviderFiles       []string
	MetricsProviderOptionsJSON string
	MetricsProxy               UIMetricsProxy
	DashboardURLTemplates      map[string]string
	HCPEnabled                 bool
}

type UIMetricsProxy struct {
	BaseURL       string
	AddHeaders    []UIMetricsProxyAddHeader
	PathAllowlist []string
}

type UIMetricsProxyAddHeader struct {
	Name  string
	Value string
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

func (c *RuntimeConfig) ConnectCAConfiguration() (*structs.CAConfiguration, error) {
	ca := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"LeafCertTTL":         structs.DefaultLeafCertTTL,
			"IntermediateCertTTL": structs.DefaultIntermediateCertTTL,
			"RootCertTTL":         structs.DefaultRootCertTTL,
		},
	}

	// Allow config to specify cluster_id provided it's a valid UUID. This is
	// meant only for tests where a deterministic ID makes fixtures much simpler
	// to work with but since it's only read on initial cluster bootstrap it's not
	// that much of a liability in production. The worst a user could do is
	// configure logically separate clusters with same ID by mistake but we can
	// avoid documenting this is even an option.
	if clusterID, ok := c.ConnectCAConfig["cluster_id"]; ok {
		// If they tried to specify an ID but typoed it then don't ignore as they
		//  will then bootstrap with a new ID and have to throw away the whole cluster
		// and start again.

		// ensure the cluster_id value in the opaque config is a string
		cIDStr, ok := clusterID.(string)
		if !ok {
			return nil, fmt.Errorf("cluster_id was supplied but was not a string")
		}

		// ensure that the cluster_id string is a valid UUID
		_, err := uuid.ParseUUID(cIDStr)
		if err != nil {
			return nil, fmt.Errorf("cluster_id was supplied but was not a valid UUID")
		}

		// now that we know the cluster_id is okay we can set it in the CAConfiguration
		ca.ClusterID = cIDStr
	}

	if c.ConnectCAProvider != "" {
		ca.Provider = c.ConnectCAProvider
	}

	// Merge connect CA Config regardless of provider (since there are some
	// common config options valid to all like leaf TTL).
	for k, v := range c.ConnectCAConfig {
		ca.Config[k] = v
	}

	return ca, nil
}

func (c *RuntimeConfig) APIConfig(includeClientCerts bool) (*api.Config, error) {
	tls := c.TLS.HTTPS

	cfg := &api.Config{
		Datacenter: c.Datacenter,
		TLSConfig:  api.TLSConfig{InsecureSkipVerify: !tls.VerifyOutgoing},
	}

	unixAddr, httpAddr, httpsAddr := c.ClientAddress()

	if httpsAddr != "" {
		cfg.Address = httpsAddr
		cfg.Scheme = "https"
		cfg.TLSConfig.CAFile = tls.CAFile
		cfg.TLSConfig.CAPath = tls.CAPath
		if includeClientCerts {
			cfg.TLSConfig.CertFile = tls.CertFile
			cfg.TLSConfig.KeyFile = tls.KeyFile
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

func (c *RuntimeConfig) VersionWithMetadata() string {
	version := c.Version
	if c.VersionMetadata != "" {
		version += "+" + c.VersionMetadata
	}
	return version
}

// StructLocality converts the RuntimeConfig Locality to a struct Locality.
func (c *RuntimeConfig) StructLocality() *structs.Locality {
	if c.Locality == nil {
		return nil
	}
	return &structs.Locality{
		Region: stringVal(c.Locality.Region),
		Zone:   stringVal(c.Locality.Zone),
	}
}

// Sanitized returns a JSON/HCL compatible representation of the runtime
// configuration where all fields with potential secrets had their
// values replaced by 'hidden'. In addition, network addresses and
// time.Duration values are formatted to improve readability.
func (c *RuntimeConfig) Sanitized() map[string]interface{} {
	return sanitize("rt", reflect.ValueOf(c)).Interface().(map[string]interface{})
}

// IsCloudEnabled returns true if a cloud.resource_id is set and the server mode is enabled
func (c *RuntimeConfig) IsCloudEnabled() bool {
	if c == nil {
		return false
	}
	return c.ServerMode && c.Cloud.ResourceID != ""
}

// isSecret determines whether a field name represents a field which
// may contain a secret.
func isSecret(name string) bool {
	// special cases for AuthMethod locality and intro token file
	if name == "TokenLocality" || name == "IntroTokenFile" {
		return false
	}
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
		case *net.IPNet:
			return reflect.ValueOf(x.String())
		default:
			return v
		}

	// check before isNumber
	case isDuration(typ):
		x := v.Interface().(time.Duration)
		return reflect.ValueOf(x.String())

	case isTime(typ):
		x := v.Interface().(time.Time)
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
		ma := make([]interface{}, 0, v.Len())

		if name == "AddHeaders" {
			// must be UIConfig.MetricsProxy.AddHeaders
			for i := 0; i < v.Len(); i++ {
				addr := v.Index(i).Addr()
				hdr := addr.Interface().(*UIMetricsProxyAddHeader)
				hm := map[string]interface{}{
					"Name":  hdr.Name,
					"Value": "hidden",
				}
				ma = append(ma, hm)
			}
			return reflect.ValueOf(ma)
		}

		if strings.HasPrefix(name, "SerfAllowedCIDRs") {
			for i := 0; i < v.Len(); i++ {
				addr := v.Index(i).Addr()
				ip := addr.Interface().(*net.IPNet)
				ma = append(ma, ip.String())
			}
			return reflect.ValueOf(ma)
		}
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
func isTime(t reflect.Type) bool     { return t == reflect.TypeOf(time.Time{}) }
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

// ApplyDefaultQueryOptions returns a function which will set default values on
// the options based on the configuration. The RuntimeConfig must not be nil.
func ApplyDefaultQueryOptions(config *RuntimeConfig) func(options *structs.QueryOptions) {
	return func(options *structs.QueryOptions) {
		switch {
		case options.MaxQueryTime > config.MaxQueryTime:
			options.MaxQueryTime = config.MaxQueryTime
		case options.MaxQueryTime == 0:
			options.MaxQueryTime = config.DefaultQueryTime
		}
	}
}
