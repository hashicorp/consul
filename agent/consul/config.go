package consul

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/time/rate"
)

const (
	DefaultDC          = "dc1"
	DefaultRPCPort     = 8300
	DefaultLANSerfPort = 8301
	DefaultWANSerfPort = 8302

	// DefaultRaftMultiplier is used as a baseline Raft configuration that
	// will be reliable on a very basic server. See docs/install/performance.html
	// for information on how this value was obtained.
	DefaultRaftMultiplier uint = 5

	// MaxRaftMultiplier is a fairly arbitrary upper bound that limits the
	// amount of performance detuning that's possible.
	MaxRaftMultiplier uint = 10
)

var (
	DefaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: DefaultRPCPort}

	// ProtocolVersionMap is the mapping of Consul protocol versions
	// to Serf protocol versions. We mask the Serf protocols using
	// our own protocol version.
	protocolVersionMap map[uint8]uint8
)

func init() {
	protocolVersionMap = map[uint8]uint8{
		1: 4,
		2: 4,
		3: 4,
	}
}

// (Enterprise-only) NetworkSegment is the address and port configuration
// for a network segment.
type NetworkSegment struct {
	Name       string
	Bind       string
	Port       int
	Advertise  string
	RPCAddr    *net.TCPAddr
	SerfConfig *serf.Config
}

