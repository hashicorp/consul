package agent

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/pascaldekloe/goe/verify"
)

func TestConfigEncryptBytes(t *testing.T) {
	t.Parallel()
	// Test with some input
	src := []byte("abc")
	c := &Config{
		EncryptKey: base64.StdEncoding.EncodeToString(src),
	}

	result, err := c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(src, result) {
		t.Fatalf("bad: %#v", result)
	}

	// Test with no input
	c = &Config{}
	result, err = c.EncryptBytes()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) > 0 {
		t.Fatalf("bad: %#v", result)
	}
}

func TestDecodeConfig(t *testing.T) {
	tests := []struct {
		desc string
		in   string
		c    *Config
		err  error
	}{
		// special flows
		{
			in:  `{"bad": "no way jose"}`,
			err: errors.New("Config has invalid keys: bad"),
		},

		// happy flows in alphabeical order
		{
			in: `{"acl_agent_master_token":"a"}`,
			c:  &Config{ACLAgentMasterToken: "a"},
		},
		{
			in: `{"acl_agent_token":"a"}`,
			c:  &Config{ACLAgentToken: "a"},
		},
		{
			in: `{"acl_datacenter":"a"}`,
			c:  &Config{ACLDatacenter: "a"},
		},
		{
			in: `{"acl_default_policy":"a"}`,
			c:  &Config{ACLDefaultPolicy: "a"},
		},
		{
			in: `{"acl_down_policy":"a"}`,
			c:  &Config{ACLDownPolicy: "a"},
		},
		{
			in: `{"acl_enforce_version_8":true}`,
			c:  &Config{ACLEnforceVersion8: Bool(true)},
		},
		{
			in: `{"acl_master_token":"a"}`,
			c:  &Config{ACLMasterToken: "a"},
		},
		{
			in: `{"acl_replication_token":"a"}`,
			c:  &Config{ACLReplicationToken: "a"},
		},
		{
			in: `{"acl_token":"a"}`,
			c:  &Config{ACLToken: "a"},
		},
		{
			in: `{"acl_ttl":"2s"}`,
			c:  &Config{ACLTTL: 2 * time.Second, ACLTTLRaw: "2s"},
		},
		{
			in: `{"addresses":{"dns":"a"}}`,
			c:  &Config{Addresses: AddressConfig{DNS: "a"}},
		},
		{
			in: `{"addresses":{"http":"a"}}`,
			c:  &Config{Addresses: AddressConfig{HTTP: "a"}},
		},
		{
			in: `{"addresses":{"https":"a"}}`,
			c:  &Config{Addresses: AddressConfig{HTTPS: "a"}},
		},
		{
			in: `{"addresses":{"rpc":"a"}}`,
			c:  &Config{Addresses: AddressConfig{RPC: "a"}},
		},
		{
			in: `{"advertise_addr":"a"}`,
			c:  &Config{AdvertiseAddr: "a"},
		},
		{
			in: `{"advertise_addr_wan":"a"}`,
			c:  &Config{AdvertiseAddrWan: "a"},
		},
		{
			in: `{"advertise_addrs":{"rpc":"1.2.3.4:5678"}}`,
			c: &Config{
				AdvertiseAddrs: AdvertiseAddrsConfig{
					RPC:    &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
					RPCRaw: "1.2.3.4:5678",
				},
			},
		},
		{
			in: `{"advertise_addrs":{"serf_lan":"1.2.3.4:5678"}}`,
			c: &Config{
				AdvertiseAddrs: AdvertiseAddrsConfig{
					SerfLan:    &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
					SerfLanRaw: "1.2.3.4:5678",
				},
			},
		},
		{
			in: `{"advertise_addrs":{"serf_wan":"1.2.3.4:5678"}}`,
			c: &Config{
				AdvertiseAddrs: AdvertiseAddrsConfig{
					SerfWan:    &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
					SerfWanRaw: "1.2.3.4:5678",
				},
			},
		},
		{
			in: `{"atlas_acl_token":"a"}`,
			c:  &Config{DeprecatedAtlasACLToken: "a"},
		},
		{
			in: `{"atlas_endpoint":"a"}`,
			c:  &Config{DeprecatedAtlasEndpoint: "a"},
		},
		{
			in: `{"atlas_infrastructure":"a"}`,
			c:  &Config{DeprecatedAtlasInfrastructure: "a"},
		},
		{
			in: `{"atlas_join":true}`,
			c:  &Config{DeprecatedAtlasJoin: true},
		},
		{
			in: `{"atlas_token":"a"}`,
			c:  &Config{DeprecatedAtlasToken: "a"},
		},
		{
			in: `{"autopilot":{"cleanup_dead_servers":true}}`,
			c:  &Config{Autopilot: Autopilot{CleanupDeadServers: Bool(true)}},
		},
		{
			in: `{"autopilot":{"disable_upgrade_migration":true}}`,
			c:  &Config{Autopilot: Autopilot{DisableUpgradeMigration: Bool(true)}},
		},
		{
			in: `{"autopilot":{"last_contact_threshold":"2s"}}`,
			c:  &Config{Autopilot: Autopilot{LastContactThreshold: Duration(2 * time.Second), LastContactThresholdRaw: "2s"}},
		},
		{
			in: `{"autopilot":{"max_trailing_logs":10}}`,
			c:  &Config{Autopilot: Autopilot{MaxTrailingLogs: Uint64(10)}},
		},
		{
			in: `{"autopilot":{"server_stabilization_time":"2s"}}`,
			c:  &Config{Autopilot: Autopilot{ServerStabilizationTime: Duration(2 * time.Second), ServerStabilizationTimeRaw: "2s"}},
		},
		{
			in: `{"autopilot":{"cleanup_dead_servers":true}}`,
			c:  &Config{Autopilot: Autopilot{CleanupDeadServers: Bool(true)}},
		},
		{
			in: `{"bind_addr":"a"}`,
			c:  &Config{BindAddr: "a"},
		},
		{
			in: `{"bootstrap":true}`,
			c:  &Config{Bootstrap: true},
		},
		{
			in: `{"bootstrap_expect":3}`,
			c:  &Config{BootstrapExpect: 3},
		},
		{
			in: `{"ca_file":"a"}`,
			c:  &Config{CAFile: "a"},
		},
		{
			in: `{"ca_path":"a"}`,
			c:  &Config{CAPath: "a"},
		},
		{
			in: `{"check_update_interval":"2s"}`,
			c:  &Config{CheckUpdateInterval: 2 * time.Second, CheckUpdateIntervalRaw: "2s"},
		},
		{
			in: `{"cert_file":"a"}`,
			c:  &Config{CertFile: "a"},
		},
		{
			in: `{"client_addr":"a"}`,
			c:  &Config{ClientAddr: "a"},
		},
		{
			in: `{"data_dir":"a"}`,
			c:  &Config{DataDir: "a"},
		},
		{
			in: `{"datacenter":"a"}`,
			c:  &Config{Datacenter: "a"},
		},
		{
			in: `{"disable_coordinates":true}`,
			c:  &Config{DisableCoordinates: true},
		},
		{
			in: `{"disable_host_node_id":false}`,
			c:  &Config{DisableHostNodeID: Bool(false)},
		},
		{
			in: `{"dns_config":{"allow_stale":true}}`,
			c:  &Config{DNSConfig: DNSConfig{AllowStale: Bool(true)}},
		},
		{
			in: `{"dns_config":{"disable_compression":true}}`,
			c:  &Config{DNSConfig: DNSConfig{DisableCompression: true}},
		},
		{
			in: `{"dns_config":{"enable_truncate":true}}`,
			c:  &Config{DNSConfig: DNSConfig{EnableTruncate: true}},
		},
		{
			in: `{"dns_config":{"max_stale":"2s"}}`,
			c:  &Config{DNSConfig: DNSConfig{MaxStale: 2 * time.Second, MaxStaleRaw: "2s"}},
		},
		{
			in: `{"dns_config":{"node_ttl":"2s"}}`,
			c:  &Config{DNSConfig: DNSConfig{NodeTTL: 2 * time.Second, NodeTTLRaw: "2s"}},
		},
		{
			in: `{"dns_config":{"only_passing":true}}`,
			c:  &Config{DNSConfig: DNSConfig{OnlyPassing: true}},
		},
		{
			in: `{"dns_config":{"recursor_timeout":"2s"}}`,
			c:  &Config{DNSConfig: DNSConfig{RecursorTimeout: 2 * time.Second, RecursorTimeoutRaw: "2s"}},
		},
		{
			in: `{"dns_config":{"service_ttl":{"*":"2s","a":"456s"}}}`,
			c: &Config{
				DNSConfig: DNSConfig{
					ServiceTTL:    map[string]time.Duration{"*": 2 * time.Second, "a": 456 * time.Second},
					ServiceTTLRaw: map[string]string{"*": "2s", "a": "456s"},
				},
			},
		},
		{
			in: `{"dns_config":{"udp_answer_limit":123}}`,
			c:  &Config{DNSConfig: DNSConfig{UDPAnswerLimit: 123}},
		},
		{
			in: `{"disable_anonymous_signature":true}`,
			c:  &Config{DisableAnonymousSignature: true},
		},
		{
			in: `{"disable_remote_exec":false}`,
			c:  &Config{DisableRemoteExec: Bool(false)},
		},
		{
			in: `{"disable_update_check":true}`,
			c:  &Config{DisableUpdateCheck: true},
		},
		{
			in: `{"dogstatsd_addr":"a"}`,
			c:  &Config{Telemetry: Telemetry{DogStatsdAddr: "a"}},
		},
		{
			in: `{"dogstatsd_tags":["a:b","c:d"]}`,
			c:  &Config{Telemetry: Telemetry{DogStatsdTags: []string{"a:b", "c:d"}}},
		},
		{
			in: `{"domain":"a"}`,
			c:  &Config{Domain: "a"},
		},
		{
			in: `{"enable_debug":true}`,
			c:  &Config{EnableDebug: true},
		},
		{
			in: `{"enable_syslog":true}`,
			c:  &Config{EnableSyslog: true},
		},
		{
			in: `{"disable_keyring_file":true}`,
			c:  &Config{DisableKeyringFile: true},
		},
		{
			in: `{"encrypt_verify_incoming":true}`,
			c:  &Config{EncryptVerifyIncoming: Bool(true)},
		},
		{
			in: `{"encrypt_verify_outgoing":true}`,
			c:  &Config{EncryptVerifyOutgoing: Bool(true)},
		},
		{
			in: `{"http_api_response_headers":{"a":"b","c":"d"}}`,
			c:  &Config{HTTPConfig: HTTPConfig{ResponseHeaders: map[string]string{"a": "b", "c": "d"}}},
		},
		{
			in: `{"http_config":{"response_headers":{"a":"b","c":"d"}}}`,
			c:  &Config{HTTPConfig: HTTPConfig{ResponseHeaders: map[string]string{"a": "b", "c": "d"}}},
		},
		{
			in: `{"key_file":"a"}`,
			c:  &Config{KeyFile: "a"},
		},
		{
			in: `{"leave_on_terminate":true}`,
			c:  &Config{LeaveOnTerm: Bool(true)},
		},
		{
			in: `{"log_level":"a"}`,
			c:  &Config{LogLevel: "a"},
		},
		{
			in: `{"node_id":"a"}`,
			c:  &Config{NodeID: "a"},
		},
		{
			in: `{"node_meta":{"a":"b","c":"d"}}`,
			c:  &Config{Meta: map[string]string{"a": "b", "c": "d"}},
		},
		{
			in: `{"node_name":"a"}`,
			c:  &Config{NodeName: "a"},
		},
		{
			in: `{"performance": { "raft_multiplier": 3 }}`,
			c:  &Config{Performance: Performance{RaftMultiplier: 3}},
		},
		{
			in:  `{"performance": { "raft_multiplier": 11 }}`,
			err: errors.New("Performance.RaftMultiplier must be <= 10"),
		},
		{
			in: `{"pid_file":"a"}`,
			c:  &Config{PidFile: "a"},
		},
		{
			in: `{"ports":{"dns":1234}}`,
			c:  &Config{Ports: PortConfig{DNS: 1234}},
		},
		{
			in: `{"ports":{"http":1234}}`,
			c:  &Config{Ports: PortConfig{HTTP: 1234}},
		},
		{
			in: `{"ports":{"https":1234}}`,
			c:  &Config{Ports: PortConfig{HTTPS: 1234}},
		},
		{
			in: `{"ports":{"serf_lan":1234}}`,
			c:  &Config{Ports: PortConfig{SerfLan: 1234}},
		},
		{
			in: `{"ports":{"serf_wan":1234}}`,
			c:  &Config{Ports: PortConfig{SerfWan: 1234}},
		},
		{
			in: `{"ports":{"server":1234}}`,
			c:  &Config{Ports: PortConfig{Server: 1234}},
		},
		{
			in: `{"ports":{"rpc":1234}}`,
			c:  &Config{Ports: PortConfig{RPC: 1234}},
		},
		{
			in: `{"raft_protocol":3}`,
			c:  &Config{RaftProtocol: 3},
		},
		{
			in:  `{"reconnect_timeout":"4h"}`,
			err: errors.New("ReconnectTimeoutLan must be >= 8h0m0s"),
		},
		{
			in: `{"reconnect_timeout":"8h"}`,
			c:  &Config{ReconnectTimeoutLan: 8 * time.Hour, ReconnectTimeoutLanRaw: "8h"},
		},
		{
			in:  `{"reconnect_timeout_wan":"4h"}`,
			err: errors.New("ReconnectTimeoutWan must be >= 8h0m0s"),
		},
		{
			in: `{"reconnect_timeout_wan":"8h"}`,
			c:  &Config{ReconnectTimeoutWan: 8 * time.Hour, ReconnectTimeoutWanRaw: "8h"},
		},
		{
			in: `{"recursor":"a"}`,
			c:  &Config{DNSRecursor: "a", DNSRecursors: []string{"a"}},
		},
		{
			in: `{"recursors":["a","b"]}`,
			c:  &Config{DNSRecursors: []string{"a", "b"}},
		},
		{
			in: `{"rejoin_after_leave":true}`,
			c:  &Config{RejoinAfterLeave: true},
		},
		{
			in: `{"retry_interval":"2s"}`,
			c:  &Config{RetryInterval: 2 * time.Second, RetryIntervalRaw: "2s"},
		},
		{
			in: `{"retry_interval_wan":"2s"}`,
			c:  &Config{RetryIntervalWan: 2 * time.Second, RetryIntervalWanRaw: "2s"},
		},
		{
			in: `{"retry_join":["a","b"]}`,
			c:  &Config{RetryJoin: []string{"a", "b"}},
		},
		{
			in: `{"retry_join_azure":{"client_id":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{ClientID: "a"}},
		},
		{
			in: `{"retry_join_azure":{"tag_name":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{TagName: "a"}},
		},
		{
			in: `{"retry_join_azure":{"tag_value":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{TagValue: "a"}},
		},
		{
			in: `{"retry_join_azure":{"secret_access_key":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{SecretAccessKey: "a"}},
		},
		{
			in: `{"retry_join_azure":{"subscription_id":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{SubscriptionID: "a"}},
		},
		{
			in: `{"retry_join_azure":{"tenant_id":"a"}}`,
			c:  &Config{RetryJoinAzure: RetryJoinAzure{TenantID: "a"}},
		},
		{
			in: `{"retry_join_ec2":{"access_key_id":"a"}}`,
			c:  &Config{RetryJoinEC2: RetryJoinEC2{AccessKeyID: "a"}},
		},
		{
			in: `{"retry_join_ec2":{"region":"a"}}`,
			c:  &Config{RetryJoinEC2: RetryJoinEC2{Region: "a"}},
		},
		{
			in: `{"retry_join_ec2":{"tag_key":"a"}}`,
			c:  &Config{RetryJoinEC2: RetryJoinEC2{TagKey: "a"}},
		},
		{
			in: `{"retry_join_ec2":{"tag_value":"a"}}`,
			c:  &Config{RetryJoinEC2: RetryJoinEC2{TagValue: "a"}},
		},
		{
			in: `{"retry_join_ec2":{"secret_access_key":"a"}}`,
			c:  &Config{RetryJoinEC2: RetryJoinEC2{SecretAccessKey: "a"}},
		},
		{
			in: `{"retry_join_gce":{"credentials_file":"a"}}`,
			c:  &Config{RetryJoinGCE: RetryJoinGCE{CredentialsFile: "a"}},
		},
		{
			in: `{"retry_join_gce":{"project_name":"a"}}`,
			c:  &Config{RetryJoinGCE: RetryJoinGCE{ProjectName: "a"}},
		},
		{
			in: `{"retry_join_gce":{"tag_value":"a"}}`,
			c:  &Config{RetryJoinGCE: RetryJoinGCE{TagValue: "a"}},
		},
		{
			in: `{"retry_join_gce":{"zone_pattern":"a"}}`,
			c:  &Config{RetryJoinGCE: RetryJoinGCE{ZonePattern: "a"}},
		},
		{
			in: `{"retry_join_wan":["a","b"]}`,
			c:  &Config{RetryJoinWan: []string{"a", "b"}},
		},
		{
			in: `{"retry_max":123}`,
			c:  &Config{RetryMaxAttempts: 123},
		},
		{
			in: `{"retry_max_wan":123}`,
			c:  &Config{RetryMaxAttemptsWan: 123},
		},
		{
			in: `{"serf_lan_bind":"a"}`,
			c:  &Config{SerfLanBindAddr: "a"},
		},
		{
			in: `{"serf_wan_bind":"a"}`,
			c:  &Config{SerfWanBindAddr: "a"},
		},
		{
			in: `{"server":true}`,
			c:  &Config{Server: true},
		},
		{
			in: `{"server_name":"a"}`,
			c:  &Config{ServerName: "a"},
		},
		{
			in: `{"session_ttl_min":"2s"}`,
			c:  &Config{SessionTTLMin: 2 * time.Second, SessionTTLMinRaw: "2s"},
		},
		{
			in: `{"skip_leave_on_interrupt":true}`,
			c:  &Config{SkipLeaveOnInt: Bool(true)},
		},
		{
			in: `{"start_join":["a","b"]}`,
			c:  &Config{StartJoin: []string{"a", "b"}},
		},
		{
			in: `{"start_join_wan":["a","b"]}`,
			c:  &Config{StartJoinWan: []string{"a", "b"}},
		},
		{
			in: `{"statsd_addr":"a"}`,
			c:  &Config{Telemetry: Telemetry{StatsdAddr: "a"}},
		},
		{
			in: `{"statsite_addr":"a"}`,
			c:  &Config{Telemetry: Telemetry{StatsiteAddr: "a"}},
		},
		{
			in: `{"statsite_prefix":"a"}`,
			c:  &Config{Telemetry: Telemetry{StatsitePrefix: "a"}},
		},
		{
			in: `{"syslog_facility":"a"}`,
			c:  &Config{SyslogFacility: "a"},
		},
		{
			in: `{"telemetry":{"circonus_api_app":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusAPIApp: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_api_token":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusAPIToken: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_api_url":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusAPIURL: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_broker_id":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusBrokerID: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_broker_select_tag":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusBrokerSelectTag: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_display_name":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckDisplayName: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_force_metric_activation":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckForceMetricActivation: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_id":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckID: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_instance_id":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckInstanceID: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_search_tag":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckSearchTag: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_check_tags":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckTags: "a"}},
		},
		{
			in: `{"telemetry":{"circonus_submission_interval":"2s"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusSubmissionInterval: "2s"}},
		},
		{
			in: `{"telemetry":{"circonus_submission_url":"a"}}`,
			c:  &Config{Telemetry: Telemetry{CirconusCheckSubmissionURL: "a"}},
		},
		{
			in: `{"telemetry":{"disable_hostname":true}}`,
			c:  &Config{Telemetry: Telemetry{DisableHostname: true}},
		},
		{
			in: `{"telemetry":{"dogstatsd_addr":"a"}}`,
			c:  &Config{Telemetry: Telemetry{DogStatsdAddr: "a"}},
		},
		{
			in: `{"telemetry":{"dogstatsd_tags":["a","b"]}}`,
			c:  &Config{Telemetry: Telemetry{DogStatsdTags: []string{"a", "b"}}},
		},
		{
			in: `{"telemetry":{"statsd_address":"a"}}`,
			c:  &Config{Telemetry: Telemetry{StatsdAddr: "a"}},
		},
		{
			in: `{"telemetry":{"statsite_address":"a"}}`,
			c:  &Config{Telemetry: Telemetry{StatsiteAddr: "a"}},
		},
		{
			in: `{"telemetry":{"statsite_prefix":"a"}}`,
			c:  &Config{Telemetry: Telemetry{StatsitePrefix: "a"}},
		},
		{
			in: `{"tls_cipher_suites":"TLS_RSA_WITH_AES_256_CBC_SHA"}`,
			c: &Config{
				TLSCipherSuites:    []uint16{tls.TLS_RSA_WITH_AES_256_CBC_SHA},
				TLSCipherSuitesRaw: "TLS_RSA_WITH_AES_256_CBC_SHA",
			},
		},
		{
			in: `{"tls_min_version":"a"}`,
			c:  &Config{TLSMinVersion: "a"},
		},
		{
			in: `{"tls_prefer_server_cipher_suites":true}`,
			c:  &Config{TLSPreferServerCipherSuites: true},
		},
		{
			in: `{"translate_wan_addrs":true}`,
			c:  &Config{TranslateWanAddrs: true},
		},
		{
			in: `{"ui":true}`,
			c:  &Config{EnableUI: true},
		},
		{
			in: `{"ui_dir":"a"}`,
			c:  &Config{UIDir: "a"},
		},
		{
			in: `{"unix_sockets":{"user":"a"}}`,
			c:  &Config{UnixSockets: UnixSocketConfig{UnixSocketPermissions{Usr: "a"}}},
		},
		{
			in: `{"unix_sockets":{"group":"a"}}`,
			c:  &Config{UnixSockets: UnixSocketConfig{UnixSocketPermissions{Grp: "a"}}},
		},
		{
			in: `{"unix_sockets":{"mode":"a"}}`,
			c:  &Config{UnixSockets: UnixSocketConfig{UnixSocketPermissions{Perms: "a"}}},
		},
		{
			in: `{"verify_incoming":true}`,
			c:  &Config{VerifyIncoming: true},
		},
		{
			in: `{"verify_incoming_https":true}`,
			c:  &Config{VerifyIncomingHTTPS: true},
		},
		{
			in: `{"verify_incoming_rpc":true}`,
			c:  &Config{VerifyIncomingRPC: true},
		},
		{
			in: `{"verify_outgoing":true}`,
			c:  &Config{VerifyOutgoing: true},
		},
		{
			in: `{"verify_server_hostname":true}`,
			c:  &Config{VerifyServerHostname: true},
		},
		{
			in: `{"watches":[{"type":"a","prefix":"b","handler":"c"}]}`,
			c: &Config{
				Watches: []map[string]interface{}{
					map[string]interface{}{
						"type":    "a",
						"prefix":  "b",
						"handler": "c",
					},
				},
			},
		},

		// complex flows
		{
			desc: "single service with check",
			in: `{
					"service": {
						"ID": "a",
						"Name": "b",
						"Tags": ["c", "d"],
						"Address": "e",
						"Token": "f",
						"Port": 123,
						"EnableTagOverride": true,
						"Check": {
							"CheckID": "g",
							"Name": "h",
							"Status": "i",
							"Notes": "j",
							"Script": "k",
							"HTTP": "l",
							"Header": {"a":["b"], "c":["d", "e"]},
							"Method": "x",
							"TCP": "m",
							"DockerContainerID": "n",
							"Shell": "o",
							"TLSSkipVerify": true,
							"Interval": "2s",
							"Timeout": "3s",
							"TTL": "4s",
							"DeregisterCriticalServiceAfter": "5s"
						}
					}
				}`,
			c: &Config{
				Services: []*structs.ServiceDefinition{
					&structs.ServiceDefinition{
						ID:                "a",
						Name:              "b",
						Tags:              []string{"c", "d"},
						Address:           "e",
						Port:              123,
						Token:             "f",
						EnableTagOverride: true,
						Check: structs.CheckType{
							CheckID:           "g",
							Name:              "h",
							Status:            "i",
							Notes:             "j",
							Script:            "k",
							HTTP:              "l",
							Header:            map[string][]string{"a": []string{"b"}, "c": []string{"d", "e"}},
							Method:            "x",
							TCP:               "m",
							DockerContainerID: "n",
							Shell:             "o",
							TLSSkipVerify:     true,
							Interval:          2 * time.Second,
							Timeout:           3 * time.Second,
							TTL:               4 * time.Second,
							DeregisterCriticalServiceAfter: 5 * time.Second,
						},
					},
				},
			},
		},
		{
			desc: "single service with multiple checks",
			in: `{
					"service": {
						"ID": "a",
						"Name": "b",
						"Tags": ["c", "d"],
						"Address": "e",
						"Token": "f",
						"Port": 123,
						"EnableTagOverride": true,
						"Checks": [
							{
								"CheckID": "g",
								"Name": "h",
								"Status": "i",
								"Notes": "j",
								"Script": "k",
								"HTTP": "l",
								"Header": {"a":["b"], "c":["d", "e"]},
								"Method": "x",
								"TCP": "m",
								"DockerContainerID": "n",
								"Shell": "o",
								"TLSSkipVerify": true,
								"Interval": "2s",
								"Timeout": "3s",
								"TTL": "4s",
								"DeregisterCriticalServiceAfter": "5s"
							},
							{
								"CheckID": "gg",
								"Name": "hh",
								"Status": "ii",
								"Notes": "jj",
								"Script": "kk",
								"HTTP": "ll",
								"Header": {"aa":["bb"], "cc":["dd", "ee"]},
								"Method": "xx",
								"TCP": "mm",
								"DockerContainerID": "nn",
								"Shell": "oo",
								"TLSSkipVerify": false,
								"Interval": "22s",
								"Timeout": "33s",
								"TTL": "44s",
								"DeregisterCriticalServiceAfter": "55s"
							}
						]
					}
				}`,
			c: &Config{
				Services: []*structs.ServiceDefinition{
					&structs.ServiceDefinition{
						ID:                "a",
						Name:              "b",
						Tags:              []string{"c", "d"},
						Address:           "e",
						Port:              123,
						Token:             "f",
						EnableTagOverride: true,
						Checks: []*structs.CheckType{
							{
								CheckID:           "g",
								Name:              "h",
								Status:            "i",
								Notes:             "j",
								Script:            "k",
								HTTP:              "l",
								Header:            map[string][]string{"a": []string{"b"}, "c": []string{"d", "e"}},
								Method:            "x",
								TCP:               "m",
								DockerContainerID: "n",
								Shell:             "o",
								TLSSkipVerify:     true,
								Interval:          2 * time.Second,
								Timeout:           3 * time.Second,
								TTL:               4 * time.Second,
								DeregisterCriticalServiceAfter: 5 * time.Second,
							},
							{
								CheckID:           "gg",
								Name:              "hh",
								Status:            "ii",
								Notes:             "jj",
								Script:            "kk",
								HTTP:              "ll",
								Header:            map[string][]string{"aa": []string{"bb"}, "cc": []string{"dd", "ee"}},
								Method:            "xx",
								TCP:               "mm",
								DockerContainerID: "nn",
								Shell:             "oo",
								TLSSkipVerify:     false,
								Interval:          22 * time.Second,
								Timeout:           33 * time.Second,
								TTL:               44 * time.Second,
								DeregisterCriticalServiceAfter: 55 * time.Second,
							},
						},
					},
				},
			},
		},
		{
			desc: "multiple services with check",
			in: `{
					"services": [
						{
							"ID": "a",
							"Name": "b",
							"Tags": ["c", "d"],
							"Address": "e",
							"Token": "f",
							"Port": 123,
							"EnableTagOverride": true,
							"Check": {
								"CheckID": "g",
								"Name": "h",
								"Status": "i",
								"Notes": "j",
								"Script": "k",
								"HTTP": "l",
								"Header": {"a":["b"], "c":["d", "e"]},
								"Method": "x",
								"TCP": "m",
								"DockerContainerID": "n",
								"Shell": "o",
								"TLSSkipVerify": true,
								"Interval": "2s",
								"Timeout": "3s",
								"TTL": "4s",
								"DeregisterCriticalServiceAfter": "5s"
							}
						},
						{
							"ID": "aa",
							"Name": "bb",
							"Tags": ["cc", "dd"],
							"Address": "ee",
							"Token": "ff",
							"Port": 246,
							"EnableTagOverride": false,
							"Check": {
								"CheckID": "gg",
								"Name": "hh",
								"Status": "ii",
								"Notes": "jj",
								"Script": "kk",
								"HTTP": "ll",
								"Header": {"aa":["bb"], "cc":["dd", "ee"]},
								"Method": "xx",
								"TCP": "mm",
								"DockerContainerID": "nn",
								"Shell": "oo",
								"TLSSkipVerify": false,
								"Interval": "22s",
								"Timeout": "33s",
								"TTL": "44s",
								"DeregisterCriticalServiceAfter": "55s"
							}
						}
					]
				}`,
			c: &Config{
				Services: []*structs.ServiceDefinition{
					&structs.ServiceDefinition{
						ID:                "a",
						Name:              "b",
						Tags:              []string{"c", "d"},
						Address:           "e",
						Port:              123,
						Token:             "f",
						EnableTagOverride: true,
						Check: structs.CheckType{
							CheckID:           "g",
							Name:              "h",
							Status:            "i",
							Notes:             "j",
							Script:            "k",
							HTTP:              "l",
							Header:            map[string][]string{"a": []string{"b"}, "c": []string{"d", "e"}},
							Method:            "x",
							TCP:               "m",
							DockerContainerID: "n",
							Shell:             "o",
							TLSSkipVerify:     true,
							Interval:          2 * time.Second,
							Timeout:           3 * time.Second,
							TTL:               4 * time.Second,
							DeregisterCriticalServiceAfter: 5 * time.Second,
						},
					},
					&structs.ServiceDefinition{
						ID:                "aa",
						Name:              "bb",
						Tags:              []string{"cc", "dd"},
						Address:           "ee",
						Port:              246,
						Token:             "ff",
						EnableTagOverride: false,
						Check: structs.CheckType{
							CheckID:           "gg",
							Name:              "hh",
							Status:            "ii",
							Notes:             "jj",
							Script:            "kk",
							HTTP:              "ll",
							Header:            map[string][]string{"aa": []string{"bb"}, "cc": []string{"dd", "ee"}},
							Method:            "xx",
							TCP:               "mm",
							DockerContainerID: "nn",
							Shell:             "oo",
							TLSSkipVerify:     false,
							Interval:          22 * time.Second,
							Timeout:           33 * time.Second,
							TTL:               44 * time.Second,
							DeregisterCriticalServiceAfter: 55 * time.Second,
						},
					},
				},
			},
		},

		{
			desc: "single check",
			in: `{
					"check": {
						"id": "a",
						"name": "b",
						"notes": "c",
						"service_id": "x",
						"token": "y",
						"status": "z",
						"script": "d",
						"shell": "e",
						"http": "f",
						"Header": {"a":["b"], "c":["d", "e"]},
						"Method": "x",
						"tcp": "g",
						"docker_container_id": "h",
						"tls_skip_verify": true,
						"interval": "2s",
						"timeout": "3s",
						"ttl": "4s",
						"deregister_critical_service_after": "5s"
					}
				}`,
			c: &Config{
				Checks: []*structs.CheckDefinition{
					&structs.CheckDefinition{
						ID:                "a",
						Name:              "b",
						Notes:             "c",
						ServiceID:         "x",
						Token:             "y",
						Status:            "z",
						Script:            "d",
						Shell:             "e",
						HTTP:              "f",
						Header:            map[string][]string{"a": []string{"b"}, "c": []string{"d", "e"}},
						Method:            "x",
						TCP:               "g",
						DockerContainerID: "h",
						TLSSkipVerify:     true,
						Interval:          2 * time.Second,
						Timeout:           3 * time.Second,
						TTL:               4 * time.Second,
						DeregisterCriticalServiceAfter: 5 * time.Second,
					},
				},
			},
		},
		{
			desc: "multiple checks",
			in: `{
					"checks": [
						{
							"id": "a",
							"name": "b",
							"notes": "c",
							"service_id": "d",
							"token": "e",
							"status": "f",
							"script": "g",
							"shell": "h",
							"http": "i",
							"Header": {"a":["b"], "c":["d", "e"]},
							"Method": "x",
							"tcp": "j",
							"docker_container_id": "k",
							"tls_skip_verify": true,
							"interval": "2s",
							"timeout": "3s",
							"ttl": "4s",
							"deregister_critical_service_after": "5s"
						},
						{
							"id": "aa",
							"name": "bb",
							"notes": "cc",
							"service_id": "dd",
							"token": "ee",
							"status": "ff",
							"script": "gg",
							"shell": "hh",
							"http": "ii",
							"Header": {"aa":["bb"], "cc":["dd", "ee"]},
							"Method": "xx",
							"tcp": "jj",
							"docker_container_id": "kk",
							"tls_skip_verify": false,
							"interval": "22s",
							"timeout": "33s",
							"ttl": "44s",
							"deregister_critical_service_after": "55s"
						}
					]
				}`,
			c: &Config{
				Checks: []*structs.CheckDefinition{
					&structs.CheckDefinition{
						ID:                "a",
						Name:              "b",
						Notes:             "c",
						ServiceID:         "d",
						Token:             "e",
						Status:            "f",
						Script:            "g",
						Shell:             "h",
						HTTP:              "i",
						Header:            map[string][]string{"a": []string{"b"}, "c": []string{"d", "e"}},
						Method:            "x",
						TCP:               "j",
						DockerContainerID: "k",
						TLSSkipVerify:     true,
						Interval:          2 * time.Second,
						Timeout:           3 * time.Second,
						TTL:               4 * time.Second,
						DeregisterCriticalServiceAfter: 5 * time.Second,
					},
					&structs.CheckDefinition{
						ID:                "aa",
						Name:              "bb",
						Notes:             "cc",
						ServiceID:         "dd",
						Token:             "ee",
						Status:            "ff",
						Script:            "gg",
						Shell:             "hh",
						HTTP:              "ii",
						Header:            map[string][]string{"aa": []string{"bb"}, "cc": []string{"dd", "ee"}},
						Method:            "xx",
						TCP:               "jj",
						DockerContainerID: "kk",
						TLSSkipVerify:     false,
						Interval:          22 * time.Second,
						Timeout:           33 * time.Second,
						TTL:               44 * time.Second,
						DeregisterCriticalServiceAfter: 55 * time.Second,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		desc := tt.desc
		if desc == "" {
			desc = tt.in
		}
		t.Run(desc, func(t *testing.T) {
			c, err := DecodeConfig(strings.NewReader(tt.in))
			if got, want := err, tt.err; !reflect.DeepEqual(got, want) {
				t.Fatalf("got error %v want %v", got, want)
			}
			got, want := c, tt.c
			verify.Values(t, "", got, want)
		})
	}
}

func TestDecodeConfig_ACLTokenPreference(t *testing.T) {
	tests := []struct {
		in  string
		tok string
	}{
		{
			in:  `{}`,
			tok: "",
		},
		{
			in:  `{"acl_token":"a"}`,
			tok: "a",
		},
		{
			in:  `{"acl_token":"a","acl_agent_token":"b"}`,
			tok: "b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			c, err := DecodeConfig(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("got error %v want nil", err)
			}
			if got, want := c.GetTokenForAgent(), tt.tok; got != want {
				t.Fatalf("got token for agent %q want %q", got, want)
			}
		})
	}
}

func TestDecodeConfig_VerifyUniqueListeners(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		in   string
		err  error
	}{
		{
			"http_dns1",
			`{"addresses": {"http": "0.0.0.0", "dns": "127.0.0.1"}, "ports": {"dns": 8000}}`,
			nil,
		},
		{
			"http_dns IP identical",
			`{"addresses": {"http": "0.0.0.0", "dns": "0.0.0.0"}, "ports": {"http": 8000, "dns": 8000}}`,
			errors.New("HTTP address already configured for DNS"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c, err := DecodeConfig(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("got error %v want nil", err)
			}

			err = c.VerifyUniqueListeners()
			if got, want := err, tt.err; !reflect.DeepEqual(got, want) {
				t.Fatalf("got error %v want %v", got, want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	// ACL flag for Consul version 0.8 features (broken out since we will
	// eventually remove this).
	config := DefaultConfig()
	if *config.ACLEnforceVersion8 != true {
		t.Fatalf("bad: %#v", config)
	}

	// Remote exec is disabled by default.
	if *config.DisableRemoteExec != true {
		t.Fatalf("bad: %#v", config)
	}
}

func TestMergeConfig(t *testing.T) {
	t.Parallel()
	a := &Config{
		Bootstrap:              false,
		BootstrapExpect:        0,
		Datacenter:             "dc1",
		DataDir:                "/tmp/foo",
		Domain:                 "basic",
		LogLevel:               "debug",
		NodeID:                 "bar",
		NodeName:               "foo",
		ClientAddr:             "127.0.0.1",
		BindAddr:               "127.0.0.1",
		AdvertiseAddr:          "127.0.0.1",
		Server:                 false,
		LeaveOnTerm:            new(bool),
		SkipLeaveOnInt:         new(bool),
		EnableDebug:            false,
		CheckUpdateIntervalRaw: "8m",
		RetryIntervalRaw:       "10s",
		RetryIntervalWanRaw:    "10s",
		RetryJoinEC2: RetryJoinEC2{
			Region:          "us-east-1",
			TagKey:          "Key1",
			TagValue:        "Value1",
			AccessKeyID:     "nope",
			SecretAccessKey: "nope",
		},
		Telemetry: Telemetry{
			DisableHostname: false,
			StatsdAddr:      "nope",
			StatsiteAddr:    "nope",
			StatsitePrefix:  "nope",
			DogStatsdAddr:   "nope",
			DogStatsdTags:   []string{"nope"},
		},
		Meta: map[string]string{
			"key": "value1",
		},
	}

	b := &Config{
		Performance: Performance{
			RaftMultiplier: 99,
		},
		Bootstrap:       true,
		BootstrapExpect: 3,
		Datacenter:      "dc2",
		DataDir:         "/tmp/bar",
		DNSRecursors:    []string{"127.0.0.2:1001"},
		DNSConfig: DNSConfig{
			AllowStale:         Bool(false),
			EnableTruncate:     true,
			DisableCompression: true,
			MaxStale:           30 * time.Second,
			NodeTTL:            10 * time.Second,
			ServiceTTL: map[string]time.Duration{
				"api": 10 * time.Second,
			},
			UDPAnswerLimit:  4,
			RecursorTimeout: 30 * time.Second,
		},
		Domain:            "other",
		LogLevel:          "info",
		NodeID:            "bar",
		DisableHostNodeID: Bool(false),
		NodeName:          "baz",
		ClientAddr:        "127.0.0.2",
		BindAddr:          "127.0.0.2",
		AdvertiseAddr:     "127.0.0.2",
		AdvertiseAddrWan:  "127.0.0.2",
		Ports: PortConfig{
			DNS:     1,
			HTTP:    2,
			SerfLan: 4,
			SerfWan: 5,
			Server:  6,
			HTTPS:   7,
		},
		Addresses: AddressConfig{
			DNS:   "127.0.0.1",
			HTTP:  "127.0.0.2",
			HTTPS: "127.0.0.4",
		},
		Server:         true,
		LeaveOnTerm:    Bool(true),
		SkipLeaveOnInt: Bool(true),
		RaftProtocol:   3,
		Autopilot: Autopilot{
			CleanupDeadServers:      Bool(true),
			LastContactThreshold:    Duration(time.Duration(10)),
			MaxTrailingLogs:         Uint64(10),
			ServerStabilizationTime: Duration(time.Duration(100)),
		},
		EnableDebug:            true,
		VerifyIncoming:         true,
		VerifyOutgoing:         true,
		CAFile:                 "test/ca.pem",
		CertFile:               "test/cert.pem",
		KeyFile:                "test/key.pem",
		TLSMinVersion:          "tls12",
		Checks:                 []*structs.CheckDefinition{nil},
		Services:               []*structs.ServiceDefinition{nil},
		StartJoin:              []string{"1.1.1.1"},
		StartJoinWan:           []string{"1.1.1.1"},
		EnableUI:               true,
		UIDir:                  "/opt/consul-ui",
		EnableSyslog:           true,
		RejoinAfterLeave:       true,
		RetryJoin:              []string{"1.1.1.1"},
		RetryIntervalRaw:       "10s",
		RetryInterval:          10 * time.Second,
		RetryJoinWan:           []string{"1.1.1.1"},
		RetryIntervalWanRaw:    "10s",
		RetryIntervalWan:       10 * time.Second,
		ReconnectTimeoutLanRaw: "24h",
		ReconnectTimeoutLan:    24 * time.Hour,
		ReconnectTimeoutWanRaw: "36h",
		ReconnectTimeoutWan:    36 * time.Hour,
		CheckUpdateInterval:    8 * time.Minute,
		CheckUpdateIntervalRaw: "8m",
		ACLToken:               "1111",
		ACLAgentMasterToken:    "2222",
		ACLAgentToken:          "3333",
		ACLMasterToken:         "4444",
		ACLDatacenter:          "dc2",
		ACLTTL:                 15 * time.Second,
		ACLTTLRaw:              "15s",
		ACLDownPolicy:          "deny",
		ACLDefaultPolicy:       "deny",
		ACLReplicationToken:    "8765309",
		ACLEnforceVersion8:     Bool(true),
		Watches: []map[string]interface{}{
			map[string]interface{}{
				"type":    "keyprefix",
				"prefix":  "foo/",
				"handler": "foobar",
			},
		},
		DisableRemoteExec: Bool(true),
		Telemetry: Telemetry{
			StatsiteAddr:    "127.0.0.1:7250",
			StatsitePrefix:  "stats_prefix",
			StatsdAddr:      "127.0.0.1:7251",
			DisableHostname: true,
			DogStatsdAddr:   "127.0.0.1:7254",
			DogStatsdTags:   []string{"tag_1:val_1", "tag_2:val_2"},
		},
		Meta: map[string]string{
			"key": "value2",
		},
		DisableUpdateCheck:        true,
		DisableAnonymousSignature: true,
		HTTPConfig: HTTPConfig{
			ResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
		UnixSockets: UnixSocketConfig{
			UnixSocketPermissions{
				Usr:   "500",
				Grp:   "500",
				Perms: "0700",
			},
		},
		RetryJoinEC2: RetryJoinEC2{
			Region:          "us-east-2",
			TagKey:          "Key2",
			TagValue:        "Value2",
			AccessKeyID:     "foo",
			SecretAccessKey: "bar",
		},
		SessionTTLMinRaw: "1000s",
		SessionTTLMin:    1000 * time.Second,
		AdvertiseAddrs: AdvertiseAddrsConfig{
			SerfLan:    &net.TCPAddr{},
			SerfLanRaw: "127.0.0.5:1231",
			SerfWan:    &net.TCPAddr{},
			SerfWanRaw: "127.0.0.5:1232",
			RPC:        &net.TCPAddr{},
			RPCRaw:     "127.0.0.5:1233",
		},
	}

	c := MergeConfig(a, b)

	if !reflect.DeepEqual(c, b) {
		t.Fatalf("should be equal %#v %#v", c, b)
	}
}

func TestReadConfigPaths_badPath(t *testing.T) {
	t.Parallel()
	_, err := ReadConfigPaths([]string{"/i/shouldnt/exist/ever/rainbows"})
	if err == nil {
		t.Fatal("should have err")
	}
}

func TestReadConfigPaths_file(t *testing.T) {
	t.Parallel()
	tf := testutil.TempFile(t, "consul")
	tf.Write([]byte(`{"node_name":"bar"}`))
	tf.Close()
	defer os.Remove(tf.Name())

	config, err := ReadConfigPaths([]string{tf.Name()})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "bar" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestReadConfigPaths_dir(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	err := ioutil.WriteFile(filepath.Join(td, "a.json"),
		[]byte(`{"node_name": "bar"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = ioutil.WriteFile(filepath.Join(td, "b.json"),
		[]byte(`{"node_name": "baz"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// A non-json file, shouldn't be read
	err = ioutil.WriteFile(filepath.Join(td, "c"),
		[]byte(`{"node_name": "bad"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// An empty file shouldn't be read
	err = ioutil.WriteFile(filepath.Join(td, "d.json"),
		[]byte{}, 0664)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	config, err := ReadConfigPaths([]string{td})
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if config.NodeName != "baz" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestUnixSockets(t *testing.T) {
	t.Parallel()
	if p := socketPath("unix:///path/to/socket"); p != "/path/to/socket" {
		t.Fatalf("bad: %q", p)
	}
	if p := socketPath("notunix://blah"); p != "" {
		t.Fatalf("bad: %q", p)
	}
}

func TestCheckDefinitionToCheckType(t *testing.T) {
	t.Parallel()
	got := &structs.CheckDefinition{
		ID:     "id",
		Name:   "name",
		Status: "green",
		Notes:  "notes",

		ServiceID:         "svcid",
		Token:             "tok",
		Script:            "/bin/foo",
		HTTP:              "someurl",
		TCP:               "host:port",
		Interval:          1 * time.Second,
		DockerContainerID: "abc123",
		Shell:             "/bin/ksh",
		TLSSkipVerify:     true,
		Timeout:           2 * time.Second,
		TTL:               3 * time.Second,
		DeregisterCriticalServiceAfter: 4 * time.Second,
	}
	want := &structs.CheckType{
		CheckID: "id",
		Name:    "name",
		Status:  "green",
		Notes:   "notes",

		Script:            "/bin/foo",
		HTTP:              "someurl",
		TCP:               "host:port",
		Interval:          1 * time.Second,
		DockerContainerID: "abc123",
		Shell:             "/bin/ksh",
		TLSSkipVerify:     true,
		Timeout:           2 * time.Second,
		TTL:               3 * time.Second,
		DeregisterCriticalServiceAfter: 4 * time.Second,
	}
	verify.Values(t, "", got.CheckType(), want)
}
