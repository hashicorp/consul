package config

import (
	"net"
	"time"

	"crypto/tls"

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
	AEInterval                 time.Duration
	ACLDisabledTTL             time.Duration
	CheckDeregisterIntervalMin time.Duration
	CheckReapInterval          time.Duration
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

	ACLAgentMasterToken string
	ACLAgentToken       string
	ACLDatacenter       string
	ACLDefaultPolicy    string
	ACLDownPolicy       string
	ACLEnforceVersion8  bool
	ACLMasterToken      string
	ACLReplicationToken string
	ACLTTL              time.Duration
	ACLToken            string

	AutopilotCleanupDeadServers      bool
	AutopilotDisableUpgradeMigration bool
	AutopilotLastContactThreshold    time.Duration
	AutopilotMaxTrailingLogs         int64
	AutopilotRedundancyZoneTag       string
	AutopilotServerStabilizationTime time.Duration
	AutopilotUpgradeVersionTag       string

	DNSAllowStale         bool
	DNSDisableCompression bool
	DNSDomain             string
	DNSEnableTruncate     bool
	DNSMaxStale           time.Duration
	DNSNodeTTL            time.Duration
	DNSOnlyPassing        bool
	DNSRecursorTimeout    time.Duration
	DNSServiceTTL         map[string]time.Duration
	DNSUDPAnswerLimit     int
	DNSRecursors          []string

	HTTPBlockEndpoints  []string
	HTTPResponseHeaders map[string]string

	PerformanceRaftMultiplier int

	TelemetryCirconusAPIApp                     string
	TelemetryCirconusAPIToken                   string
	TelemetryCirconusAPIURL                     string
	TelemetryCirconusBrokerID                   string
	TelemetryCirconusBrokerSelectTag            string
	TelemetryCirconusCheckDisplayName           string
	TelemetryCirconusCheckForceMetricActivation string
	TelemetryCirconusCheckID                    string
	TelemetryCirconusCheckInstanceID            string
	TelemetryCirconusCheckSearchTag             string
	TelemetryCirconusCheckTags                  string
	TelemetryCirconusSubmissionInterval         string
	TelemetryCirconusSubmissionURL              string
	TelemetryDisableHostname                    bool
	TelemetryDogstatsdAddr                      string
	TelemetryDogstatsdTags                      []string
	TelemetryFilterDefault                      bool
	TelemetryAllowedPrefixes                    []string
	TelemetryBlockedPrefixes                    []string
	TelemetryStatsdAddr                         string
	TelemetryStatsiteAddr                       string
	TelemetryStatsitePrefix                     string

	AdvertiseAddrLAN            *net.IPAddr
	AdvertiseAddrWAN            *net.IPAddr
	BindAddr                    *net.IPAddr
	Bootstrap                   bool
	BootstrapExpect             int
	CAFile                      string
	CAPath                      string
	CertFile                    string
	CheckUpdateInterval         time.Duration
	Checks                      []*structs.CheckDefinition
	ClientAddrs                 []*net.IPAddr
	DNSAddrs                    []net.Addr
	DNSPort                     int
	DataDir                     string
	Datacenter                  string
	DevMode                     bool
	DisableAnonymousSignature   bool
	DisableCoordinates          bool
	DisableHostNodeID           bool
	DisableKeyringFile          bool
	DisableRemoteExec           bool
	DisableUpdateCheck          bool
	EnableACLReplication        bool
	EnableDebug                 bool
	EnableScriptChecks          bool
	EnableSyslog                bool
	EnableUI                    bool
	EncryptKey                  string
	EncryptVerifyIncoming       bool
	EncryptVerifyOutgoing       bool
	HTTPAddrs                   []net.Addr
	HTTPPort                    int
	HTTPSAddrs                  []net.Addr
	HTTPSPort                   int
	KeyFile                     string
	LeaveOnTerm                 bool
	LogLevel                    string
	NodeID                      types.NodeID
	NodeMeta                    map[string]string
	NodeName                    string
	NonVotingServer             bool
	PidFile                     string
	RPCAdvertiseAddr            *net.TCPAddr
	RPCBindAddr                 *net.TCPAddr
	RPCMaxBurst                 int
	RPCProtocol                 int
	RPCRateLimit                rate.Limit
	RaftProtocol                int
	ReconnectTimeoutLAN         time.Duration
	ReconnectTimeoutWAN         time.Duration
	RejoinAfterLeave            bool
	RetryJoinIntervalLAN        time.Duration
	RetryJoinIntervalWAN        time.Duration
	RetryJoinLAN                []string
	RetryJoinMaxAttemptsLAN     int
	RetryJoinMaxAttemptsWAN     int
	RetryJoinWAN                []string
	SegmentName                 string
	Segments                    []structs.NetworkSegment
	SerfAdvertiseAddrLAN        *net.TCPAddr
	SerfAdvertiseAddrWAN        *net.TCPAddr
	SerfBindAddrLAN             *net.TCPAddr
	SerfBindAddrWAN             *net.TCPAddr
	SerfPortLAN                 int
	SerfPortWAN                 int
	ServerMode                  bool
	ServerName                  string
	ServerPort                  int
	Services                    []*structs.ServiceDefinition
	SessionTTLMin               time.Duration
	SkipLeaveOnInt              bool
	StartJoinAddrsLAN           []string
	StartJoinAddrsWAN           []string
	SyslogFacility              string
	TLSCipherSuites             []uint16
	TLSMinVersion               string
	TLSPreferServerCipherSuites bool
	TaggedAddresses             map[string]string
	TranslateWANAddrs           bool
	UIDir                       string
	UnixSocketGroup             string
	UnixSocketMode              string
	UnixSocketUser              string
	VerifyIncoming              bool
	VerifyIncomingHTTPS         bool
	VerifyIncomingRPC           bool
	VerifyOutgoing              bool
	VerifyServerHostname        bool
	Watches                     []map[string]interface{}
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

func (c *RuntimeConfig) Sanitized() RuntimeConfig {
	var rt RuntimeConfig
	rt = *c
	rt.ACLMasterToken = "hidden"
	rt.ACLToken = "hidden"
	rt.ACLReplicationToken = "hidden"
	rt.ACLAgentMasterToken = "hidden"
	rt.ACLAgentToken = "hidden"
	rt.EncryptKey = "hidden"
	rt.RetryJoinLAN = []string{"hidden (improve me)"}
	rt.RetryJoinWAN = []string{"hidden (improve me)"}
	rt.TelemetryCirconusAPIToken = "hidden"
	return rt
}