// Config is used to configure the server
type Config struct {
	// Bootstrap mode is used to bring up the first Consul server.
	// It is required so that it can elect a leader without any
	// other nodes being present
	Bootstrap bool

	// BootstrapExpect mode is used to automatically bring up a collection of
	// Consul servers. This can be used to automatically bring up a collection
	// of nodes.
	BootstrapExpect int

	// Datacenter is the datacenter this Consul server represents.
	Datacenter string

	// PrimaryDatacenter is the authoritative datacenter for features like ACLs
	// and Connect.
	PrimaryDatacenter string

	// DataDir is the directory to store our state in.
	DataDir string

	// DefaultQueryTime is the amount of time a blocking query will wait before
	// Consul will force a response. This value can be overridden by the 'wait'
	// query parameter.
	DefaultQueryTime time.Duration

	// MaxQueryTime is the maximum amount of time a blocking query can wait
	// before Consul will force a response. Consul applies jitter to the wait
	// time. The jittered time will be capped to MaxQueryTime.
	MaxQueryTime time.Duration

	// DevMode is used to enable a development server mode.
	DevMode bool

	// NodeID is a unique identifier for this node across space and time.
	NodeID types.NodeID

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// Domain is the DNS domain for the records. Defaults to "consul."
	Domain string

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// (Enterprise-only) NonVoter is used to prevent this server from being added
	// as a voting member of the Raft cluster.
	NonVoter bool

	// NotifyListen is called after the RPC listener has been configured.
	// RPCAdvertise will be set to the listener address if it hasn't been
	// configured at this point.
	NotifyListen func()

	// NotifyShutdown is called after Server is completely Shutdown.
	NotifyShutdown func()

	// RPCAddr is the RPC address used by Consul. This should be reachable
	// by the WAN and LAN
	RPCAddr *net.TCPAddr

	// RPCAdvertise is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:8300", but this address must be
	// reachable. If RPCAdvertise is nil then it will be set to the Listener
	// address after the listening socket is configured.
	RPCAdvertise *net.TCPAddr

	// RPCSrcAddr is the source address for outgoing RPC connections.
	RPCSrcAddr *net.TCPAddr

	// (Enterprise-only) The network segment this agent is part of.
	Segment string

	// (Enterprise-only) Segments is a list of network segments for a server to
	// bind on.
	Segments []NetworkSegment

	// SerfLANConfig is the configuration for the intra-dc serf
	SerfLANConfig *serf.Config

	// SerfWANConfig is the configuration for the cross-dc serf
	SerfWANConfig *serf.Config

	// SerfFloodInterval controls how often we attempt to flood local Serf
	// Consul servers into the global areas (WAN and user-defined areas in
	// Consul Enterprise).
	SerfFloodInterval time.Duration

	// ReconcileInterval controls how often we reconcile the strongly
	// consistent store with the Serf info. This is used to handle nodes
	// that are force removed, as well as intermittent unavailability during
	// leader election.
	ReconcileInterval time.Duration

	// LogLevel is the level of the logs to write. Defaults to "INFO".
	LogLevel string

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion uint8

	// VerifyIncoming is used to verify the authenticity of incoming connections.
	// This means that TCP requests are forbidden, only allowing for TLS. TLS connections
	// must match a provided certificate authority. This can be used to force client auth.
	VerifyIncoming bool

	// VerifyOutgoing is used to force verification of the authenticity of outgoing connections.
	// This means that TLS requests are used, and TCP requests are not made. TLS connections
	// must match a provided certificate authority.
	VerifyOutgoing bool

	// UseTLS is used to enable TLS for outgoing connections to other TLS-capable Consul
	// servers. This doesn't imply any verification, it only enables TLS if possible.
	UseTLS bool

	// VerifyServerHostname is used to enable hostname verification of servers. This
	// ensures that the certificate presented is valid for server.<datacenter>.<domain>.
	// This prevents a compromised client from being restarted as a server, and then
	// intercepting request traffic as well as being added as a raft peer. This should be
	// enabled by default with VerifyOutgoing, but for legacy reasons we cannot break
	// existing clients.
	VerifyServerHostname bool

	// CAFile is a path to a certificate authority file. This is used with VerifyIncoming
	// or VerifyOutgoing to verify the TLS connection.
	CAFile string

	// CAPath is a path to a directory of certificate authority files. This is used with
	// VerifyIncoming or VerifyOutgoing to verify the TLS connection.
	CAPath string

	// CertFile is used to provide a TLS certificate that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	CertFile string

	// KeyFile is used to provide a TLS key that is used for serving TLS connections.
	// Must be provided to serve TLS connections.
	KeyFile string

	// ServerName is used with the TLS certificate to ensure the name we
	// provide matches the certificate
	ServerName string

	// TLSMinVersion is used to set the minimum TLS version used for TLS connections.
	TLSMinVersion string

	// TLSCipherSuites is used to specify the list of supported ciphersuites.
	TLSCipherSuites []uint16

	// TLSPreferServerCipherSuites specifies whether to prefer the server's ciphersuite
	// over the client ciphersuites.
	TLSPreferServerCipherSuites bool

	// RejoinAfterLeave controls our interaction with Serf.
	// When set to false (default), a leave causes a Consul to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	RejoinAfterLeave bool

	// Build is a string that is gossiped around, and can be used to help
	// operators track which versions are actively deployed
	Build string

	// ACLEnabled is used to enable ACLs
	ACLsEnabled bool

	// ACLEnforceVersion8 is used to gate a set of ACL policy features that
	// are opt-in prior to Consul 0.8 and opt-out in Consul 0.8 and later.
	ACLEnforceVersion8 bool

	// ACLMasterToken is used to bootstrap the ACL system. It should be specified
	// on the servers in the ACLDatacenter. When the leader comes online, it ensures
	// that the Master token is available. This provides the initial token.
	ACLMasterToken string

	// ACLDatacenter provides the authoritative datacenter for ACL
	// tokens. If not provided, ACL verification is disabled.
	ACLDatacenter string

	// ACLTokenTTL controls the time-to-live of cached ACL tokens.
	// It can be set to zero to disable caching, but this adds
	// a substantial cost.
	ACLTokenTTL time.Duration

	// ACLPolicyTTL controls the time-to-live of cached ACL policies.
	// It can be set to zero to disable caching, but this adds
	// a substantial cost.
	ACLPolicyTTL time.Duration

	// ACLRoleTTL controls the time-to-live of cached ACL roles.
	// It can be set to zero to disable caching, but this adds
	// a substantial cost.
	ACLRoleTTL time.Duration

	// ACLDisabledTTL is the time between checking if ACLs should be
	// enabled. This
	ACLDisabledTTL time.Duration

	// ACLTokenReplication is used to enabled token replication.
	//
	// By default policy-only replication is enabled. When token
	// replication is off and the primary datacenter is not
	// yet upgraded to the new ACLs no replication will be performed
	ACLTokenReplication bool

	// ACLDefaultPolicy is used to control the ACL interaction when
	// there is no defined policy. This can be "allow" which means
	// ACLs are used to black-list, or "deny" which means ACLs are
	// white-lists.
	ACLDefaultPolicy string

	// ACLDownPolicy controls the behavior of ACLs if the ACLDatacenter
	// cannot be contacted. It can be either "deny" to deny all requests,
	// "extend-cache" or "async-cache" which ignores the ACLCacheInterval and
	// uses cached policies.
	// If a policy is not in the cache, it acts like deny.
	// "allow" can be used to allow all requests. This is not recommended.
	ACLDownPolicy string

	// ACLReplicationRate is the max number of replication rounds that can
	// be run per second. Note that either 1 or 2 RPCs are used during each replication
	// round
	ACLReplicationRate int

	// ACLReplicationBurst is how many replication RPCs can be bursted after a
	// period of idleness
	ACLReplicationBurst int

	// ACLReplicationApplyLimit is the max number of replication-related
	// apply operations that we allow during a one second period. This is
	// used to limit the amount of Raft bandwidth used for replication.
	ACLReplicationApplyLimit int

	// ACLEnableKeyListPolicy is used to gate enforcement of the new "list" policy that
	// protects listing keys by prefix. This behavior is opt-in
	// by default in Consul 1.0 and later.
	ACLEnableKeyListPolicy bool

	// TombstoneTTL is used to control how long KV tombstones are retained.
	// This provides a window of time where the X-Consul-Index is monotonic.
	// Outside this window, the index may not be monotonic. This is a result
	// of a few trade offs:
	// 1) The index is defined by the data view and not globally. This is a
	// performance optimization that prevents any write from incrementing the
	// index for all data views.
	// 2) Tombstones are not kept indefinitely, since otherwise storage required
	// is also monotonic. This prevents deletes from reducing the disk space
	// used.
	// In theory, neither of these are intrinsic limitations, however for the
	// purposes of building a practical system, they are reasonable trade offs.
	//
	// It is also possible to set this to an incredibly long time, thereby
	// simulating infinite retention. This is not recommended however.
	//
	TombstoneTTL time.Duration

	// TombstoneTTLGranularity is used to control how granular the timers are
	// for the Tombstone GC. This is used to batch the GC of many keys together
	// to reduce overhead. It is unlikely a user would ever need to tune this.
	TombstoneTTLGranularity time.Duration

	// Minimum Session TTL
	SessionTTLMin time.Duration

	// maxTokenExpirationDuration is the maximum difference allowed between
	// ACLToken CreateTime and ExpirationTime values if ExpirationTime is set
	// on a token.
	ACLTokenMaxExpirationTTL time.Duration

	// ACLTokenMinExpirationTTL is the minimum difference allowed between
	// ACLToken CreateTime and ExpirationTime values if ExpirationTime is set
	// on a token.
	ACLTokenMinExpirationTTL time.Duration

	// ServerUp callback can be used to trigger a notification that
	// a Consul server is now up and known about.
	ServerUp func()

	// Shutdown callback is used to trigger a full Consul shutdown
	Shutdown func()

	// UserEventHandler callback can be used to handle incoming
	// user events. This function should not block.
	UserEventHandler func(serf.UserEvent)

	// ConfigReplicationRate is the max number of replication rounds that can
	// be run per second. Note that either 1 or 2 RPCs are used during each replication
	// round
	ConfigReplicationRate int

	// ConfigReplicationBurst is how many replication rounds can be bursted after a
	// period of idleness
	ConfigReplicationBurst int

	// ConfigReplicationApply limit is the max number of replication-related
	// apply operations that we allow during a one second period. This is
	// used to limit the amount of Raft bandwidth used for replication.
	ConfigReplicationApplyLimit int

	// FederationStateReplicationRate is the max number of replication rounds that can
	// be run per second. Note that either 1 or 2 RPCs are used during each replication
	// round
	FederationStateReplicationRate int

	// FederationStateReplicationBurst is how many replication rounds can be bursted after a
	// period of idleness
	FederationStateReplicationBurst int

	// FederationStateReplicationApply limit is the max number of replication-related
	// apply operations that we allow during a one second period. This is
	// used to limit the amount of Raft bandwidth used for replication.
	FederationStateReplicationApplyLimit int

	// CoordinateUpdatePeriod controls how long a server batches coordinate
	// updates before applying them in a Raft transaction. A larger period
	// leads to fewer Raft transactions, but also the stored coordinates
	// being more stale.
	CoordinateUpdatePeriod time.Duration

	// CoordinateUpdateBatchSize controls the maximum number of updates a
	// server batches before applying them in a Raft transaction.
	CoordinateUpdateBatchSize int

	// CoordinateUpdateMaxBatches controls the maximum number of batches we
	// are willing to apply in one period. After this limit we will issue a
	// warning and discard the remaining updates.
	CoordinateUpdateMaxBatches int

	// CheckOutputMaxSize control the max size of output of checks
	CheckOutputMaxSize int

	// RPCHandshakeTimeout limits how long we will wait for the initial magic byte
	// on an RPC client connection. It also governs how long we will wait for a
	// TLS handshake when TLS is configured however the timout applies separately
	// for the initial magic byte and the TLS handshake and inner magic byte.
	RPCHandshakeTimeout time.Duration

	// RPCHoldTimeout is how long an RPC can be "held" before it is errored.
	// This is used to paper over a loss of leadership by instead holding RPCs,
	// so that the caller experiences a slow response rather than an error.
	// This period is meant to be long enough for a leader election to take
	// place, and a small jitter is applied to avoid a thundering herd.
	RPCHoldTimeout time.Duration

	// RPCRate and RPCMaxBurst control how frequently RPC calls are allowed
	// to happen. In any large enough time interval, rate limiter limits the
	// rate to RPCRate tokens per second, with a maximum burst size of
	// RPCMaxBurst events. As a special case, if RPCRate == Inf (the infinite
	// rate), RPCMaxBurst is ignored.
	//
	// See https://en.wikipedia.org/wiki/Token_bucket for more about token
	// buckets.
	RPCRate     rate.Limit
	RPCMaxBurst int

	// RPCMaxConnsPerClient is the limit of how many concurrent connections are
	// allowed from a single source IP.
	RPCMaxConnsPerClient int

	// LeaveDrainTime is used to wait after a server has left the LAN Serf
	// pool for RPCs to drain and new requests to be sent to other servers.
	LeaveDrainTime time.Duration

	// AutopilotConfig is used to apply the initial autopilot config when
	// bootstrapping.
	AutopilotConfig *autopilot.Config

	// ServerHealthInterval is the frequency with which the health of the
	// servers in the cluster will be updated.
	ServerHealthInterval time.Duration

	// AutopilotInterval is the frequency with which the leader will perform
	// autopilot tasks, such as promoting eligible non-voters and removing
	// dead servers.
	AutopilotInterval time.Duration

	// ConnectEnabled is whether to enable Connect features such as the CA.
	ConnectEnabled bool

	// ConnectMeshGatewayWANFederationEnabled determines if wan federation of
	// datacenters should exclusively traverse mesh gateways.
	ConnectMeshGatewayWANFederationEnabled bool

	// DisableFederationStateAntiEntropy solely exists for use in unit tests to
	// disable a background routine.
	DisableFederationStateAntiEntropy bool

	// DisableFederationStateDatacenterNameValidation solely exists for use in
	// unit tests to allow federation states to be persisted that are not
	// currently on the WAN.
	DisableFederationStateDatacenterNameValidation bool

	// CAConfig is used to apply the initial Connect CA configuration when
	// bootstrapping.
	CAConfig *structs.CAConfiguration

	// ConfigEntryBootstrap contains a list of ConfigEntries to ensure are created
	// If entries of the same Kind/Name exist already these will not update them.
	ConfigEntryBootstrap []structs.ConfigEntry

	// AutoEncryptAllowTLS is whether to enable the server responding to
	// AutoEncrypt.Sign requests.
	AutoEncryptAllowTLS bool

	// Embedded Consul Enterprise specific configuration
	*EnterpriseConfig
}

