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
	AEInterval                 time.Duration
	ACLDisabledTTL             time.Duration
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

	ACLAgentMasterToken    string
	ACLAgentToken          string
	ACLDatacenter          string
	ACLDefaultPolicy       string
	ACLDownPolicy          string
	ACLEnforceVersion8     bool
	ACLEnableKeyListPolicy bool
	ACLMasterToken         string
	ACLReplicationToken    string
	ACLTTL                 time.Duration
	ACLToken               string

	AutopilotCleanupDeadServers      bool
	AutopilotDisableUpgradeMigration bool
	AutopilotLastContactThreshold    time.Duration
	AutopilotMaxTrailingLogs         int
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
	TelemetryMetricsPrefix                      string
	TelemetryStatsdAddr                         string
	TelemetryStatsiteAddr                       string

	// Datacenter and NodeName are exposed via /v1/agent/self from here and
	// used in lots of places like CLI commands. Treat this as an interface
	// that must be stable.
	Datacenter string
	NodeName   string

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
	DevMode                     bool
	DisableAnonymousSignature   bool
	DisableCoordinates          bool
	DisableHostNodeID           bool
	DisableKeyringFile          bool
	DisableRemoteExec           bool
	DisableUpdateCheck          bool
	DiscardCheckOutput          bool
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
	LeaveDrainTime              time.Duration
	LeaveOnTerm                 bool
	LogLevel                    string
	NodeID                      types.NodeID
	NodeMeta                    map[string]string
	NonVotingServer             bool
	PidFile                     string
	RPCAdvertiseAddr            *net.TCPAddr
	RPCBindAddr                 *net.TCPAddr
	RPCHoldTimeout              time.Duration
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