// ToTLSUtilConfig is only used by tests, usually the config is being passed
// down from the agent.
func (c *Config) ToTLSUtilConfig() tlsutil.Config {
	return tlsutil.Config{
		VerifyIncoming:           c.VerifyIncoming,
		VerifyOutgoing:           c.VerifyOutgoing,
		VerifyServerHostname:     c.VerifyServerHostname,
		CAFile:                   c.CAFile,
		CAPath:                   c.CAPath,
		CertFile:                 c.CertFile,
		KeyFile:                  c.KeyFile,
		NodeName:                 c.NodeName,
		Domain:                   c.Domain,
		ServerName:               c.ServerName,
		TLSMinVersion:            c.TLSMinVersion,
		CipherSuites:             c.TLSCipherSuites,
		PreferServerCipherSuites: c.TLSPreferServerCipherSuites,
	}
}

// CheckProtocolVersion validates the protocol version.
func (c *Config) CheckProtocolVersion() error {
	if c.ProtocolVersion < ProtocolVersionMin {
		return fmt.Errorf("Protocol version '%d' too low. Must be in range: [%d, %d]", c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}
	if c.ProtocolVersion > ProtocolVersionMax {
		return fmt.Errorf("Protocol version '%d' too high. Must be in range: [%d, %d]", c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}
	return nil
}

// CheckACL validates the ACL configuration.
func (c *Config) CheckACL() error {
	switch c.ACLDefaultPolicy {
	case "allow":
	case "deny":
	default:
		return fmt.Errorf("Unsupported default ACL policy: %s", c.ACLDefaultPolicy)
	}
	switch c.ACLDownPolicy {
	case "allow":
	case "deny":
	case "async-cache", "extend-cache":
	default:
		return fmt.Errorf("Unsupported down ACL policy: %s", c.ACLDownPolicy)
	}
	return nil
}

// DefaultConfig returns a sane default configuration.
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	conf := &Config{
		Build:                                version.Version,
		Datacenter:                           DefaultDC,
		NodeName:                             hostname,
		RPCAddr:                              DefaultRPCAddr,
		RaftConfig:                           raft.DefaultConfig(),
		SerfLANConfig:                        lib.SerfDefaultConfig(),
		SerfWANConfig:                        lib.SerfDefaultConfig(),
		SerfFloodInterval:                    60 * time.Second,
		ReconcileInterval:                    60 * time.Second,
		ProtocolVersion:                      ProtocolVersion2Compatible,
		ACLRoleTTL:                           30 * time.Second,
		ACLPolicyTTL:                         30 * time.Second,
		ACLTokenTTL:                          30 * time.Second,
		ACLDefaultPolicy:                     "allow",
		ACLDownPolicy:                        "extend-cache",
		ACLReplicationRate:                   1,
		ACLReplicationBurst:                  5,
		ACLReplicationApplyLimit:             100, // ops / sec
		ConfigReplicationRate:                1,
		ConfigReplicationBurst:               5,
		ConfigReplicationApplyLimit:          100, // ops / sec
		FederationStateReplicationRate:       1,
		FederationStateReplicationBurst:      5,
		FederationStateReplicationApplyLimit: 100, // ops / sec
		TombstoneTTL:                         15 * time.Minute,
		TombstoneTTLGranularity:              30 * time.Second,
		SessionTTLMin:                        10 * time.Second,
		ACLTokenMinExpirationTTL:             1 * time.Minute,
		ACLTokenMaxExpirationTTL:             24 * time.Hour,

		// These are tuned to provide a total throughput of 128 updates
		// per second. If you update these, you should update the client-
		// side SyncCoordinateRateTarget parameter accordingly.
		CoordinateUpdatePeriod:     5 * time.Second,
		CoordinateUpdateBatchSize:  128,
		CoordinateUpdateMaxBatches: 5,

		CheckOutputMaxSize: checks.DefaultBufSize,

		RPCRate:     rate.Inf,
		RPCMaxBurst: 1000,

		TLSMinVersion: "tls10",

		// TODO (slackpad) - Until #3744 is done, we need to keep these
		// in sync with agent/config/default.go.
		AutopilotConfig: &autopilot.Config{
			CleanupDeadServers:      true,
			LastContactThreshold:    200 * time.Millisecond,
			MaxTrailingLogs:         250,
			ServerStabilizationTime: 10 * time.Second,
		},

		CAConfig: &structs.CAConfiguration{
			Provider: "consul",
			Config: map[string]interface{}{
				"RotationPeriod":      "2160h",
				"LeafCertTTL":         "72h",
				"IntermediateCertTTL": "8760h", // 365 * 24h
			},
		},

		ServerHealthInterval: 2 * time.Second,
		AutopilotInterval:    10 * time.Second,
		DefaultQueryTime:     300 * time.Second,
		MaxQueryTime:         600 * time.Second,
		EnterpriseConfig:     DefaultEnterpriseConfig(),
	}

	// Increase our reap interval to 3 days instead of 24h.
	conf.SerfLANConfig.ReconnectTimeout = 3 * 24 * time.Hour
	conf.SerfWANConfig.ReconnectTimeout = 3 * 24 * time.Hour

	// WAN Serf should use the WAN timing, since we are using it
	// to communicate between DC's
	conf.SerfWANConfig.MemberlistConfig = memberlist.DefaultWANConfig()

	// Ensure we don't have port conflicts
	conf.SerfLANConfig.MemberlistConfig.BindPort = DefaultLANSerfPort
	conf.SerfWANConfig.MemberlistConfig.BindPort = DefaultWANSerfPort

	// Allow dead nodes to be replaced after 30 seconds.
	conf.SerfLANConfig.MemberlistConfig.DeadNodeReclaimTime = 30 * time.Second
	conf.SerfWANConfig.MemberlistConfig.DeadNodeReclaimTime = 30 * time.Second

	// Raft protocol version 3 only works with other Consul servers running
	// 0.8.0 or later.
	conf.RaftConfig.ProtocolVersion = 3

	// Disable shutdown on removal
	conf.RaftConfig.ShutdownOnRemove = false

	// Check every 5 seconds to see if there are enough new entries for a snapshot, can be overridden
	conf.RaftConfig.SnapshotInterval = 30 * time.Second

	// Snapshots are created every 16384 entries by default, can be overridden
	conf.RaftConfig.SnapshotThreshold = 16384

	return conf
}
