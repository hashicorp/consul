package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics/prometheus"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/consul"
	consulrate "github.com/hashicorp/consul/agent/consul/rate"
	hcpconfig "github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

// testCase used to test different config loading and flag parsing scenarios.
type testCase struct {
	desc             string
	args             []string
	setup            func() // TODO: accept a testing.T instead of panic
	expected         func(rt *RuntimeConfig)
	expectedErr      string
	expectedWarnings []string
	opts             LoadOpts
	json             []string
	hcl              []string
}

func (tc testCase) source(format string) []string {
	if format == "hcl" {
		return tc.hcl
	}
	return tc.json
}

var defaultGrpcTlsAddr = net.TCPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:8503"))

// TestConfigFlagsAndEdgecases tests the command line flags and
// edgecases for the config parsing. It provides a test structure which
// checks for warnings on deprecated fields and flags.  These tests
// should check one option at a time if possible
func TestLoad_IntegrationWithFlags(t *testing.T) {
	dataDir := testutil.TempDir(t, "config")

	run := func(t *testing.T, tc testCase) {
		t.Helper()
		if len(tc.json) == 0 && len(tc.hcl) == 0 {
			runCase(t, tc.desc, tc.run("", dataDir))
			return
		}

		for _, format := range []string{"json", "hcl"} {
			name := fmt.Sprintf("%v_%v", tc.desc, format)
			runCase(t, name, tc.run(format, dataDir))
		}
	}

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	// ------------------------------------------------------------
	// cmd line flags
	//

	run(t, testCase{
		desc: "-advertise",
		args: []string{
			`-advertise=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-advertise-wan",
		args: []string{
			`-advertise-wan=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "10.0.0.1",
				"lan_ipv4": "10.0.0.1",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-advertise and -advertise-wan",
		args: []string{
			`-advertise=1.2.3.4`,
			`-advertise-wan=5.6.7.8`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("5.6.7.8")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("5.6.7.8:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "5.6.7.8",
				"wan_ipv4": "5.6.7.8",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-bind",
		args: []string{
			`-bind=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.BindAddr = ipAddr("1.2.3.4")
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:8300")
			rt.RPCBindAddr = tcpAddr("1.2.3.4:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.SerfBindAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfBindAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-bootstrap",
		args: []string{
			`-bootstrap`,
			`-server`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Bootstrap = true
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
		expectedWarnings: []string{"bootstrap = true: do not enable unless necessary"},
	})
	run(t, testCase{
		desc: "-bootstrap-expect",
		args: []string{
			`-bootstrap-expect=3`,
			`-server`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.BootstrapExpect = 3
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
		expectedWarnings: []string{"bootstrap_expect > 0: expecting 3 servers"},
	})
	run(t, testCase{
		desc: "-client",
		args: []string{
			`-client=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.TLS.ServerMode = false
			rt.ClientAddrs = []*net.IPAddr{ipAddr("1.2.3.4")}
			rt.DNSAddrs = []net.Addr{tcpAddr("1.2.3.4:8600"), udpAddr("1.2.3.4:8600")}
			rt.HTTPAddrs = []net.Addr{tcpAddr("1.2.3.4:8500")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-config-dir",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-dir`, filepath.Join(dataDir, "conf.d"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf.d/conf.json"), []byte(`{"datacenter":"a"}`))
		},
	})
	run(t, testCase{
		desc: "-config-file json",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-file`, filepath.Join(dataDir, "conf.json"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf.json"), []byte(`{"datacenter":"a"}`))
		},
	})
	run(t, testCase{
		desc: "-config-file hcl and json",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-file`, filepath.Join(dataDir, "conf.hcl"),
			`-config-file`, filepath.Join(dataDir, "conf.json"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "b"
			rt.PrimaryDatacenter = "b"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf.hcl"), []byte(`datacenter = "a"`))
			writeFile(filepath.Join(dataDir, "conf.json"), []byte(`{"datacenter":"b"}`))
		},
	})
	run(t, testCase{
		desc: "-data-dir empty",
		args: []string{
			`-data-dir=`,
		},
		expectedErr: "data_dir cannot be empty",
	})
	run(t, testCase{
		desc: "-data-dir non-directory",
		args: []string{
			`-data-dir=runtime_test.go`,
		},
		expectedErr: `data_dir "runtime_test.go" is not a directory`,
	})
	run(t, testCase{
		desc: "-datacenter",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-datacenter empty",
		args: []string{
			`-datacenter=`,
			`-data-dir=` + dataDir,
		},
		expectedErr: "datacenter cannot be empty",
	})
	run(t, testCase{
		desc: "-dev",
		args: []string{
			`-dev`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("127.0.0.1")
			rt.AdvertiseAddrWAN = ipAddr("127.0.0.1")
			rt.BindAddr = ipAddr("127.0.0.1")
			rt.ConnectEnabled = true
			rt.DevMode = true
			rt.DisableAnonymousSignature = true
			rt.DisableKeyringFile = true
			rt.EnableDebug = true
			rt.UIConfig.Enabled = true
			rt.LeaveOnTerm = false
			rt.Logging.LogLevel = "DEBUG"
			rt.RPCAdvertiseAddr = tcpAddr("127.0.0.1:8300")
			rt.RPCBindAddr = tcpAddr("127.0.0.1:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("127.0.0.1:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("127.0.0.1:8302")
			rt.SerfBindAddrLAN = tcpAddr("127.0.0.1:8301")
			rt.SerfBindAddrWAN = tcpAddr("127.0.0.1:8302")
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.SkipLeaveOnInt = true
			rt.TaggedAddresses = map[string]string{
				"lan":      "127.0.0.1",
				"lan_ipv4": "127.0.0.1",
				"wan":      "127.0.0.1",
				"wan_ipv4": "127.0.0.1",
			}
			rt.ConsulCoordinateUpdatePeriod = 100 * time.Millisecond
			rt.ConsulRaftElectionTimeout = 52 * time.Millisecond
			rt.ConsulRaftHeartbeatTimeout = 35 * time.Millisecond
			rt.ConsulRaftLeaderLeaseTimeout = 20 * time.Millisecond
			rt.GossipLANGossipInterval = 100 * time.Millisecond
			rt.GossipLANProbeInterval = 100 * time.Millisecond
			rt.GossipLANProbeTimeout = 100 * time.Millisecond
			rt.GossipLANSuspicionMult = 3
			rt.GossipWANGossipInterval = 100 * time.Millisecond
			rt.GossipWANProbeInterval = 100 * time.Millisecond
			rt.GossipWANProbeTimeout = 100 * time.Millisecond
			rt.GossipWANSuspicionMult = 3
			rt.ConsulServerHealthInterval = 10 * time.Millisecond
			rt.GRPCPort = 8502
			rt.GRPCAddrs = []net.Addr{tcpAddr("127.0.0.1:8502")}
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "-disable-host-node-id",
		args: []string{
			`-disable-host-node-id`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DisableHostNodeID = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-disable-keyring-file",
		args: []string{
			`-disable-keyring-file`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DisableKeyringFile = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-dns-port",
		args: []string{
			`-dns-port=123`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DNSPort = 123
			rt.DNSAddrs = []net.Addr{tcpAddr("127.0.0.1:123"), udpAddr("127.0.0.1:123")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-domain",
		args: []string{
			`-domain=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DNSDomain = "a"
			rt.TLS.Domain = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-alt-domain",
		args: []string{
			`-alt-domain=alt`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DNSAltDomain = "alt"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-alt-domain can't be prefixed by DC",
		args: []string{
			`-datacenter=a`,
			`-alt-domain=a.alt`,
			`-data-dir=` + dataDir,
		},
		expectedErr: "alt_domain cannot start with {service,connect,node,query,addr,a}",
	})
	run(t, testCase{
		desc: "-alt-domain can't be prefixed by service",
		args: []string{
			`-alt-domain=service.alt`,
			`-data-dir=` + dataDir,
		},
		expectedErr: "alt_domain cannot start with {service,connect,node,query,addr,dc1}",
	})
	run(t, testCase{
		desc: "-alt-domain can be prefixed by non-keywords",
		args: []string{
			`-alt-domain=mydomain.alt`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DNSAltDomain = "mydomain.alt"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-alt-domain can't be prefixed by DC",
		args: []string{
			`-alt-domain=dc1.alt`,
			`-data-dir=` + dataDir,
		},
		expectedErr: "alt_domain cannot start with {service,connect,node,query,addr,dc1}",
	})
	run(t, testCase{
		desc: "-enable-script-checks",
		args: []string{
			`-enable-script-checks`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.EnableLocalScriptChecks = true
			rt.EnableRemoteScriptChecks = true
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{remoteScriptCheckSecurityWarning},
	})
	run(t, testCase{
		desc: "-encrypt",
		args: []string{
			`-encrypt=pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.EncryptKey = "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s="
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-config-format disabled, skip unknown files",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-dir`, filepath.Join(dataDir, "conf"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf", "valid.json"), []byte(`{"datacenter":"a"}`))
			writeFile(filepath.Join(dataDir, "conf", "invalid.skip"), []byte(`NOPE`))
		},
		expectedWarnings: []string{
			"skipping file " + filepath.Join(dataDir, "conf", "invalid.skip") + ", extension must be .hcl or .json, or config format must be set",
		},
	})
	run(t, testCase{
		desc: "-config-format=json",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-format=json`,
			`-config-file`, filepath.Join(dataDir, "conf"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf"), []byte(`{"datacenter":"a"}`))
		},
	})
	run(t, testCase{
		desc: "-config-format=hcl",
		args: []string{
			`-data-dir=` + dataDir,
			`-config-format=hcl`,
			`-config-file`, filepath.Join(dataDir, "conf"),
		},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
		setup: func() {
			writeFile(filepath.Join(dataDir, "conf"), []byte(`datacenter = "a"`))
		},
	})
	run(t, testCase{
		desc: "-http-port",
		args: []string{
			`-http-port=123`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.HTTPPort = 123
			rt.HTTPAddrs = []net.Addr{tcpAddr("127.0.0.1:123")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-join",
		args: []string{
			`-join=a`,
			`-join=b`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinLAN = []string{"a", "b"}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			deprecatedFlagWarning("-join", "-retry-join"),
		},
	})
	run(t, testCase{
		desc: "-join-wan",
		args: []string{
			`-join-wan=a`,
			`-join-wan=b`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinWAN = []string{"a", "b"}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			deprecatedFlagWarning("-join-wan", "-retry-join-wan"),
		},
	})
	run(t, testCase{
		desc: "-log-level",
		args: []string{
			`-log-level=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Logging.LogLevel = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-log-json",
		args: []string{
			`-log-json`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Logging.LogJSON = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-log-rotate-max-files",
		args: []string{
			`-log-rotate-max-files=2`,
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "log_rotate_max_files": 2 }`},
		hcl:  []string{`log_rotate_max_files = 2`},
		expected: func(rt *RuntimeConfig) {
			rt.Logging.LogRotateMaxFiles = 2
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-node",
		args: []string{
			`-node=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.NodeName = "a"
			rt.TLS.NodeName = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-node-id",
		args: []string{
			`-node-id=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.NodeID = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-node-meta",
		args: []string{
			`-node-meta=a:b`,
			`-node-meta=c:d`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.NodeMeta = map[string]string{"a": "b", "c": "d"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-non-voting-server",
		args: []string{
			`-non-voting-server`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.ReadReplica = true
			rt.DataDir = dataDir
		},
		expectedWarnings: enterpriseReadReplicaWarnings,
	})
	run(t, testCase{
		desc: "-pid-file",
		args: []string{
			`-pid-file=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.PidFile = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-primary-gateway",
		args: []string{
			`-server`,
			`-datacenter=dc2`,
			`-primary-gateway=a`,
			`-primary-gateway=b`,
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "primary_datacenter": "dc1" }`},
		hcl:  []string{`primary_datacenter = "dc1"`},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "dc2"
			rt.PrimaryDatacenter = "dc1"
			rt.PrimaryGateways = []string{"a", "b"}
			rt.DataDir = dataDir
			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "-protocol",
		args: []string{
			`-protocol=1`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RPCProtocol = 1
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-raft-protocol",
		args: []string{
			`-raft-protocol=3`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RaftProtocol = 3
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-raft-protocol unsupported",
		args: []string{
			`-raft-protocol=2`,
			`-data-dir=` + dataDir,
		},
		expectedErr: "raft_protocol version 2 is not supported by this version of Consul",
	})
	run(t, testCase{
		desc: "-recursor",
		args: []string{
			`-recursor=1.2.3.4`,
			`-recursor=5.6.7.8`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DNSRecursors = []string{"1.2.3.4", "5.6.7.8"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-rejoin",
		args: []string{
			`-rejoin`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RejoinAfterLeave = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-interval",
		args: []string{
			`-retry-interval=5s`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinIntervalLAN = 5 * time.Second
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-interval-wan",
		args: []string{
			`-retry-interval-wan=5s`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinIntervalWAN = 5 * time.Second
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-join",
		args: []string{
			`-retry-join=a`,
			`-retry-join=b`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinLAN = []string{"a", "b"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-join-wan",
		args: []string{
			`-retry-join-wan=a`,
			`-retry-join-wan=b`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinWAN = []string{"a", "b"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-max",
		args: []string{
			`-retry-max=1`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinMaxAttemptsLAN = 1
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-retry-max-wan",
		args: []string{
			`-retry-max-wan=1`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinMaxAttemptsWAN = 1
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-serf-lan-bind",
		args: []string{
			`-serf-lan-bind=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.SerfBindAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-serf-lan-port",
		args: []string{
			`-serf-lan-port=123`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.SerfPortLAN = 123
			rt.SerfAdvertiseAddrLAN = tcpAddr("10.0.0.1:123")
			rt.SerfBindAddrLAN = tcpAddr("0.0.0.0:123")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-serf-wan-bind",
		args: []string{
			`-serf-wan-bind=1.2.3.4`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.SerfBindAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-serf-wan-port",
		args: []string{
			`-serf-wan-port=123`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.SerfPortWAN = 123
			rt.SerfAdvertiseAddrWAN = tcpAddr("10.0.0.1:123")
			rt.SerfBindAddrWAN = tcpAddr("0.0.0.0:123")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-server",
		args: []string{
			`-server`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "-server-port",
		args: []string{
			`-server-port=123`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.ServerPort = 123
			rt.RPCAdvertiseAddr = tcpAddr("10.0.0.1:123")
			rt.RPCBindAddr = tcpAddr("0.0.0.0:123")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-syslog",
		args: []string{
			`-syslog`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Logging.EnableSyslog = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-ui",
		args: []string{
			`-ui`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.Enabled = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-ui-dir",
		args: []string{
			`-ui-dir=a`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.Dir = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "-ui-content-path",
		args: []string{
			`-ui-content-path=/a/b`,
			`-data-dir=` + dataDir,
		},

		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.ContentPath = "/a/b/"
			rt.DataDir = dataDir
		},
	})

	run(t, testCase{
		desc: "-datacenter empty",
		args: []string{
			`-auto-reload-config`,
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AutoReloadConfig = true
			rt.DataDir = dataDir
		},
	})

	// ------------------------------------------------------------
	// ports and addresses
	//

	run(t, testCase{
		desc: "bind addr any v4",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr":"0.0.0.0" }`},
		hcl:  []string{`bind_addr = "0.0.0.0"`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("10.0.0.1")
			rt.AdvertiseAddrWAN = ipAddr("10.0.0.1")
			rt.BindAddr = ipAddr("0.0.0.0")
			rt.RPCAdvertiseAddr = tcpAddr("10.0.0.1:8300")
			rt.RPCBindAddr = tcpAddr("0.0.0.0:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("10.0.0.1:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("10.0.0.1:8302")
			rt.SerfBindAddrLAN = tcpAddr("0.0.0.0:8301")
			rt.SerfBindAddrWAN = tcpAddr("0.0.0.0:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "10.0.0.1",
				"lan_ipv4": "10.0.0.1",
				"wan":      "10.0.0.1",
				"wan_ipv4": "10.0.0.1",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "bind addr any v6",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr":"::" }`},
		hcl:  []string{`bind_addr = "::"`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("dead:beef::1")
			rt.AdvertiseAddrWAN = ipAddr("dead:beef::1")
			rt.BindAddr = ipAddr("::")
			rt.RPCAdvertiseAddr = tcpAddr("[dead:beef::1]:8300")
			rt.RPCBindAddr = tcpAddr("[::]:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("[dead:beef::1]:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("[dead:beef::1]:8302")
			rt.SerfBindAddrLAN = tcpAddr("[::]:8301")
			rt.SerfBindAddrWAN = tcpAddr("[::]:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "dead:beef::1",
				"lan_ipv6": "dead:beef::1",
				"wan":      "dead:beef::1",
				"wan_ipv6": "dead:beef::1",
			}
			rt.DataDir = dataDir
		},
		opts: LoadOpts{
			getPublicIPv6: func() ([]*net.IPAddr, error) {
				return []*net.IPAddr{ipAddr("dead:beef::1")}, nil
			},
		},
	})
	run(t, testCase{
		desc: "bind addr any and advertise set should not detect",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr":"0.0.0.0", "advertise_addr": "1.2.3.4" }`},
		hcl:  []string{`bind_addr = "0.0.0.0" advertise_addr = "1.2.3.4"`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.BindAddr = ipAddr("0.0.0.0")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:8300")
			rt.RPCBindAddr = tcpAddr("0.0.0.0:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.SerfBindAddrLAN = tcpAddr("0.0.0.0:8301")
			rt.SerfBindAddrWAN = tcpAddr("0.0.0.0:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
		opts: LoadOpts{
			getPrivateIPv4: func() ([]*net.IPAddr, error) {
				return nil, fmt.Errorf("should not detect advertise_addr")
			},
		},
	})
	run(t, testCase{
		desc: "client addr and ports == 0",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr":"0.0.0.0",
					"ports":{}
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					ports {}
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSAddrs = []net.Addr{tcpAddr("0.0.0.0:8600"), udpAddr("0.0.0.0:8600")}
			rt.HTTPAddrs = []net.Addr{tcpAddr("0.0.0.0:8500")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client addr and ports < 0",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr":"0.0.0.0",
					"ports": { "dns":-1, "http":-2, "https":-3, "grpc":-4 }
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					ports { dns = -1 http = -2 https = -3 grpc = -4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSPort = -1
			rt.DNSAddrs = nil
			rt.HTTPPort = -1
			rt.HTTPAddrs = nil
			// HTTPS and gRPC default to disabled so shouldn't be different from
			// default rt.
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client addr and ports > 0",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr":"0.0.0.0",
					"ports":{ "dns": 1, "http": 2, "https": 3, "grpc": 4 }
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					ports { dns = 1 http = 2 https = 3 grpc = 4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSPort = 1
			rt.DNSAddrs = []net.Addr{tcpAddr("0.0.0.0:1"), udpAddr("0.0.0.0:1")}
			rt.HTTPPort = 2
			rt.HTTPAddrs = []net.Addr{tcpAddr("0.0.0.0:2")}
			rt.HTTPSPort = 3
			rt.HTTPSAddrs = []net.Addr{tcpAddr("0.0.0.0:3")}
			rt.GRPCPort = 4
			rt.GRPCAddrs = []net.Addr{tcpAddr("0.0.0.0:4")}
			rt.DataDir = dataDir
		},
	})

	run(t, testCase{
		desc: "client addr, addresses and ports == 0",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr":"0.0.0.0",
					"addresses": { "dns": "1.1.1.1", "http": "2.2.2.2", "https": "3.3.3.3", "grpc": "4.4.4.4" },
					"ports":{}
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					addresses = { dns = "1.1.1.1" http = "2.2.2.2" https = "3.3.3.3" grpc = "4.4.4.4" }
					ports {}
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSAddrs = []net.Addr{tcpAddr("1.1.1.1:8600"), udpAddr("1.1.1.1:8600")}
			rt.HTTPAddrs = []net.Addr{tcpAddr("2.2.2.2:8500")}
			// HTTPS and gRPC default to disabled so shouldn't be different from
			// default rt.
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client addr, addresses and ports < 0",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr":"0.0.0.0",
					"addresses": { "dns": "1.1.1.1", "http": "2.2.2.2", "https": "3.3.3.3", "grpc": "4.4.4.4" },
					"ports": { "dns":-1, "http":-2, "https":-3, "grpc":-4 }
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					addresses = { dns = "1.1.1.1" http = "2.2.2.2" https = "3.3.3.3" grpc = "4.4.4.4" }
					ports { dns = -1 http = -2 https = -3 grpc = -4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSPort = -1
			rt.DNSAddrs = nil
			rt.HTTPPort = -1
			rt.HTTPAddrs = nil
			// HTTPS and gRPC default to disabled so shouldn't be different from
			// default rt.
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client addr, addresses and ports",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr": "0.0.0.0",
					"addresses": { "dns": "1.1.1.1", "http": "2.2.2.2", "https": "3.3.3.3", "grpc": "4.4.4.4" },
					"ports":{ "dns":1, "http":2, "https":3, "grpc":4 }
				}`},
		hcl: []string{`
					client_addr = "0.0.0.0"
					addresses = { dns = "1.1.1.1" http = "2.2.2.2" https = "3.3.3.3" grpc = "4.4.4.4" }
					ports { dns = 1 http = 2 https = 3 grpc = 4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("0.0.0.0")}
			rt.DNSPort = 1
			rt.DNSAddrs = []net.Addr{tcpAddr("1.1.1.1:1"), udpAddr("1.1.1.1:1")}
			rt.HTTPPort = 2
			rt.HTTPAddrs = []net.Addr{tcpAddr("2.2.2.2:2")}
			rt.HTTPSPort = 3
			rt.HTTPSAddrs = []net.Addr{tcpAddr("3.3.3.3:3")}
			rt.GRPCPort = 4
			rt.GRPCAddrs = []net.Addr{tcpAddr("4.4.4.4:4")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client template and ports",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr": "{{ printf \"1.2.3.4 2001:db8::1\" }}",
					"ports":{ "dns":1, "http":2, "https":3, "grpc":4 }
				}`},
		hcl: []string{`
					client_addr = "{{ printf \"1.2.3.4 2001:db8::1\" }}"
					ports { dns = 1 http = 2 https = 3 grpc = 4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("1.2.3.4"), ipAddr("2001:db8::1")}
			rt.DNSPort = 1
			rt.DNSAddrs = []net.Addr{tcpAddr("1.2.3.4:1"), tcpAddr("[2001:db8::1]:1"), udpAddr("1.2.3.4:1"), udpAddr("[2001:db8::1]:1")}
			rt.HTTPPort = 2
			rt.HTTPAddrs = []net.Addr{tcpAddr("1.2.3.4:2"), tcpAddr("[2001:db8::1]:2")}
			rt.HTTPSPort = 3
			rt.HTTPSAddrs = []net.Addr{tcpAddr("1.2.3.4:3"), tcpAddr("[2001:db8::1]:3")}
			rt.GRPCPort = 4
			rt.GRPCAddrs = []net.Addr{tcpAddr("1.2.3.4:4"), tcpAddr("[2001:db8::1]:4")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client, address template and ports",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
					"client_addr": "{{ printf \"1.2.3.4 2001:db8::1\" }}",
					"addresses": {
						"dns": "{{ printf \"1.1.1.1 2001:db8::10 \" }}",
						"http": "{{ printf \"2.2.2.2 unix://http 2001:db8::20 \" }}",
						"https": "{{ printf \"3.3.3.3 unix://https 2001:db8::30 \" }}",
						"grpc": "{{ printf \"4.4.4.4 unix://grpc 2001:db8::40 \" }}"
					},
					"ports":{ "dns":1, "http":2, "https":3, "grpc":4 }
				}`},
		hcl: []string{`
					client_addr = "{{ printf \"1.2.3.4 2001:db8::1\" }}"
					addresses = {
						dns = "{{ printf \"1.1.1.1 2001:db8::10 \" }}"
						http = "{{ printf \"2.2.2.2 unix://http 2001:db8::20 \" }}"
						https = "{{ printf \"3.3.3.3 unix://https 2001:db8::30 \" }}"
						grpc = "{{ printf \"4.4.4.4 unix://grpc 2001:db8::40 \" }}"
					}
					ports { dns = 1 http = 2 https = 3 grpc = 4 }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.ClientAddrs = []*net.IPAddr{ipAddr("1.2.3.4"), ipAddr("2001:db8::1")}
			rt.DNSPort = 1
			rt.DNSAddrs = []net.Addr{tcpAddr("1.1.1.1:1"), tcpAddr("[2001:db8::10]:1"), udpAddr("1.1.1.1:1"), udpAddr("[2001:db8::10]:1")}
			rt.HTTPPort = 2
			rt.HTTPAddrs = []net.Addr{tcpAddr("2.2.2.2:2"), unixAddr("unix://http"), tcpAddr("[2001:db8::20]:2")}
			rt.HTTPSPort = 3
			rt.HTTPSAddrs = []net.Addr{tcpAddr("3.3.3.3:3"), unixAddr("unix://https"), tcpAddr("[2001:db8::30]:3")}
			rt.GRPCPort = 4
			rt.GRPCAddrs = []net.Addr{tcpAddr("4.4.4.4:4"), unixAddr("unix://grpc"), tcpAddr("[2001:db8::40]:4")}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "advertise address lan template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "advertise_addr": "{{ printf \"1.2.3.4\" }}" }`},
		hcl:  []string{`advertise_addr = "{{ printf \"1.2.3.4\" }}"`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "advertise address wan template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "advertise_addr_wan": "{{ printf \"1.2.3.4\" }}" }`},
		hcl:  []string{`advertise_addr_wan = "{{ printf \"1.2.3.4\" }}"`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.TaggedAddresses = map[string]string{
				"lan":      "10.0.0.1",
				"lan_ipv4": "10.0.0.1",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "advertise address lan with ports",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ports": {
					"server": 1000,
					"serf_lan": 2000,
					"serf_wan": 3000
				},
				"advertise_addr": "1.2.3.4"
			}`},
		hcl: []string{`
				ports {
					server = 1000
					serf_lan = 2000
					serf_wan = 3000
				}
				advertise_addr = "1.2.3.4"
			`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.2.3.4")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.RPCAdvertiseAddr = tcpAddr("1.2.3.4:1000")
			rt.RPCBindAddr = tcpAddr("0.0.0.0:1000")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.2.3.4:2000")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:3000")
			rt.SerfBindAddrLAN = tcpAddr("0.0.0.0:2000")
			rt.SerfBindAddrWAN = tcpAddr("0.0.0.0:3000")
			rt.SerfPortLAN = 2000
			rt.SerfPortWAN = 3000
			rt.ServerPort = 1000
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.2.3.4",
				"lan_ipv4": "1.2.3.4",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "advertise address wan with ports",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ports": {
					"server": 1000,
					"serf_lan": 2000,
					"serf_wan": 3000
				},
				"advertise_addr_wan": "1.2.3.4"
			}`},
		hcl: []string{`
				ports {
					server = 1000
					serf_lan = 2000
					serf_wan = 3000
				}
				advertise_addr_wan = "1.2.3.4"
			`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("10.0.0.1")
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.RPCAdvertiseAddr = tcpAddr("10.0.0.1:1000")
			rt.RPCBindAddr = tcpAddr("0.0.0.0:1000")
			rt.SerfAdvertiseAddrLAN = tcpAddr("10.0.0.1:2000")
			rt.SerfAdvertiseAddrWAN = tcpAddr("1.2.3.4:3000")
			rt.SerfBindAddrLAN = tcpAddr("0.0.0.0:2000")
			rt.SerfBindAddrWAN = tcpAddr("0.0.0.0:3000")
			rt.SerfPortLAN = 2000
			rt.SerfPortWAN = 3000
			rt.ServerPort = 1000
			rt.TaggedAddresses = map[string]string{
				"lan":      "10.0.0.1",
				"lan_ipv4": "10.0.0.1",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "allow disabling serf wan port",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ports": {
					"serf_wan": -1
				},
				"advertise_addr_wan": "1.2.3.4"
			}`},
		hcl: []string{`
				ports {
					serf_wan = -1
				}
				advertise_addr_wan = "1.2.3.4"
			`},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrWAN = ipAddr("1.2.3.4")
			rt.SerfAdvertiseAddrWAN = nil
			rt.SerfBindAddrWAN = nil
			rt.TaggedAddresses = map[string]string{
				"lan":      "10.0.0.1",
				"lan_ipv4": "10.0.0.1",
				"wan":      "1.2.3.4",
				"wan_ipv4": "1.2.3.4",
			}
			rt.DataDir = dataDir
			rt.SerfPortWAN = -1
		},
	})
	run(t, testCase{
		desc: "serf bind address lan template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "serf_lan": "{{ printf \"1.2.3.4\" }}" }`},
		hcl:  []string{`serf_lan = "{{ printf \"1.2.3.4\" }}"`},
		expected: func(rt *RuntimeConfig) {
			rt.SerfBindAddrLAN = tcpAddr("1.2.3.4:8301")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "serf bind address wan template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "serf_wan": "{{ printf \"1.2.3.4\" }}" }`},
		hcl:  []string{`serf_wan = "{{ printf \"1.2.3.4\" }}"`},
		expected: func(rt *RuntimeConfig) {
			rt.SerfBindAddrWAN = tcpAddr("1.2.3.4:8302")
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "dns recursor templates with deduplication",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "recursors": [ "{{ printf \"5.6.7.8:9999\" }}", "{{ printf \"1.2.3.4\" }}", "{{ printf \"5.6.7.8:9999\" }}" ] }`},
		hcl:  []string{`recursors = [ "{{ printf \"5.6.7.8:9999\" }}", "{{ printf \"1.2.3.4\" }}", "{{ printf \"5.6.7.8:9999\" }}" ] `},
		expected: func(rt *RuntimeConfig) {
			rt.DNSRecursors = []string{"5.6.7.8:9999", "1.2.3.4"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "start_join address template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "start_join": ["{{ printf \"1.2.3.4 4.3.2.1\" }}"] }`},
		hcl:  []string{`start_join = ["{{ printf \"1.2.3.4 4.3.2.1\" }}"]`},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinLAN = []string{"1.2.3.4", "4.3.2.1"}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			deprecationWarning("start_join", "retry_join"),
		},
	})
	run(t, testCase{
		desc: "start_join_wan address template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "start_join_wan": ["{{ printf \"1.2.3.4 4.3.2.1\" }}"] }`},
		hcl:  []string{`start_join_wan = ["{{ printf \"1.2.3.4 4.3.2.1\" }}"]`},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinWAN = []string{"1.2.3.4", "4.3.2.1"}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			deprecationWarning("start_join_wan", "retry_join_wan"),
		},
	})
	run(t, testCase{
		desc: "retry_join address template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "retry_join": ["{{ printf \"1.2.3.4 4.3.2.1\" }}"] }`},
		hcl:  []string{`retry_join = ["{{ printf \"1.2.3.4 4.3.2.1\" }}"]`},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinLAN = []string{"1.2.3.4", "4.3.2.1"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "retry_join_wan address template",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "retry_join_wan": ["{{ printf \"1.2.3.4 4.3.2.1\" }}"] }`},
		hcl:  []string{`retry_join_wan = ["{{ printf \"1.2.3.4 4.3.2.1\" }}"]`},
		expected: func(rt *RuntimeConfig) {
			rt.RetryJoinWAN = []string{"1.2.3.4", "4.3.2.1"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "min/max ports for dynamic exposed listeners",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ports": {
					"expose_min_port": 1234,
					"expose_max_port": 5678
				}
			}`},
		hcl: []string{`
				ports {
					expose_min_port = 1234
					expose_max_port = 5678
				}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.ExposeMinPort = 1234
			rt.ExposeMaxPort = 5678
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "defaults for dynamic exposed listeners",
		args: []string{`-data-dir=` + dataDir},
		expected: func(rt *RuntimeConfig) {
			rt.ExposeMinPort = 21500
			rt.ExposeMaxPort = 21755
			rt.DataDir = dataDir
		},
	})

	// ------------------------------------------------------------
	// precedence rules
	//

	run(t, testCase{
		desc: "precedence: merge order",
		args: []string{`-data-dir=` + dataDir},
		json: []string{
			`{
						"bootstrap": true,
						"bootstrap_expect": 1,
						"datacenter": "a",
						"start_join": ["a", "b"],
						"node_meta": {"a":"b"}
					}`,
			`{
						"bootstrap": false,
						"bootstrap_expect": 0,
						"datacenter":"b",
						"start_join": ["c", "d"],
						"node_meta": {"a":"c"}
					}`,
		},
		hcl: []string{
			`
					bootstrap = true
					bootstrap_expect = 1
					datacenter = "a"
					start_join = ["a", "b"]
					node_meta = { "a" = "b" }
					`,
			`
					bootstrap = false
					bootstrap_expect = 0
					datacenter = "b"
					start_join = ["c", "d"]
					node_meta = { "a" = "c" }
					`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Bootstrap = false
			rt.BootstrapExpect = 0
			rt.Datacenter = "b"
			rt.PrimaryDatacenter = "b"
			rt.RetryJoinLAN = []string{"a", "b", "c", "d"}
			rt.NodeMeta = map[string]string{"a": "c"}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			// TODO: deduplicate warnings?
			deprecationWarning("start_join", "retry_join"),
			deprecationWarning("start_join", "retry_join"),
		},
	})
	run(t, testCase{
		desc: "precedence: flag before file",
		json: []string{
			`{
						"advertise_addr": "1.2.3.4",
						"advertise_addr_wan": "5.6.7.8",
						"bootstrap":true,
						"bootstrap_expect": 3,
						"datacenter":"a",
						"node_meta": {"a":"b"},
						"recursors":["1.2.3.5", "5.6.7.9"],
						"serf_lan": "a",
						"serf_wan": "a",
						"start_join":["a", "b"]
					}`,
		},
		hcl: []string{`
					advertise_addr = "1.2.3.4"
					advertise_addr_wan = "5.6.7.8"
					bootstrap = true
					bootstrap_expect = 3
					datacenter = "a"
					node_meta = { "a" = "b" }
					recursors = ["1.2.3.5", "5.6.7.9"]
					serf_lan = "a"
					serf_wan = "a"
					start_join = ["a", "b"]
					`,
		},
		args: []string{
			`-advertise=1.1.1.1`,
			`-advertise-wan=2.2.2.2`,
			`-bootstrap=false`,
			`-bootstrap-expect=0`,
			`-datacenter=b`,
			`-data-dir=` + dataDir,
			`-join`, `c`, `-join=d`,
			`-node-meta=a:c`,
			`-recursor`, `1.2.3.6`, `-recursor=5.6.7.10`,
			`-serf-lan-bind=3.3.3.3`,
			`-serf-wan-bind=4.4.4.4`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.AdvertiseAddrLAN = ipAddr("1.1.1.1")
			rt.AdvertiseAddrWAN = ipAddr("2.2.2.2")
			rt.RPCAdvertiseAddr = tcpAddr("1.1.1.1:8300")
			rt.SerfAdvertiseAddrLAN = tcpAddr("1.1.1.1:8301")
			rt.SerfAdvertiseAddrWAN = tcpAddr("2.2.2.2:8302")
			rt.Datacenter = "b"
			rt.PrimaryDatacenter = "b"
			rt.DNSRecursors = []string{"1.2.3.6", "5.6.7.10", "1.2.3.5", "5.6.7.9"}
			rt.NodeMeta = map[string]string{"a": "c"}
			rt.SerfBindAddrLAN = tcpAddr("3.3.3.3:8301")
			rt.SerfBindAddrWAN = tcpAddr("4.4.4.4:8302")
			rt.RetryJoinLAN = []string{"c", "d", "a", "b"}
			rt.TaggedAddresses = map[string]string{
				"lan":      "1.1.1.1",
				"lan_ipv4": "1.1.1.1",
				"wan":      "2.2.2.2",
				"wan_ipv4": "2.2.2.2",
			}
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{
			deprecatedFlagWarning("-join", "-retry-join"),
			deprecationWarning("start_join", "retry_join"),
		},
	})

	// ------------------------------------------------------------
	// transformations
	//

	run(t, testCase{
		desc: "raft performance scaling",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "performance": { "raft_multiplier": 9} }`},
		hcl:  []string{`performance = { raft_multiplier=9 }`},
		expected: func(rt *RuntimeConfig) {
			rt.ConsulRaftElectionTimeout = 9 * 1000 * time.Millisecond
			rt.ConsulRaftHeartbeatTimeout = 9 * 1000 * time.Millisecond
			rt.ConsulRaftLeaderLeaseTimeout = 9 * 500 * time.Millisecond
			rt.DataDir = dataDir
		},
	})

	run(t, testCase{
		desc: "Serf Allowed CIDRS LAN, multiple values from flags",
		args: []string{`-data-dir=` + dataDir, `-serf-lan-allowed-cidrs=127.0.0.0/4`, `-serf-lan-allowed-cidrs=192.168.0.0/24`},
		json: []string{},
		hcl:  []string{},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.SerfAllowedCIDRsLAN = []net.IPNet{*(parseCIDR(t, "127.0.0.0/4")), *(parseCIDR(t, "192.168.0.0/24"))}
		},
	})
	run(t, testCase{
		desc: "Serf Allowed CIDRS LAN/WAN, multiple values from HCL/JSON",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{"serf_lan_allowed_cidrs": ["127.0.0.0/4", "192.168.0.0/24"]}`,
			`{"serf_wan_allowed_cidrs": ["10.228.85.46/25"]}`},
		hcl: []string{`serf_lan_allowed_cidrs=["127.0.0.0/4", "192.168.0.0/24"]`,
			`serf_wan_allowed_cidrs=["10.228.85.46/25"]`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.SerfAllowedCIDRsLAN = []net.IPNet{*(parseCIDR(t, "127.0.0.0/4")), *(parseCIDR(t, "192.168.0.0/24"))}
			rt.SerfAllowedCIDRsWAN = []net.IPNet{*(parseCIDR(t, "10.228.85.46/25"))}
		},
	})
	run(t, testCase{
		desc: "Serf Allowed CIDRS WAN, multiple values from flags",
		args: []string{`-data-dir=` + dataDir, `-serf-wan-allowed-cidrs=192.168.4.0/24`, `-serf-wan-allowed-cidrs=192.168.3.0/24`},
		json: []string{},
		hcl:  []string{},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.SerfAllowedCIDRsWAN = []net.IPNet{*(parseCIDR(t, "192.168.4.0/24")), *(parseCIDR(t, "192.168.3.0/24"))}
		},
	})

	// ------------------------------------------------------------
	// validations
	//

	run(t, testCase{
		desc:        "invalid input",
		args:        []string{`-data-dir=` + dataDir},
		json:        []string{`this is not JSON`},
		hcl:         []string{`*** 0123 this is not HCL`},
		expectedErr: "failed to parse",
	})
	run(t, testCase{
		desc: "datacenter is lower-cased",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "datacenter": "A" }`},
		hcl:  []string{`datacenter = "A"`},
		expected: func(rt *RuntimeConfig) {
			rt.Datacenter = "a"
			rt.PrimaryDatacenter = "a"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "acl_datacenter is lower-cased",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "acl_datacenter": "A" }`},
		hcl:  []string{`acl_datacenter = "A"`},
		expected: func(rt *RuntimeConfig) {
			rt.ACLsEnabled = true
			rt.DataDir = dataDir
			rt.PrimaryDatacenter = "a"
		},
		expectedWarnings: []string{`The 'acl_datacenter' field is deprecated. Use the 'primary_datacenter' field instead.`},
	})
	run(t, testCase{
		desc:             "acl_replication_token enables acl replication",
		args:             []string{`-data-dir=` + dataDir},
		json:             []string{`{ "acl_replication_token": "a" }`},
		hcl:              []string{`acl_replication_token = "a"`},
		expectedWarnings: []string{deprecationWarning("acl_replication_token", "acl.tokens.replication")},
		expected: func(rt *RuntimeConfig) {
			rt.ACLTokens.ACLReplicationToken = "a"
			rt.ACLTokenReplication = true
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "acl.tokens.replace does not enable acl replication",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "acl": { "tokens": { "replication": "a" }}}`},
		hcl:  []string{`acl { tokens { replication = "a"}}`},
		expected: func(rt *RuntimeConfig) {
			rt.ACLTokens.ACLReplicationToken = "a"
			rt.ACLTokenReplication = false
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "acl_enforce_version_8 is deprecated",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "acl_enforce_version_8": true }`},
		hcl:  []string{`acl_enforce_version_8 = true`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{`config key "acl_enforce_version_8" is deprecated and should be removed`},
	})

	run(t, testCase{
		desc: "advertise address detect fails v4",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "0.0.0.0"}`},
		hcl:  []string{`bind_addr = "0.0.0.0"`},
		opts: LoadOpts{
			getPrivateIPv4: func() ([]*net.IPAddr, error) {
				return nil, errors.New("some error")
			},
		},
		expectedErr: "Error detecting private IPv4 address: some error",
	})
	run(t, testCase{
		desc: "advertise address detect none v4",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "0.0.0.0"}`},
		hcl:  []string{`bind_addr = "0.0.0.0"`},
		opts: LoadOpts{
			getPrivateIPv4: func() ([]*net.IPAddr, error) {
				return nil, nil
			},
		},
		expectedErr: "No private IPv4 address found",
	})
	run(t, testCase{
		desc: "advertise address detect multiple v4",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "0.0.0.0"}`},
		hcl:  []string{`bind_addr = "0.0.0.0"`},
		opts: LoadOpts{
			getPrivateIPv4: func() ([]*net.IPAddr, error) {
				return []*net.IPAddr{ipAddr("1.1.1.1"), ipAddr("2.2.2.2")}, nil
			},
		},
		expectedErr: "Multiple private IPv4 addresses found. Please configure one",
	})
	run(t, testCase{
		desc: "advertise address detect fails v6",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "::"}`},
		hcl:  []string{`bind_addr = "::"`},
		opts: LoadOpts{
			getPublicIPv6: func() ([]*net.IPAddr, error) {
				return nil, errors.New("some error")
			},
		},
		expectedErr: "Error detecting public IPv6 address: some error",
	})
	run(t, testCase{
		desc: "advertise address detect none v6",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "::"}`},
		hcl:  []string{`bind_addr = "::"`},
		opts: LoadOpts{
			getPublicIPv6: func() ([]*net.IPAddr, error) {
				return nil, nil
			},
		},
		expectedErr: "No public IPv6 address found",
	})
	run(t, testCase{
		desc: "advertise address detect multiple v6",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "bind_addr": "::"}`},
		hcl:  []string{`bind_addr = "::"`},
		opts: LoadOpts{
			getPublicIPv6: func() ([]*net.IPAddr, error) {
				return []*net.IPAddr{ipAddr("dead:beef::1"), ipAddr("dead:beef::2")}, nil
			},
		},
		expectedErr: "Multiple public IPv6 addresses found. Please configure one",
	})
	run(t, testCase{
		desc: "ae_interval is overridden by NonUserSource",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{ "ae_interval": "-1s" }`},
		hcl:  []string{`ae_interval = "-1s"`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.AEInterval = time.Minute
		},
	})
	run(t, testCase{
		desc: "primary_datacenter invalid",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "primary_datacenter": "%" }`},
		hcl:         []string{`primary_datacenter = "%"`},
		expectedErr: `primary_datacenter can only contain lowercase alphanumeric, - or _ characters.`,
	})
	run(t, testCase{
		desc: "acl_datacenter deprecated",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:             []string{`{ "acl_datacenter": "ab" }`},
		hcl:              []string{`acl_datacenter = "ab"`},
		expectedWarnings: []string{`The 'acl_datacenter' field is deprecated. Use the 'primary_datacenter' field instead.`},
		expected: func(rt *RuntimeConfig) {
			rt.ACLsEnabled = true
			rt.PrimaryDatacenter = "ab"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "autopilot.max_trailing_logs invalid",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "autopilot": { "max_trailing_logs": -1 } }`},
		hcl:         []string{`autopilot = { max_trailing_logs = -1 }`},
		expectedErr: "autopilot.max_trailing_logs cannot be -1. Must be greater than or equal to zero",
	})
	run(t, testCase{
		desc:        "bind_addr cannot be empty",
		args:        []string{`-data-dir=` + dataDir},
		json:        []string{`{ "bind_addr": "" }`},
		hcl:         []string{`bind_addr = ""`},
		expectedErr: "bind_addr cannot be empty",
	})
	run(t, testCase{
		desc:        "bind_addr does not allow multiple addresses",
		args:        []string{`-data-dir=` + dataDir},
		json:        []string{`{ "bind_addr": "1.1.1.1 2.2.2.2" }`},
		hcl:         []string{`bind_addr = "1.1.1.1 2.2.2.2"`},
		expectedErr: "bind_addr cannot contain multiple addresses",
	})
	run(t, testCase{
		desc:        "bind_addr cannot be a unix socket",
		args:        []string{`-data-dir=` + dataDir},
		json:        []string{`{ "bind_addr": "unix:///foo" }`},
		hcl:         []string{`bind_addr = "unix:///foo"`},
		expectedErr: "bind_addr cannot be a unix socket",
	})
	run(t, testCase{
		desc: "bootstrap without server",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "bootstrap": true }`},
		hcl:         []string{`bootstrap = true`},
		expectedErr: "'bootstrap = true' requires 'server = true'",
	})
	run(t, testCase{
		desc: "bootstrap-expect without server",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "bootstrap_expect": 3 }`},
		hcl:         []string{`bootstrap_expect = 3`},
		expectedErr: "'bootstrap_expect > 0' requires 'server = true'",
	})
	run(t, testCase{
		desc: "bootstrap-expect invalid",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "bootstrap_expect": -1 }`},
		hcl:         []string{`bootstrap_expect = -1`},
		expectedErr: "bootstrap_expect cannot be -1. Must be greater than or equal to zero",
	})
	run(t, testCase{
		desc: "bootstrap-expect and dev mode",
		args: []string{
			`-dev`,
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "bootstrap_expect": 3, "server": true }`},
		hcl:         []string{`bootstrap_expect = 3 server = true`},
		expectedErr: "'bootstrap_expect > 0' not allowed in dev mode",
	})
	run(t, testCase{
		desc: "bootstrap-expect and bootstrap",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "bootstrap": true, "bootstrap_expect": 3, "server": true }`},
		hcl:         []string{`bootstrap = true bootstrap_expect = 3 server = true`},
		expectedErr: "'bootstrap_expect > 0' and 'bootstrap = true' are mutually exclusive",
	})
	run(t, testCase{
		desc: "bootstrap-expect=1 equals bootstrap",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "bootstrap_expect": 1, "server": true }`},
		hcl:  []string{`bootstrap_expect = 1 server = true`},
		expected: func(rt *RuntimeConfig) {
			rt.Bootstrap = true
			rt.BootstrapExpect = 0
			rt.LeaveOnTerm = false
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
		expectedWarnings: []string{"BootstrapExpect is set to 1; this is the same as Bootstrap mode.", "bootstrap = true: do not enable unless necessary"},
	})
	run(t, testCase{
		desc: "bootstrap-expect=2 warning",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "bootstrap_expect": 2, "server": true }`},
		hcl:  []string{`bootstrap_expect = 2 server = true`},
		expected: func(rt *RuntimeConfig) {
			rt.BootstrapExpect = 2
			rt.LeaveOnTerm = false
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
		expectedWarnings: []string{
			`bootstrap_expect = 2: A cluster with 2 servers will provide no failure tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`,
			`bootstrap_expect > 0: expecting 2 servers`,
		},
	})
	run(t, testCase{
		desc: "bootstrap-expect > 2 but even warning",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "bootstrap_expect": 4, "server": true }`},
		hcl:  []string{`bootstrap_expect = 4 server = true`},
		expected: func(rt *RuntimeConfig) {
			rt.BootstrapExpect = 4
			rt.LeaveOnTerm = false
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.SkipLeaveOnInt = true
			rt.DataDir = dataDir
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
		expectedWarnings: []string{
			`bootstrap_expect is even number: A cluster with an even number of servers does not achieve optimum fault tolerance. See https://www.consul.io/docs/internals/consensus.html#deployment-table`,
			`bootstrap_expect > 0: expecting 4 servers`,
		},
	})
	run(t, testCase{
		desc: "client mode sets LeaveOnTerm and SkipLeaveOnInt correctly",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "server": false }`},
		hcl:  []string{` server = false`},
		expected: func(rt *RuntimeConfig) {
			rt.LeaveOnTerm = true
			rt.ServerMode = false
			rt.TLS.ServerMode = false
			rt.SkipLeaveOnInt = false
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "client does not allow socket",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "client_addr": "unix:///foo" }`},
		hcl:         []string{`client_addr = "unix:///foo"`},
		expectedErr: "client_addr cannot be a unix socket",
	})
	run(t, testCase{
		desc:        "datacenter invalid",
		args:        []string{`-data-dir=` + dataDir},
		json:        []string{`{ "datacenter": "%" }`},
		hcl:         []string{`datacenter = "%"`},
		expectedErr: `datacenter can only contain lowercase alphanumeric, - or _ characters.`,
	})
	run(t, testCase{
		desc: "dns does not allow socket",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "addresses": {"dns": "unix:///foo" } }`},
		hcl:         []string{`addresses = { dns = "unix:///foo" }`},
		expectedErr: "DNS address cannot be a unix socket",
	})
	run(t, testCase{
		desc: "ui enabled and dir specified",
		args: []string{
			`-datacenter=a`,
			`-data-dir=` + dataDir,
		},
		json: []string{`{ "ui_config": { "enabled": true, "dir": "a" } }`},
		hcl:  []string{`ui_config { enabled = true dir = "a"}`},
		expectedErr: "Both the ui_config.enabled and ui_config.dir (or -ui and -ui-dir) were specified, please provide only one.\n" +
			"If trying to use your own web UI resources, use ui_config.dir or the -ui-dir flag.\n" +
			"The web UI is included in the binary so use ui_config.enabled or the -ui flag to enable it",
	})

	// test ANY address failures
	// to avoid combinatory explosion for tests use 0.0.0.0, :: or [::] but not all of them
	run(t, testCase{
		desc: "advertise_addr any",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "advertise_addr": "0.0.0.0" }`},
		hcl:         []string{`advertise_addr = "0.0.0.0"`},
		expectedErr: "Advertise address cannot be 0.0.0.0, :: or [::]",
	})
	run(t, testCase{
		desc: "advertise_addr_wan any",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "advertise_addr_wan": "::" }`},
		hcl:         []string{`advertise_addr_wan = "::"`},
		expectedErr: "Advertise WAN address cannot be 0.0.0.0, :: or [::]",
	})
	run(t, testCase{
		desc: "recursors any",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "recursors": ["::"] }`},
		hcl:         []string{`recursors = ["::"]`},
		expectedErr: "DNS recursor address cannot be 0.0.0.0, :: or [::]",
	})
	run(t, testCase{
		desc: "dns_config.udp_answer_limit invalid",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "dns_config": { "udp_answer_limit": -1 } }`},
		hcl:         []string{`dns_config = { udp_answer_limit = -1 }`},
		expectedErr: "dns_config.udp_answer_limit cannot be -1. Must be greater than or equal to zero",
	})
	run(t, testCase{
		desc: "dns_config.a_record_limit invalid",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "dns_config": { "a_record_limit": -1 } }`},
		hcl:         []string{`dns_config = { a_record_limit = -1 }`},
		expectedErr: "dns_config.a_record_limit cannot be -1. Must be greater than or equal to zero",
	})
	run(t, testCase{
		desc: "performance.raft_multiplier < 0",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "performance": { "raft_multiplier": -1 } }`},
		hcl:         []string{`performance = { raft_multiplier = -1 }`},
		expectedErr: `performance.raft_multiplier cannot be -1. Must be between 1 and 10`,
	})
	run(t, testCase{
		desc: "performance.raft_multiplier == 0",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "performance": { "raft_multiplier": 0 } }`},
		hcl:         []string{`performance = { raft_multiplier = 0 }`},
		expectedErr: `performance.raft_multiplier cannot be 0. Must be between 1 and 10`,
	})
	run(t, testCase{
		desc: "performance.raft_multiplier > 10",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "performance": { "raft_multiplier": 20 } }`},
		hcl:         []string{`performance = { raft_multiplier = 20 }`},
		expectedErr: `performance.raft_multiplier cannot be 20. Must be between 1 and 10`,
	})
	run(t, testCase{
		desc: "node_name invalid",
		args: []string{
			`-data-dir=` + dataDir,
			`-node=`,
		},
		opts: LoadOpts{
			hostname: func() (string, error) { return "", nil },
		},
		expectedErr: "node_name cannot be empty",
	})
	run(t, testCase{
		desc: "node_meta key too long",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "dns_config": { "udp_answer_limit": 1 } }`,
			`{ "node_meta": { "` + randomString(130) + `": "a" } }`,
		},
		hcl: []string{
			`dns_config = { udp_answer_limit = 1 }`,
			`node_meta = { "` + randomString(130) + `" = "a" }`,
		},
		expectedErr: "Key is too long (limit: 128 characters)",
	})
	run(t, testCase{
		desc: "node_meta value too long",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "dns_config": { "udp_answer_limit": 1 } }`,
			`{ "node_meta": { "a": "` + randomString(520) + `" } }`,
		},
		hcl: []string{
			`dns_config = { udp_answer_limit = 1 }`,
			`node_meta = { "a" = "` + randomString(520) + `" }`,
		},
		expectedErr: "Value is too long (limit: 512 characters)",
	})
	run(t, testCase{
		desc: "node_meta too many keys",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "dns_config": { "udp_answer_limit": 1 } }`,
			`{ "node_meta": {` + metaPairs(70, "json") + `} }`,
		},
		hcl: []string{
			`dns_config = { udp_answer_limit = 1 }`,
			`node_meta = {` + metaPairs(70, "hcl") + ` }`,
		},
		expectedErr: "Node metadata cannot contain more than 64 key/value pairs",
	})
	run(t, testCase{
		desc: "unique listeners dns vs http",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"client_addr": "1.2.3.4",
					"ports": { "dns": 1000, "http": 1000 }
				}`},
		hcl: []string{`
					client_addr = "1.2.3.4"
					ports = { dns = 1000 http = 1000 }
				`},
		expectedErr: "HTTP address 1.2.3.4:1000 already configured for DNS",
	})
	run(t, testCase{
		desc: "unique listeners dns vs https",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"client_addr": "1.2.3.4",
					"ports": { "dns": 1000, "https": 1000 }
				}`},
		hcl: []string{`
					client_addr = "1.2.3.4"
					ports = { dns = 1000 https = 1000 }
				`},
		expectedErr: "HTTPS address 1.2.3.4:1000 already configured for DNS",
	})
	run(t, testCase{
		desc: "unique listeners http vs https",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"client_addr": "1.2.3.4",
					"ports": { "http": 1000, "https": 1000 }
				}`},
		hcl: []string{`
					client_addr = "1.2.3.4"
					ports = { http = 1000 https = 1000 }
				`},
		expectedErr: "HTTPS address 1.2.3.4:1000 already configured for HTTP",
	})
	run(t, testCase{
		desc: "unique advertise addresses HTTP vs RPC",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"addresses": { "http": "10.0.0.1" },
					"ports": { "http": 1000, "server": 1000 }
				}`},
		hcl: []string{`
					addresses = { http = "10.0.0.1" }
					ports = { http = 1000 server = 1000 }
				`},
		expectedErr: "RPC Advertise address 10.0.0.1:1000 already configured for HTTP",
	})
	run(t, testCase{
		desc: "unique advertise addresses RPC vs Serf LAN",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"ports": { "server": 1000, "serf_lan": 1000 }
				}`},
		hcl: []string{`
					ports = { server = 1000 serf_lan = 1000 }
				`},
		expectedErr: "Serf Advertise LAN address 10.0.0.1:1000 already configured for RPC Advertise",
	})
	run(t, testCase{
		desc: "unique advertise addresses RPC vs Serf WAN",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"ports": { "server": 1000, "serf_wan": 1000 }
				}`},
		hcl: []string{`
					ports = { server = 1000 serf_wan = 1000 }
				`},
		expectedErr: "Serf Advertise WAN address 10.0.0.1:1000 already configured for RPC Advertise",
	})
	run(t, testCase{
		desc: "http use_cache defaults to true",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"http_config": {}
			}`},
		hcl: []string{`
				http_config = {}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.HTTPUseCache = true
		},
	})
	run(t, testCase{
		desc: "http use_cache is enabled when true",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"http_config": { "use_cache": true }
			}`},
		hcl: []string{`
				http_config = { use_cache = true }
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.HTTPUseCache = true
		},
	})
	run(t, testCase{
		desc: "http use_cache is disabled when false",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"http_config": { "use_cache": false }
			}`},
		hcl: []string{`
				http_config = { use_cache = false }
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.HTTPUseCache = false
		},
	})
	run(t, testCase{
		desc: "cloud resource id from env",
		args: []string{
			`-server`,
			`-data-dir=` + dataDir,
		},
		setup: func() {
			os.Setenv("HCP_RESOURCE_ID", "env-id")
			t.Cleanup(func() {
				os.Unsetenv("HCP_RESOURCE_ID")
			})
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Cloud = hcpconfig.CloudConfig{
				// ID is only populated from env if not populated from other sources.
				ResourceID: "env-id",
			}

			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "cloud resource id from file",
		args: []string{
			`-server`,
			`-data-dir=` + dataDir,
		},
		setup: func() {
			os.Setenv("HCP_RESOURCE_ID", "env-id")
			t.Cleanup(func() {
				os.Unsetenv("HCP_RESOURCE_ID")
			})
		},
		json: []string{`{
			  "cloud": {
              	"resource_id": "file-id" 
              }
			}`},
		hcl: []string{`
			  cloud = {
	            resource_id = "file-id" 
			  }
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Cloud = hcpconfig.CloudConfig{
				// ID is only populated from env if not populated from other sources.
				ResourceID: "file-id",
			}

			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "sidecar_service can't have ID",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				  "service": {
						"name": "web",
						"port": 1234,
						"connect": {
							"sidecar_service": {
								"ID": "random-sidecar-id"
							}
						}
					}
				}`},
		hcl: []string{`
				service {
					name = "web"
					port = 1234
					connect {
						sidecar_service {
							ID = "random-sidecar-id"
						}
					}
				}
			`},
		expectedErr: "sidecar_service can't specify an ID",
	})
	run(t, testCase{
		desc: "sidecar_service can't have nested sidecar",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				  "service": {
						"name": "web",
						"port": 1234,
						"connect": {
							"sidecar_service": {
								"connect": {
									"sidecar_service": {}
								}
							}
						}
					}
				}`},
		hcl: []string{`
				service {
					name = "web"
					port = 1234
					connect {
						sidecar_service {
							connect {
								sidecar_service {
								}
							}
						}
					}
				}
			`},
		expectedErr: "sidecar_service can't have a nested sidecar_service",
	})
	run(t, testCase{
		desc: "telemetry.prefix_filter cannot be empty",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"telemetry": { "prefix_filter": [""] }
				}`},
		hcl: []string{`
					telemetry = { prefix_filter = [""] }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
		},
		expectedWarnings: []string{"Cannot have empty filter rule in prefix_filter"},
	})
	run(t, testCase{
		desc: "telemetry.prefix_filter must start with + or -",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
					"telemetry": { "prefix_filter": ["+foo", "-bar", "nix"] }
				}`},
		hcl: []string{`
					telemetry = { prefix_filter = ["+foo", "-bar", "nix"] }
				`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Telemetry.AllowedPrefixes = []string{"foo"}
			rt.Telemetry.BlockedPrefixes = []string{"bar", "consul.rpc.server.call"}
		},
		expectedWarnings: []string{`Filter rule must begin with either '+' or '-': "nix"`},
	})
	run(t, testCase{
		desc: "encrypt has invalid key",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json:        []string{`{ "encrypt": "this is not a valid key" }`},
		hcl:         []string{` encrypt = "this is not a valid key" `},
		expectedErr: "encrypt has invalid key: illegal base64 data at input byte 4",
	})
	run(t, testCase{
		desc: "multiple check files",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "args": ["/bin/true"], "interval": "1s" } }`,
			`{ "check": { "name": "b", "args": ["/bin/false"], "interval": "1s" } }`,
		},
		hcl: []string{
			`check = { name = "a" args = ["/bin/true"] interval = "1s"}`,
			`check = { name = "b" args = ["/bin/false"] interval = "1s" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a", ScriptArgs: []string{"/bin/true"}, OutputMaxSize: checks.DefaultBufSize, Interval: time.Second},
				{Name: "b", ScriptArgs: []string{"/bin/false"}, OutputMaxSize: checks.DefaultBufSize, Interval: time.Second},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "grpc check",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "grpc": "localhost:12345/foo", "grpc_use_tls": true, "interval": "1s" } }`,
		},
		hcl: []string{
			`check = { name = "a" grpc = "localhost:12345/foo", grpc_use_tls = true interval = "1s" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a", GRPC: "localhost:12345/foo", GRPCUseTLS: true, OutputMaxSize: checks.DefaultBufSize, Interval: time.Second},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "h2ping check without h2ping_use_tls set",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "h2ping": "localhost:55555", "interval": "5s" } }`,
		},
		hcl: []string{
			`check = { name = "a" h2ping = "localhost:55555" interval = "5s" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a", H2PING: "localhost:55555", H2PingUseTLS: true, OutputMaxSize: checks.DefaultBufSize, Interval: 5 * time.Second},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "h2ping check with h2ping_use_tls set to false",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "h2ping": "localhost:55555", "h2ping_use_tls": false, "interval": "5s" } }`,
		},
		hcl: []string{
			`check = { name = "a" h2ping = "localhost:55555" h2ping_use_tls = false interval = "5s" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a", H2PING: "localhost:55555", H2PingUseTLS: false, OutputMaxSize: checks.DefaultBufSize, Interval: 5 * time.Second},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "alias check with no node",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "alias_service": "foo" } }`,
		},
		hcl: []string{
			`check = { name = "a", alias_service = "foo" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a", AliasService: "foo", OutputMaxSize: checks.DefaultBufSize},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "os_service check no interval",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "os_service": "foo" } }`,
		},
		hcl: []string{
			`check = { name = "a", os_service = "foo" }`,
		},
		expectedErr: `Interval must be > 0 for Script, HTTP, H2PING, TCP, UDP or OSService checks`,
	})
	run(t, testCase{
		desc: "os_service check",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "check": { "name": "a", "os_service": "foo", "interval": "30s" } }`,
		},
		hcl: []string{
			`check = { name = "a", os_service = "foo", interval = "30s" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Checks = []*structs.CheckDefinition{
				{Name: "a",
					OSService:     "foo",
					Interval:      30 * time.Second,
					OutputMaxSize: checks.DefaultBufSize,
				},
			}
			rt.DataDir = dataDir
		}})
	run(t, testCase{
		desc: "multiple service files",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "service": { "name": "a", "port": 80 } }`,
			`{ "service": { "name": "b", "port": 90, "meta": {"my": "value"}, "weights": {"passing": 13} } }`,
		},
		hcl: []string{
			`service = { name = "a" port = 80 }`,
			`service = { name = "b" port = 90 meta={my="value"}, weights={passing=13}}`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Services = []*structs.ServiceDefinition{
				{
					Name: "a",
					Port: 80,
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
				},
				{
					Name: "b",
					Port: 90,
					Meta: map[string]string{"my": "value"},
					Weights: &structs.Weights{
						Passing: 13,
						Warning: 1,
					},
				},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "service with wrong meta: too long key",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "service": { "name": "a", "port": 80, "meta": { "` + randomString(520) + `": "metaValue" } } }`,
		},
		hcl: []string{
			`service = { name = "a" port = 80, meta={` + randomString(520) + `="metaValue"} }`,
		},
		expectedErr: `Key is too long`,
	})
	run(t, testCase{
		desc: "service with wrong meta: too long value",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "service": { "name": "a", "port": 80, "meta": { "a": "` + randomString(520) + `" } } }`,
		},
		hcl: []string{
			`service = { name = "a" port = 80, meta={a="` + randomString(520) + `"} }`,
		},
		expectedErr: `Value is too long`,
	})
	run(t, testCase{
		desc: "service with wrong meta: too many meta",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "service": { "name": "a", "port": 80, "meta": { ` + metaPairs(70, "json") + `} } }`,
		},
		hcl: []string{
			`service = { name = "a" port = 80 meta={` + metaPairs(70, "hcl") + `} }`,
		},
		expectedErr: `invalid meta for service a: Node metadata cannot contain more than 64 key`,
	})
	run(t, testCase{
		desc: "verify_outgoing in the grpc stanza",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
			tls {
				grpc {
					verify_outgoing = true
				}
			}
		`},
		json: []string{`
			{
				"tls": {
					"grpc": {
						"verify_outgoing": true
					}
				}
			}
		`},
		expectedErr: "verify_outgoing is not valid in the tls.grpc stanza",
	})
	run(t, testCase{
		desc: "verify_server_hostname in the defaults stanza",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
			tls {
				defaults {
					verify_server_hostname = true
				}
			}
		`},
		json: []string{`
			{
				"tls": {
					"defaults": {
						"verify_server_hostname": true
					}
				}
			}
		`},
		expectedErr: "verify_server_hostname is only valid in the tls.internal_rpc stanza",
	})
	run(t, testCase{
		desc: "verify_server_hostname in the grpc stanza",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
			tls {
				grpc {
					verify_server_hostname = true
				}
			}
		`},
		json: []string{`
			{
				"tls": {
					"grpc": {
						"verify_server_hostname": true
					}
				}
			}
		`},
		expectedErr: "verify_server_hostname is only valid in the tls.internal_rpc stanza",
	})
	run(t, testCase{
		desc: "verify_server_hostname in the https stanza",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
			tls {
				https {
					verify_server_hostname = true
				}
			}
		`},
		json: []string{`
			{
				"tls": {
					"https": {
						"verify_server_hostname": true
					}
				}
			}
		`},
		expectedErr: "verify_server_hostname is only valid in the tls.internal_rpc stanza",
	})
	run(t, testCase{
		desc: "translated keys",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{
					"service": {
						"name": "a",
						"port": 80,
						"tagged_addresses": {
							"wan": {
								"address": "198.18.3.4",
								"port": 443
							}
						},
						"enable_tag_override": true,
						"check": {
							"id": "x",
							"name": "y",
							"DockerContainerID": "z",
							"DeregisterCriticalServiceAfter": "10s",
							"ScriptArgs": ["a", "b"],
							"Interval": "2s"
						}
					}
				}`,
		},
		hcl: []string{
			`service = {
					name = "a"
					port = 80
					enable_tag_override = true
					tagged_addresses = {
						wan = {
							address = "198.18.3.4"
							port = 443
						}
					}
					check = {
						id = "x"
						name = "y"
						DockerContainerID = "z"
						DeregisterCriticalServiceAfter = "10s"
						ScriptArgs = ["a", "b"]
						Interval = "2s"
					}
				}`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.Services = []*structs.ServiceDefinition{
				{
					Name: "a",
					Port: 80,
					TaggedAddresses: map[string]structs.ServiceAddress{
						"wan": {
							Address: "198.18.3.4",
							Port:    443,
						},
					},
					EnableTagOverride: true,
					Checks: []*structs.CheckType{
						{
							CheckID:                        "x",
							Name:                           "y",
							DockerContainerID:              "z",
							DeregisterCriticalServiceAfter: 10 * time.Second,
							ScriptArgs:                     []string{"a", "b"},
							OutputMaxSize:                  checks.DefaultBufSize,
							Interval:                       2 * time.Second,
						},
					},
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
				},
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "ignore snapshot_agent sub-object",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			`{ "snapshot_agent": { "dont": "care" } }`,
		},
		hcl: []string{
			`snapshot_agent = { dont = "care" }`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
		},
	})

	run(t, testCase{
		// Test that slices in structured config are preserved by
		// decode.HookWeakDecodeFromSlice.
		desc: "service.connectsidecar_service with checks and upstreams",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				  "service": {
						"name": "web",
						"port": 1234,
						"connect": {
							"sidecar_service": {
								"port": 2345,
								"checks": [
									{
										"TCP": "127.0.0.1:2345",
										"Interval": "10s"
									}
								],
								"proxy": {
									"expose": {
										"checks": true,
										"paths": [
											{
												"path": "/health",
												"local_path_port": 8080,
												"listener_port": 21500,
												"protocol": "http"
											}
										]
									},
									"mode": "transparent",
									"transparent_proxy": {
										"outbound_listener_port": 10101,
										"dialed_directly": true
									},
									"upstreams": [
										{
											"destination_name": "db",
											"local_bind_port": 7000
										},
										{
											"destination_name": "db2",
											"local_bind_socket_path": "/tmp/socketpath",
											"local_bind_socket_mode": "0644"
										}
									]
								}
							}
						}
					}
				}`},
		hcl: []string{`
				service {
					name = "web"
					port = 1234
					connect {
						sidecar_service {
							port = 2345
							checks = [
								{
									tcp = "127.0.0.1:2345"
									interval = "10s"
								}
							]
							proxy {
								expose {
									checks = true
									paths = [
										{
											path = "/health"
											local_path_port = 8080
											listener_port = 21500
											protocol = "http"
										}
									]
								}
								mode = "transparent"
								transparent_proxy = {
									outbound_listener_port = 10101
									dialed_directly = true
								}
								upstreams = [
									{
										destination_name = "db"
										local_bind_port = 7000
									},
									{
									    destination_name = "db2",
									    local_bind_socket_path = "/tmp/socketpath",
									    local_bind_socket_mode = "0644"
									}
								]
							}
						}
					}
				}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Services = []*structs.ServiceDefinition{
				{
					Name: "web",
					Port: 1234,
					Connect: &structs.ServiceConnect{
						SidecarService: &structs.ServiceDefinition{
							Port: 2345,
							Checks: structs.CheckTypes{
								{
									TCP:           "127.0.0.1:2345",
									Interval:      10 * time.Second,
									OutputMaxSize: checks.DefaultBufSize,
								},
							},
							Proxy: &structs.ConnectProxyConfig{
								Expose: structs.ExposeConfig{
									Checks: true,
									Paths: []structs.ExposePath{
										{
											Path:          "/health",
											LocalPathPort: 8080,
											ListenerPort:  21500,
											Protocol:      "http",
										},
									},
								},
								Mode: structs.ProxyModeTransparent,
								TransparentProxy: structs.TransparentProxyConfig{
									OutboundListenerPort: 10101,
									DialedDirectly:       true,
								},
								Upstreams: structs.Upstreams{
									structs.Upstream{
										DestinationType: "service",
										DestinationName: "db",
										LocalBindPort:   7000,
									},
									structs.Upstream{
										DestinationType:     "service",
										DestinationName:     "db2",
										LocalBindSocketPath: "/tmp/socketpath",
										LocalBindSocketMode: "0644",
									},
								},
							},
							Weights: &structs.Weights{
								Passing: 1,
								Warning: 1,
							},
						},
					},
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
				},
			}
		},
	})
	run(t, testCase{
		// Test that slices in structured config are preserved by
		// decode.HookWeakDecodeFromSlice.
		desc: "services.connect.sidecar_service with checks and upstreams",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				  "services": [{
						"name": "web",
						"port": 1234,
						"connect": {
							"sidecar_service": {
								"port": 2345,
								"checks": [
									{
										"TCP": "127.0.0.1:2345",
										"Interval": "10s"
									}
								],
								"proxy": {
									"expose": {
										"checks": true,
										"paths": [
											{
												"path": "/health",
												"local_path_port": 8080,
												"listener_port": 21500,
												"protocol": "http"
											}
										]
									},
									"mode": "transparent",
									"transparent_proxy": {
										"outbound_listener_port": 10101,
										"dialed_directly": true
									},
									"upstreams": [
										{
											"destination_name": "db",
											"local_bind_port": 7000
										}
									]
								}
							}
						}
					}]
				}`},
		hcl: []string{`
				services = [{
					name = "web"
					port = 1234
					connect {
						sidecar_service {
							port = 2345
							checks = [
								{
									tcp = "127.0.0.1:2345"
									interval = "10s"
								}
							]
							proxy {
								expose {
									checks = true
									paths = [
										{
											path = "/health"
											local_path_port = 8080
											listener_port = 21500
											protocol = "http"
										}
									]
								}
								mode = "transparent"
								transparent_proxy = {
									outbound_listener_port = 10101
									dialed_directly = true
								}
								upstreams = [
									{
										destination_name = "db"
										local_bind_port = 7000
									},
								]
							}
						}
					}
				}]
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Services = []*structs.ServiceDefinition{
				{
					Name: "web",
					Port: 1234,
					Connect: &structs.ServiceConnect{
						SidecarService: &structs.ServiceDefinition{
							Port: 2345,
							Checks: structs.CheckTypes{
								{
									TCP:           "127.0.0.1:2345",
									Interval:      10 * time.Second,
									OutputMaxSize: checks.DefaultBufSize,
								},
							},
							Proxy: &structs.ConnectProxyConfig{
								Expose: structs.ExposeConfig{
									Checks: true,
									Paths: []structs.ExposePath{
										{
											Path:          "/health",
											LocalPathPort: 8080,
											ListenerPort:  21500,
											Protocol:      "http",
										},
									},
								},
								Mode: structs.ProxyModeTransparent,
								TransparentProxy: structs.TransparentProxyConfig{
									OutboundListenerPort: 10101,
									DialedDirectly:       true,
								},
								Upstreams: structs.Upstreams{
									structs.Upstream{
										DestinationType: "service",
										DestinationName: "db",
										LocalBindPort:   7000,
									},
								},
							},
							Weights: &structs.Weights{
								Passing: 1,
								Warning: 1,
							},
						},
					},
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
				},
			}
		},
	})
	run(t, testCase{
		// This tests checks that VerifyServerHostname implies VerifyOutgoing
		desc: "verify_server_hostname implies verify_outgoing",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "verify_server_hostname": true
			}`},
		hcl: []string{`
			  verify_server_hostname = true
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.InternalRPC.VerifyServerHostname = true
			rt.TLS.InternalRPC.VerifyOutgoing = true
		},
		expectedWarnings: []string{
			deprecationWarning("verify_server_hostname", "tls.internal_rpc.verify_server_hostname"),
		},
	})
	run(t, testCase{
		desc: "auto_encrypt.allow_tls works implies connect",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "tls": { "internal_rpc": { "verify_incoming": true } },
			  "auto_encrypt": { "allow_tls": true },
			  "server": true
			}`},
		hcl: []string{`
			  tls { internal_rpc { verify_incoming = true } }
			  auto_encrypt { allow_tls = true }
			  server = true
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.InternalRPC.VerifyIncoming = true
			rt.AutoEncryptAllowTLS = true
			rt.ConnectEnabled = true

			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "auto_encrypt.allow_tls works with tls.defaults.verify_incoming",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "tls": { "defaults": { "verify_incoming": true } },
			  "auto_encrypt": { "allow_tls": true },
			  "server": true
			}`},
		hcl: []string{`
			  tls { defaults { verify_incoming = true } }
			  auto_encrypt { allow_tls = true }
			  server = true
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.AutoEncryptAllowTLS = true
			rt.ConnectEnabled = true

			rt.TLS.InternalRPC.VerifyIncoming = true
			rt.TLS.GRPC.VerifyIncoming = true
			rt.TLS.HTTPS.VerifyIncoming = true

			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "auto_encrypt.allow_tls works with tls.internal_rpc.verify_incoming",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "tls": { "internal_rpc": { "verify_incoming": true } },
			  "auto_encrypt": { "allow_tls": true },
			  "server": true
			}`},
		hcl: []string{`
			  tls { internal_rpc { verify_incoming = true } }
			  auto_encrypt { allow_tls = true }
			  server = true
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.AutoEncryptAllowTLS = true
			rt.ConnectEnabled = true
			rt.TLS.InternalRPC.VerifyIncoming = true

			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "auto_encrypt.allow_tls warns without tls.defaults.verify_incoming or tls.internal_rpc.verify_incoming",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "auto_encrypt": { "allow_tls": true },
			  "server": true
			}`},
		hcl: []string{`
			  auto_encrypt { allow_tls = true }
			  server = true
			`},
		expectedWarnings: []string{"if auto_encrypt.allow_tls is turned on, tls.internal_rpc.verify_incoming should be enabled (either explicitly or via tls.defaults.verify_incoming). It is necessary to turn it off during a migration to TLS, but it should definitely be turned on afterwards."},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.AutoEncryptAllowTLS = true
			rt.ConnectEnabled = true
			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "rpc.enable_streaming = true has no effect when not running in server mode",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "rpc": { "enable_streaming": true }
			}`},
		hcl: []string{`
			  rpc { enable_streaming = true }
			`},
		expectedWarnings: []string{"rpc.enable_streaming = true has no effect when not running in server mode"},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			// rpc.enable_streaming make no sense in not-server mode
			rt.RPCConfig.EnableStreaming = true
			rt.ServerMode = false
			rt.TLS.ServerMode = false
		},
	})
	run(t, testCase{
		desc: "use_streaming_backend = true requires rpc.enable_streaming on servers to work properly",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "use_streaming_backend": true,
              "rpc": {"enable_streaming": false},
			  "server": true
			}`},
		hcl: []string{`
			  use_streaming_backend = true
              rpc { enable_streaming = false }
			  server = true
			`},
		expectedWarnings: []string{"use_streaming_backend = true requires rpc.enable_streaming on servers to work properly"},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.UseStreamingBackend = true
			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	run(t, testCase{
		desc: "auto_encrypt.allow_tls errors in client mode",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "auto_encrypt": { "allow_tls": true },
			  "server": false
			}`},
		hcl: []string{`
			  auto_encrypt { allow_tls = true }
			  server = false
			`},
		expectedErr: "auto_encrypt.allow_tls can only be used on a server.",
	})
	run(t, testCase{
		desc: "auto_encrypt.tls errors in server mode",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "auto_encrypt": { "tls": true },
			  "server": true
			}`},
		hcl: []string{`
			  auto_encrypt { tls = true }
			  server = true
			`},
		expectedErr: "auto_encrypt.tls can only be used on a client.",
	})
	run(t, testCase{
		desc: "test connect vault provider configuration",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"connect": {
					"enabled": true,
					"ca_provider": "vault",
					"ca_config": {
						"ca_file": "/capath/ca.pem",
						"ca_path": "/capath/",
						"cert_file": "/certpath/cert.pem",
						"key_file": "/certpath/key.pem",
						"tls_server_name": "server.name",
						"tls_skip_verify": true,
						"token": "abc",
						"root_pki_path": "consul-vault",
						"intermediate_pki_path": "connect-intermediate"
					}
				}
			}`},
		hcl: []string{`
			  connect {
					enabled = true
					ca_provider = "vault"
					ca_config {
						ca_file = "/capath/ca.pem"
						ca_path = "/capath/"
						cert_file = "/certpath/cert.pem"
						key_file = "/certpath/key.pem"
						tls_server_name = "server.name"
						tls_skip_verify = true
						token = "abc"
						root_pki_path = "consul-vault"
						intermediate_pki_path = "connect-intermediate"
					}
				}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConnectEnabled = true
			rt.ConnectCAProvider = "vault"
			rt.ConnectCAConfig = map[string]interface{}{
				"CAFile":              "/capath/ca.pem",
				"CAPath":              "/capath/",
				"CertFile":            "/certpath/cert.pem",
				"KeyFile":             "/certpath/key.pem",
				"TLSServerName":       "server.name",
				"TLSSkipVerify":       true,
				"Token":               "abc",
				"RootPKIPath":         "consul-vault",
				"IntermediatePKIPath": "connect-intermediate",
			}
		},
	})
	run(t, testCase{
		desc: "test connect vault provider configuration with root cert ttl",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"connect": {
					"enabled": true,
					"ca_provider": "vault",
					"ca_config": {
						"ca_file": "/capath/ca.pem",
						"ca_path": "/capath/",
						"cert_file": "/certpath/cert.pem",
						"key_file": "/certpath/key.pem",
						"tls_server_name": "server.name",
						"tls_skip_verify": true,
						"token": "abc",
						"root_pki_path": "consul-vault",
						"root_cert_ttl": "96360h",
						"intermediate_pki_path": "connect-intermediate"
					}
				}
			}`},
		hcl: []string{`
			  connect {
					enabled = true
					ca_provider = "vault"
					ca_config {
						ca_file = "/capath/ca.pem"
						ca_path = "/capath/"
						cert_file = "/certpath/cert.pem"
						key_file = "/certpath/key.pem"
						tls_server_name = "server.name"
						tls_skip_verify = true
						root_pki_path = "consul-vault"
						token = "abc"
						intermediate_pki_path = "connect-intermediate"
						root_cert_ttl = "96360h"
					}
				}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConnectEnabled = true
			rt.ConnectCAProvider = "vault"
			rt.ConnectCAConfig = map[string]interface{}{
				"CAFile":              "/capath/ca.pem",
				"CAPath":              "/capath/",
				"CertFile":            "/certpath/cert.pem",
				"KeyFile":             "/certpath/key.pem",
				"TLSServerName":       "server.name",
				"TLSSkipVerify":       true,
				"Token":               "abc",
				"RootPKIPath":         "consul-vault",
				"RootCertTTL":         "96360h",
				"IntermediatePKIPath": "connect-intermediate",
			}
		},
	})
	run(t, testCase{
		desc: "Connect AWS CA provider configuration",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"connect": {
					"enabled": true,
					"ca_provider": "aws-pca",
					"ca_config": {
						"existing_arn": "foo",
						"delete_on_exit": true
					}
				}
			}`},
		hcl: []string{`
			  connect {
					enabled = true
					ca_provider = "aws-pca"
					ca_config {
						existing_arn = "foo"
						delete_on_exit = true
					}
				}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConnectEnabled = true
			rt.ConnectCAProvider = "aws-pca"
			rt.ConnectCAConfig = map[string]interface{}{
				"ExistingARN":  "foo",
				"DeleteOnExit": true,
			}
		},
	})
	run(t, testCase{
		desc: "Connect AWS CA provider TTL validation",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"connect": {
					"enabled": true,
					"ca_provider": "aws-pca",
					"ca_config": {
						"leaf_cert_ttl": "1h"
					}
				}
			}`},
		hcl: []string{`
			  connect {
					enabled = true
					ca_provider = "aws-pca"
					ca_config {
						leaf_cert_ttl = "1h"
					}
				}
			`},
		expectedErr: "AWS PCA doesn't support certificates that are valid for less than 24 hours",
	})
	run(t, testCase{
		desc: "Connect AWS CA provider EC key length validation",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
				"connect": {
					"enabled": true,
					"ca_provider": "aws-pca",
					"ca_config": {
						"private_key_bits": 521
					}
				}
			}`},
		hcl: []string{`
			  connect {
					enabled = true
					ca_provider = "aws-pca"
					ca_config {
						private_key_bits = 521
					}
				}
			`},
		expectedErr: "AWS PCA only supports P256 EC curve",
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation requires connect.enabled",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "connect": {
				"enabled": false,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  connect {
			    enabled = false
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expectedErr: "'connect.enable_mesh_gateway_wan_federation=true' requires 'connect.enabled=true'",
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation cannot use -join-wan",
		args: []string{
			`-data-dir=` + dataDir,
			`-join-wan=1.2.3.4`,
		},
		json: []string{`{
			  "server": true,
			  "primary_datacenter": "one",
			  "datacenter": "one",
			  "connect": {
				"enabled": true,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  server = true
			  primary_datacenter = "one"
			  datacenter = "one"
			  connect {
			    enabled = true
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expectedErr: "'retry_join_wan' is incompatible with 'connect.enable_mesh_gateway_wan_federation = true'",
		expectedWarnings: []string{
			deprecatedFlagWarning("-join-wan", "-retry-join-wan"),
		},
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation cannot use -retry-join-wan",
		args: []string{
			`-data-dir=` + dataDir,
			`-retry-join-wan=1.2.3.4`,
		},
		json: []string{`{
			  "server": true,
			  "primary_datacenter": "one",
			  "datacenter": "one",
			  "connect": {
				"enabled": true,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  server = true
			  primary_datacenter = "one"
			  datacenter = "one"
			  connect {
			    enabled = true
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expectedErr: "'retry_join_wan' is incompatible with 'connect.enable_mesh_gateway_wan_federation = true'",
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation requires server mode",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "server": false,
			  "connect": {
				"enabled": true,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  server = false
			  connect {
			    enabled = true
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expectedErr: "'connect.enable_mesh_gateway_wan_federation = true' requires 'server = true'",
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation requires no slashes in node names",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "server": true,
			  "node_name": "really/why",
			  "connect": {
				"enabled": true,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  server = true
			  node_name = "really/why"
			  connect {
			    enabled = true
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expectedErr: "'connect.enable_mesh_gateway_wan_federation = true' requires that 'node_name' not contain '/' characters",
	})
	run(t, testCase{
		desc: "primary_gateways requires server mode",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "server": false,
			  "primary_gateways": [ "foo.local", "bar.local" ]
			}`},
		hcl: []string{`
			  server = false
			  primary_gateways = [ "foo.local", "bar.local" ]
			`},
		expectedErr: "'primary_gateways' requires 'server = true'",
	})
	run(t, testCase{
		desc: "primary_gateways only works in a secondary datacenter",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "server": true,
			  "primary_datacenter": "one",
			  "datacenter": "one",
			  "primary_gateways": [ "foo.local", "bar.local" ]
			}`},
		hcl: []string{`
			  server = true
			  primary_datacenter = "one"
			  datacenter = "one"
			  primary_gateways = [ "foo.local", "bar.local" ]
			`},
		expectedErr: "'primary_gateways' should only be configured in a secondary datacenter",
	})
	run(t, testCase{
		desc: "connect.enable_mesh_gateway_wan_federation in secondary with primary_gateways configured",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			  "server": true,
			  "primary_datacenter": "one",
			  "datacenter": "two",
			  "primary_gateways": [ "foo.local", "bar.local" ],
			  "connect": {
				"enabled": true,
				"enable_mesh_gateway_wan_federation": true
			  }
			}`},
		hcl: []string{`
			  server = true
			  primary_datacenter = "one"
			  datacenter = "two"
			  primary_gateways = [ "foo.local", "bar.local" ]
			  connect {
			    enabled = true
			    enable_mesh_gateway_wan_federation = true
			  }
			`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.Datacenter = "two"
			rt.PrimaryDatacenter = "one"
			rt.PrimaryGateways = []string{"foo.local", "bar.local"}
			rt.ConnectEnabled = true
			rt.ConnectMeshGatewayWANFederationEnabled = true
			// server things
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.LeaveOnTerm = false
			rt.SkipLeaveOnInt = true
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})

	// ------------------------------------------------------------
	// ConfigEntry Handling
	//
	run(t, testCase{
		desc: "ConfigEntry bootstrap doesn't parse",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"foo": "bar"
						}
					]
				}
			}`},
		hcl: []string{`
			config_entries {
				bootstrap {
					foo = "bar"
				}
			}`},
		expectedErr: "config_entries.bootstrap[0]: Payload does not contain a kind/Kind",
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap unknown kind",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "foo",
							"name": "bar",
							"baz": 1
						}
					]
				}
			}`},
		hcl: []string{`
			config_entries {
				bootstrap {
					kind = "foo"
					name = "bar"
					baz = 1
				}
			}`},
		expectedErr: "config_entries.bootstrap[0]: invalid config entry kind: foo",
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap invalid service-defaults",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "service-defaults",
							"name": "web",
							"made_up_key": "blah"
						}
					]
				}
			}`},
		hcl: []string{`
			config_entries {
				bootstrap {
					kind = "service-defaults"
					name = "web"
					made_up_key = "blah"
				}
			}`},
		expectedErr: "config_entries.bootstrap[0]: 1 error occurred:\n\t* invalid config key \"made_up_key\"\n\n",
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap proxy-defaults (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "proxy-defaults",
							"name": "global",
							"config": {
								"bar": "abc",
								"moreconfig": {
									"moar": "config"
								}
							},
							"mesh_gateway": {
								"mode": "remote"
							},
							"mode": "transparent",
							"transparent_proxy": {
								"outbound_listener_port": 10101,
								"dialed_directly": true
							}
						}
					]
				}
			}`},
		hcl: []string{`
				config_entries {
					bootstrap {
						kind = "proxy-defaults"
						name = "global"
						config {
						  "bar" = "abc"
						  "moreconfig" {
							"moar" = "config"
						  }
						}
						mesh_gateway {
							mode = "remote"
						}
						mode = "transparent"
						transparent_proxy = {
							outbound_listener_port = 10101
							dialed_directly = true
						}
					}
				}`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind:           structs.ProxyDefaults,
					Name:           structs.ProxyConfigGlobal,
					EnterpriseMeta: *defaultEntMeta,
					Config: map[string]interface{}{
						"bar": "abc",
						"moreconfig": map[string]interface{}{
							"moar": "config",
						},
					},
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap proxy-defaults (camel-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"Kind": "proxy-defaults",
							"Name": "global",
							"Config": {
								"bar": "abc",
								"moreconfig": {
									"moar": "config"
								}
							},
							"MeshGateway": {
								"Mode": "remote"
							},
							"Mode": "transparent",
							"TransparentProxy": {
								"OutboundListenerPort": 10101,
								"DialedDirectly": true
							}
						}
					]
				}
			}`},
		hcl: []string{`
				config_entries {
					bootstrap {
						Kind = "proxy-defaults"
						Name = "global"
						Config {
						  "bar" = "abc"
						  "moreconfig" {
							"moar" = "config"
						  }
						}
						MeshGateway {
							Mode = "remote"
						}
						Mode = "transparent"
						TransparentProxy = {
							OutboundListenerPort = 10101
							DialedDirectly = true
						}
					}
				}`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ProxyConfigEntry{
					Kind:           structs.ProxyDefaults,
					Name:           structs.ProxyConfigGlobal,
					EnterpriseMeta: *defaultEntMeta,
					Config: map[string]interface{}{
						"bar": "abc",
						"moreconfig": map[string]interface{}{
							"moar": "config",
						},
					},
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap service-defaults (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "service-defaults",
							"name": "web",
							"meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"protocol": "http",
							"external_sni": "abc-123",
							"mesh_gateway": {
								"mode": "remote"
							},
							"mode": "transparent",
							"transparent_proxy": {
								"outbound_listener_port": 10101,
								"dialed_directly": true
							}
						}
					]
				}
			}`},
		hcl: []string{`
				config_entries {
					bootstrap {
						kind = "service-defaults"
						name = "web"
						meta {
							"foo" = "bar"
							"gir" = "zim"
						}
						protocol = "http"
						external_sni = "abc-123"
						mesh_gateway {
							mode = "remote"
						}
						mode = "transparent"
						transparent_proxy = {
							outbound_listener_port = 10101
							dialed_directly = true
						}
					}
				}`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "web",
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					Protocol:       "http",
					ExternalSNI:    "abc-123",
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap service-defaults (camel-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"Kind": "service-defaults",
							"Name": "web",
							"Meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"Protocol": "http",
							"ExternalSNI": "abc-123",
							"MeshGateway": {
								"Mode": "remote"
							},
							"Mode": "transparent",
							"TransparentProxy": {
								"OutboundListenerPort": 10101,
								"DialedDirectly": true
							}
						}
					]
				}
			}`},
		hcl: []string{`
				config_entries {
					bootstrap {
						Kind = "service-defaults"
						Name = "web"
						Meta {
							"foo" = "bar"
							"gir" = "zim"
						}
						Protocol = "http"
						ExternalSNI = "abc-123"
						MeshGateway {
							Mode = "remote"
						}
						Mode = "transparent"
						TransparentProxy = {
							OutboundListenerPort = 10101
							DialedDirectly = true
						}
					}
				}`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ServiceConfigEntry{
					Kind: structs.ServiceDefaults,
					Name: "web",
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					Protocol:       "http",
					ExternalSNI:    "abc-123",
					MeshGateway: structs.MeshGatewayConfig{
						Mode: structs.MeshGatewayModeRemote,
					},
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap service-router (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "service-router",
							"name": "main",
							"meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"routes": [
								{
									"match": {
										"http": {
											"path_exact": "/foo",
											"header": [
												{
													"name": "debug1",
													"present": true
												},
												{
													"name": "debug2",
													"present": true,
													"invert": true
												},
												{
													"name": "debug3",
													"exact": "1"
												},
												{
													"name": "debug4",
													"prefix": "aaa"
												},
												{
													"name": "debug5",
													"suffix": "bbb"
												},
												{
													"name": "debug6",
													"regex": "a.*z"
												}
											]
										}
									},
									"destination": {
									  "service"                 : "carrot",
									  "service_subset"          : "kale",
									  "namespace"               : "leek",
									  "prefix_rewrite"          : "/alternate",
									  "request_timeout"         : "99s",
									  "idle_timeout"            : "99s",
									  "num_retries"             : 12345,
									  "retry_on_connect_failure": true,
									  "retry_on_status_codes"   : [401, 209]
									}
								},
								{
									"match": {
										"http": {
											"path_prefix": "/foo",
											"methods": [ "GET", "DELETE" ],
											"query_param": [
												{
													"name": "hack1",
													"present": true
												},
												{
													"name": "hack2",
													"exact": "1"
												},
												{
													"name": "hack3",
													"regex": "a.*z"
												}
											]
										}
									}
								},
								{
									"match": {
										"http": {
											"path_regex": "/foo"
										}
									}
								}
							]
						}
					]
				}
			}`},
		hcl: []string{`
				config_entries {
					bootstrap {
						kind = "service-router"
						name = "main"
						meta {
							"foo" = "bar"
							"gir" = "zim"
						}
						routes = [
							{
								match {
									http {
										path_exact = "/foo"
										header = [
											{
												name = "debug1"
												present = true
											},
											{
												name = "debug2"
												present = true
												invert = true
											},
											{
												name = "debug3"
												exact = "1"
											},
											{
												name = "debug4"
												prefix = "aaa"
											},
											{
												name = "debug5"
												suffix = "bbb"
											},
											{
												name = "debug6"
												regex = "a.*z"
											},
										]
									}
								}
								destination {
								  service               = "carrot"
								  service_subset         = "kale"
								  namespace             = "leek"
								  prefix_rewrite         = "/alternate"
								  request_timeout        = "99s"
								  idle_timeout           = "99s"
								  num_retries            = 12345
								  retry_on_connect_failure = true
								  retry_on_status_codes    = [401, 209]
								}
							},
							{
								match {
									http {
										path_prefix = "/foo"
										methods = [ "GET", "DELETE" ]
										query_param = [
											{
												name = "hack1"
												present = true
											},
											{
												name = "hack2"
												exact = "1"
											},
											{
												name = "hack3"
												regex = "a.*z"
											},
										]
									}
								}
							},
							{
								match {
									http {
										path_regex = "/foo"
									}
								}
							},
						]
					}
				}`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ServiceRouterConfigEntry{
					Kind: structs.ServiceRouter,
					Name: "main",
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					Routes: []structs.ServiceRoute{
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathExact: "/foo",
									Header: []structs.ServiceRouteHTTPMatchHeader{
										{
											Name:    "debug1",
											Present: true,
										},
										{
											Name:    "debug2",
											Present: true,
											Invert:  true,
										},
										{
											Name:  "debug3",
											Exact: "1",
										},
										{
											Name:   "debug4",
											Prefix: "aaa",
										},
										{
											Name:   "debug5",
											Suffix: "bbb",
										},
										{
											Name:  "debug6",
											Regex: "a.*z",
										},
									},
								},
							},
							Destination: &structs.ServiceRouteDestination{
								Service:               "carrot",
								ServiceSubset:         "kale",
								Namespace:             "leek",
								Partition:             acl.DefaultPartitionName,
								PrefixRewrite:         "/alternate",
								RequestTimeout:        99 * time.Second,
								IdleTimeout:           99 * time.Second,
								NumRetries:            12345,
								RetryOnConnectFailure: true,
								RetryOnStatusCodes:    []uint32{401, 209},
							},
						},
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathPrefix: "/foo",
									Methods:    []string{"GET", "DELETE"},
									QueryParam: []structs.ServiceRouteHTTPMatchQueryParam{
										{
											Name:    "hack1",
											Present: true,
										},
										{
											Name:  "hack2",
											Exact: "1",
										},
										{
											Name:  "hack3",
											Regex: "a.*z",
										},
									},
								},
							},
						},
						{
							Match: &structs.ServiceRouteMatch{
								HTTP: &structs.ServiceRouteHTTPMatch{
									PathRegex: "/foo",
								},
							},
						},
					},
				},
			}
		},
	})
	// TODO(rb): add in missing tests for ingress-gateway (snake + camel)
	// TODO(rb): add in missing tests for terminating-gateway (snake + camel)
	run(t, testCase{
		desc: "ConfigEntry bootstrap service-intentions (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "service-intentions",
							"name": "web",
							"meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"sources": [
								{
									"name": "foo",
									"action": "deny",
									"type": "consul",
									"description": "foo desc"
								},
								{
									"name": "bar",
									"action": "allow",
									"description": "bar desc"
								},
								{
									"name": "*",
									"action": "deny",
									"description": "wild desc"
								}
							]
						}
					]
				}
			}`,
		},
		hcl: []string{`
				config_entries {
				  bootstrap {
					kind = "service-intentions"
					name = "web"
					meta {
						"foo" = "bar"
						"gir" = "zim"
					}
					sources = [
					  {
						name        = "foo"
						action      = "deny"
						type        = "consul"
						description = "foo desc"
					  },
					  {
						name        = "bar"
						action      = "allow"
						description = "bar desc"
					  }
					]
					sources {
					  name        = "*"
					  action      = "deny"
					  description = "wild desc"
					}
				  }
				}
			`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind: "service-intentions",
					Name: "web",
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					Sources: []*structs.SourceIntention{
						{
							Name:           "foo",
							Action:         "deny",
							Type:           "consul",
							Description:    "foo desc",
							Precedence:     9,
							EnterpriseMeta: *defaultEntMeta,
						},
						{
							Name:           "bar",
							Action:         "allow",
							Type:           "consul",
							Description:    "bar desc",
							Precedence:     9,
							EnterpriseMeta: *defaultEntMeta,
						},
						{
							Name:           "*",
							Action:         "deny",
							Type:           "consul",
							Description:    "wild desc",
							Precedence:     8,
							EnterpriseMeta: *defaultEntMeta,
						},
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap service-intentions wildcard destination (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "service-intentions",
							"name": "*",
							"sources": [
								{
									"name": "foo",
									"action": "deny",
									"precedence": 6
								}
							]
						}
					]
				}
			}`,
		},
		hcl: []string{`
				config_entries {
				  bootstrap {
					kind = "service-intentions"
					name = "*"
					sources {
					  name   = "foo"
					  action = "deny"
					  # should be parsed, but we'll ignore it later
					  precedence = 6
					}
				  }
				}
			`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.ServiceIntentionsConfigEntry{
					Kind:           "service-intentions",
					Name:           "*",
					EnterpriseMeta: *defaultEntMeta,
					Sources: []*structs.SourceIntention{
						{
							Name:           "foo",
							Action:         "deny",
							Type:           "consul",
							Precedence:     6,
							EnterpriseMeta: *defaultEntMeta,
						},
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap cluster (snake-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"kind": "mesh",
							"meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"transparent_proxy": {
								"mesh_destinations_only": true
							}
						}
					]
				}
			}`,
		},
		hcl: []string{`
				config_entries {
				  bootstrap {
					kind = "mesh"
					meta {
						"foo" = "bar"
						"gir" = "zim"
					}
					transparent_proxy {
						mesh_destinations_only = true
					}
				  }
				}
			`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.MeshConfigEntry{
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					TransparentProxy: structs.TransparentProxyMeshConfig{
						MeshDestinationsOnly: true,
					},
				},
			}
		},
	})
	run(t, testCase{
		desc: "ConfigEntry bootstrap cluster (camel-case)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"config_entries": {
					"bootstrap": [
						{
							"Kind": "mesh",
							"Meta" : {
								"foo": "bar",
								"gir": "zim"
							},
							"TransparentProxy": {
								"MeshDestinationsOnly": true
							}
						}
					]
				}
			}`,
		},
		hcl: []string{`
				config_entries {
				  bootstrap {
					Kind = "mesh"
					Meta {
						"foo" = "bar"
						"gir" = "zim"
					}
					TransparentProxy {
						MeshDestinationsOnly = true
					}
				  }
				}
			`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.ConfigEntryBootstrap = []structs.ConfigEntry{
				&structs.MeshConfigEntry{
					Meta: map[string]string{
						"foo": "bar",
						"gir": "zim",
					},
					EnterpriseMeta: *defaultEntMeta,
					TransparentProxy: structs.TransparentProxyMeshConfig{
						MeshDestinationsOnly: true,
					},
				},
			}
		},
	})

	// /////////////////////////////////
	// Defaults sanity checks

	run(t, testCase{
		desc: "default limits",
		args: []string{
			`-data-dir=` + dataDir,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			// Note that in the happy case this test will pass even if you comment
			// out all the stuff below since rt is also initialized from the
			// defaults. But it's still valuable as it will fail as soon as the
			// defaults are changed from these values forcing that change to be
			// intentional.
			rt.RPCHandshakeTimeout = 5 * time.Second
			rt.RPCClientTimeout = 60 * time.Second
			rt.HTTPSHandshakeTimeout = 5 * time.Second
			rt.HTTPMaxConnsPerClient = 200
			rt.RPCMaxConnsPerClient = 100
			rt.RequestLimitsMode = consulrate.ModeDisabled
			rt.RequestLimitsReadRate = rate.Inf
			rt.RequestLimitsWriteRate = rate.Inf
			rt.SegmentLimit = 64
			rt.XDSUpdateRateLimit = 250
			rt.RPCRateLimit = rate.Inf
			rt.RPCMaxBurst = 1000
		},
	})

	// /////////////////////////////////
	// Auto Config related tests
	run(t, testCase{
		desc: "auto config and auto encrypt error",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					enabled = true
					intro_token = "blah"
					server_addresses = ["198.18.0.1"]
				}
				auto_encrypt {
					tls = true
				}
				tls {
					internal_rpc {
						verify_outgoing = true
					}
				}
			`},
		json: []string{`{
				"auto_config": {
					"enabled": true,
					"intro_token": "blah",
					"server_addresses": ["198.18.0.1"]
				},
				"auto_encrypt": {
					"tls": true
				},
				"tls": {
					"internal_rpc": {
						"verify_outgoing": true
					}
				}
			}`},
		expectedErr: "both auto_encrypt.tls and auto_config.enabled cannot be set to true.",
	})
	run(t, testCase{
		desc: "auto config not allowed for servers",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				server = true
				auto_config {
					enabled = true
					intro_token = "blah"
					server_addresses = ["198.18.0.1"]
				}
				tls {
					internal_rpc {
						verify_outgoing = true
					}
				}
			`},
		json: []string{`
			{
				"server": true,
				"auto_config": {
					"enabled": true,
					"intro_token": "blah",
					"server_addresses": ["198.18.0.1"]
				},
				"tls": {
					"internal_rpc": {
						"verify_outgoing": true
					}
				}
			}`},
		expectedErr: "auto_config.enabled cannot be set to true for server agents",
	})

	run(t, testCase{
		desc: "auto config tls not enabled",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					enabled = true
					server_addresses = ["198.18.0.1"]
					intro_token = "foo"
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"enabled": true,
					"server_addresses": ["198.18.0.1"],
					"intro_token": "foo"
				}
			}`},
		expectedErr: "auto_config.enabled cannot be set without configuring TLS for server communications",
	})

	run(t, testCase{
		desc: "auto config server tls not enabled",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				server = true
				auto_config {
					authorization {
						enabled = true
					}
				}
			`},
		json: []string{`
			{
				"server": true,
				"auto_config": {
					"authorization": {
						"enabled": true
					}
				}
			}`},
		expectedErr: "auto_config.authorization.enabled cannot be set without providing a TLS certificate for the server",
	})

	run(t, testCase{
		desc: "auto config no intro token",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					enabled = true
				 	server_addresses = ["198.18.0.1"]
				}
				tls {
					internal_rpc {
						verify_outgoing = true
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"enabled": true,
					"server_addresses": ["198.18.0.1"]
				},
				"tls": {
					"internal_rpc": {
						"verify_outgoing": true
					}
				}
			}`},
		expectedErr: "One of auto_config.intro_token, auto_config.intro_token_file or the CONSUL_INTRO_TOKEN environment variable must be set to enable auto_config",
	})

	run(t, testCase{
		desc: "auto config no server addresses",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					enabled = true
					intro_token = "blah"
				}
				tls {
					internal_rpc {
						verify_outgoing = true
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"enabled": true,
					"intro_token": "blah"
				},
				"tls": {
					"internal_rpc": {
						"verify_outgoing": true
					}
				}
			}`},
		expectedErr: "auto_config.enabled is set without providing a list of addresses",
	})

	run(t, testCase{
		desc: "auto config client",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					enabled = true
					intro_token = "blah"
					intro_token_file = "blah"
					server_addresses = ["198.18.0.1"]
					dns_sans = ["foo"]
					ip_sans = ["invalid", "127.0.0.1"]
				}
				tls {
					internal_rpc {
						verify_outgoing = true
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"enabled": true,
					"intro_token": "blah",
					"intro_token_file": "blah",
					"server_addresses": ["198.18.0.1"],
					"dns_sans": ["foo"],
					"ip_sans": ["invalid", "127.0.0.1"]
				},
				"tls": {
					"internal_rpc": {
						"verify_outgoing": true
					}
				}
			}`},
		expectedWarnings: []string{
			"Cannot parse ip \"invalid\" from auto_config.ip_sans",
			"Both an intro token and intro token file are set. The intro token will be used instead of the file",
		},
		expected: func(rt *RuntimeConfig) {
			rt.ConnectEnabled = true
			rt.AutoConfig.Enabled = true
			rt.AutoConfig.IntroToken = "blah"
			rt.AutoConfig.IntroTokenFile = "blah"
			rt.AutoConfig.ServerAddresses = []string{"198.18.0.1"}
			rt.AutoConfig.DNSSANs = []string{"foo"}
			rt.AutoConfig.IPSANs = []net.IP{net.IPv4(127, 0, 0, 1)}
			rt.DataDir = dataDir
			rt.TLS.InternalRPC.VerifyOutgoing = true
			rt.TLS.AutoTLS = true
		},
	})

	run(t, testCase{
		desc: "auto config authorizer client not allowed",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
				auto_config {
					authorization {
						enabled = true
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"authorization": {
						"enabled": true
					}
				}
			}`},
		expectedErr: "auto_config.authorization.enabled cannot be set to true for client agents",
	})

	run(t, testCase{
		desc: "auto config authorizer invalid config",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				auto_config {
					authorization {
						enabled = true
					}
				}
				tls {
					internal_rpc {
						cert_file = "foo"
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"authorization": {
						"enabled": true
					}
				},
				"tls": {
					"internal_rpc": {
						"cert_file": "foo"
					}
				}
			}`},
		expectedErr: `auto_config.authorization.static has invalid configuration: exactly one of 'JWTValidationPubKeys', 'JWKSURL', or 'OIDCDiscoveryURL' must be set for type "jwt"`,
	})

	run(t, testCase{
		desc: "auto config authorizer invalid config 2",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				auto_config {
					authorization {
						enabled = true
						static {
							jwks_url = "https://fake.uri.local"
							oidc_discovery_url = "https://fake.uri.local"
						}
					}
				}
				tls {
					internal_rpc {
						cert_file = "foo"
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"authorization": {
						"enabled": true,
						"static": {
							"jwks_url": "https://fake.uri.local",
							"oidc_discovery_url": "https://fake.uri.local"
						}
					}
				},
				"tls": {
					"internal_rpc": {
						"cert_file": "foo"
					}
				}
			}`},
		expectedErr: `auto_config.authorization.static has invalid configuration: exactly one of 'JWTValidationPubKeys', 'JWKSURL', or 'OIDCDiscoveryURL' must be set for type "jwt"`,
	})

	run(t, testCase{
		desc: "auto config authorizer require token replication in secondary",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				primary_datacenter = "otherdc"
				acl {
					enabled = true
				}
				auto_config {
					authorization {
						enabled = true
						static {
							jwks_url = "https://fake.uri.local"
							oidc_discovery_url = "https://fake.uri.local"
						}
					}
				}
				tls {
					internal_rpc {
						cert_file = "foo"
					}
				}
			`},
		json: []string{`
			{
				"primary_datacenter": "otherdc",
				"acl": {
					"enabled": true
				},
				"auto_config": {
					"authorization": {
						"enabled": true,
						"static": {
							"jwks_url": "https://fake.uri.local",
							"oidc_discovery_url": "https://fake.uri.local"
						}
					}
				},
				"tls": {
					"internal_rpc": {
						"cert_file": "foo"
					}
				}
			}`},
		expectedErr: `Enabling auto-config authorization (auto_config.authorization.enabled) in non primary datacenters with ACLs enabled (acl.enabled) requires also enabling ACL token replication (acl.enable_token_replication)`,
	})

	run(t, testCase{
		desc: "auto config authorizer invalid claim assertion",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				auto_config {
					authorization {
						enabled = true
						static {
							jwt_validation_pub_keys = ["-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"]
							claim_assertions = [
								"values.node == ${node}"
							]
						}
					}
				}
				tls {
					internal_rpc {
						cert_file = "foo"
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"authorization": {
						"enabled": true,
						"static": {
							"jwt_validation_pub_keys": ["-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"],
							"claim_assertions": [
								"values.node == ${node}"
							]
						}
					}
				},
				"tls": {
					"internal_rpc": {
						"cert_file": "foo"
					}
				}
			}`},
		expectedErr: `auto_config.authorization.static.claim_assertion "values.node == ${node}" is invalid: Selector "values" is not valid`,
	})
	run(t, testCase{
		desc: "auto config authorizer ok",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				auto_config {
					authorization {
						enabled = true
						static {
							jwt_validation_pub_keys = ["-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"]
							claim_assertions = [
								"value.node == ${node}"
							]
							claim_mappings = {
								node = "node"
							}
						}
					}
				}
				tls {
					internal_rpc {
						cert_file = "foo"
					}
				}
			`},
		json: []string{`
			{
				"auto_config": {
					"authorization": {
						"enabled": true,
						"static": {
							"jwt_validation_pub_keys": ["-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"],
							"claim_assertions": [
								"value.node == ${node}"
							],
							"claim_mappings": {
								"node": "node"
							}
						}
					}
				},
				"tls": {
					"internal_rpc": {
						"cert_file": "foo"
					}
				}
			}`},
		expected: func(rt *RuntimeConfig) {
			rt.AutoConfig.Authorizer.Enabled = true
			rt.AutoConfig.Authorizer.AuthMethod.Config["ClaimMappings"] = map[string]string{
				"node": "node",
			}
			rt.AutoConfig.Authorizer.AuthMethod.Config["JWTValidationPubKeys"] = []string{"-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"}
			rt.AutoConfig.Authorizer.ClaimAssertions = []string{"value.node == ${node}"}
			rt.DataDir = dataDir
			rt.LeaveOnTerm = false
			rt.ServerMode = true
			rt.TLS.ServerMode = true
			rt.SkipLeaveOnInt = true
			rt.TLS.InternalRPC.CertFile = "foo"
			rt.RPCConfig.EnableStreaming = true
			rt.GRPCTLSPort = 8503
			rt.GRPCTLSAddrs = []net.Addr{defaultGrpcTlsAddr}
		},
	})
	// UI Config tests
	run(t, testCase{
		desc: "ui config deprecated",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui": true,
				"ui_content_path": "/bar"
			}`},
		hcl: []string{`
			ui = true
			ui_content_path = "/bar"
			`},
		expectedWarnings: []string{
			`The 'ui' field is deprecated. Use the 'ui_config.enabled' field instead.`,
			`The 'ui_content_path' field is deprecated. Use the 'ui_config.content_path' field instead.`,
		},
		expected: func(rt *RuntimeConfig) {
			// Should still work!
			rt.UIConfig.Enabled = true
			rt.UIConfig.ContentPath = "/bar/"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "ui-dir config deprecated",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_dir": "/bar"
			}`},
		hcl: []string{`
			ui_dir = "/bar"
			`},
		expectedWarnings: []string{
			`The 'ui_dir' field is deprecated. Use the 'ui_config.dir' field instead.`,
		},
		expected: func(rt *RuntimeConfig) {
			// Should still work!
			rt.UIConfig.Dir = "/bar"
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "metrics_provider constraint",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_provider": "((((lisp 4 life))))"
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_provider = "((((lisp 4 life))))"
			}
			`},
		expectedErr: `ui_config.metrics_provider can only contain lowercase alphanumeric, - or _ characters.`,
	})
	run(t, testCase{
		desc: "metrics_provider_options_json invalid JSON",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_provider_options_json": "not valid JSON"
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_provider_options_json = "not valid JSON"
			}
			`},
		expectedErr: `ui_config.metrics_provider_options_json must be empty or a string containing a valid JSON object.`,
	})
	run(t, testCase{
		desc: "metrics_provider_options_json not an object",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_provider_options_json": "1.0"
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_provider_options_json = "1.0"
			}
			`},
		expectedErr: `ui_config.metrics_provider_options_json must be empty or a string containing a valid JSON object.`,
	})
	run(t, testCase{
		desc: "metrics_proxy.base_url valid",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"base_url": "___"
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					base_url = "___"
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.base_url must be a valid http or https URL.`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (empty)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (relative)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["bar/baz", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["bar/baz", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "bar/baz" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (weird)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["://bar/baz", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["://bar/baz", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "://bar/baz" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (fragment)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["/bar/baz#stuff", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["/bar/baz#stuff", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "/bar/baz#stuff" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (querystring)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["/bar/baz?stu=ff", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["/bar/baz?stu=ff", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "/bar/baz?stu=ff" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist invalid (encoded slash)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["/bar%2fbaz", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["/bar%2fbaz", "/foo"]
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.path_allowlist: path "/bar%2fbaz" is not an absolute path`,
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist ok",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"path_allowlist": ["/bar/baz", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					path_allowlist = ["/bar/baz", "/foo"]
				}
			}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.MetricsProxy.PathAllowlist = []string{"/bar/baz", "/foo"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist defaulted for prometheus",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_provider": "prometheus"
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_provider = "prometheus"
			}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.MetricsProvider = "prometheus"
			rt.UIConfig.MetricsProxy.PathAllowlist = []string{
				"/api/v1/query",
				"/api/v1/query_range",
			}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "metrics_proxy.path_allowlist not overridden with defaults for prometheus",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_provider": "prometheus",
					"metrics_proxy": {
						"path_allowlist": ["/bar/baz", "/foo"]
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_provider = "prometheus"
				metrics_proxy {
					path_allowlist = ["/bar/baz", "/foo"]
				}
			}
			`},
		expected: func(rt *RuntimeConfig) {
			rt.UIConfig.MetricsProvider = "prometheus"
			rt.UIConfig.MetricsProxy.PathAllowlist = []string{"/bar/baz", "/foo"}
			rt.DataDir = dataDir
		},
	})
	run(t, testCase{
		desc: "metrics_proxy.base_url http(s)",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"metrics_proxy": {
						"base_url": "localhost:1234"
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				metrics_proxy {
					base_url = "localhost:1234"
				}
			}
			`},
		expectedErr: `ui_config.metrics_proxy.base_url must be a valid http or https URL.`,
	})
	run(t, testCase{
		desc: "dashboard_url_templates key format",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"dashboard_url_templates": {
						"(*&ASDOUISD)": "localhost:1234"
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				dashboard_url_templates {
					"(*&ASDOUISD)" = "localhost:1234"
				}
			}
			`},
		expectedErr: `ui_config.dashboard_url_templates key names can only contain lowercase alphanumeric, - or _ characters.`,
	})
	run(t, testCase{
		desc: "dashboard_url_templates value format",
		args: []string{`-data-dir=` + dataDir},
		json: []string{`{
				"ui_config": {
					"dashboard_url_templates": {
						"services": "localhost:1234"
					}
				}
			}`},
		hcl: []string{`
			ui_config {
				dashboard_url_templates {
					services = "localhost:1234"
				}
			}
			`},
		expectedErr: `ui_config.dashboard_url_templates values must be a valid http or https URL.`,
	})

	// Per node reconnect timeout test
	run(t, testCase{
		desc: "server and advertised reconnect timeout error",
		args: []string{
			`-data-dir=` + dataDir,
			`-server`,
		},
		hcl: []string{`
				advertise_reconnect_timeout = "5s"
			`},
		json: []string{`
			{
				"advertise_reconnect_timeout": "5s"
			}`},
		expectedErr: "advertise_reconnect_timeout can only be used on a client",
	})

	run(t, testCase{
		desc: "TLS defaults and overrides",
		args: []string{
			`-data-dir=` + dataDir,
		},
		hcl: []string{`
			ports {
				https = 4321
			}

			tls {
				defaults {
					ca_file = "default_ca_file"
					ca_path = "default_ca_path"
					cert_file = "default_cert_file"
					tls_min_version = "TLSv1_2"
					tls_cipher_suites = "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256"
					verify_incoming = true
				}

				internal_rpc {
					ca_file = "internal_rpc_ca_file"
				}

				https {
					cert_file = "https_cert_file"
					tls_min_version = "TLSv1_3"
				}

				grpc {
					verify_incoming = false
					tls_cipher_suites = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
				}
			}
		`},
		json: []string{`
			{
				"ports": {
					"https": 4321
				},
				"tls": {
					"defaults": {
						"ca_file": "default_ca_file",
						"ca_path": "default_ca_path",
						"cert_file": "default_cert_file",
						"tls_min_version": "TLSv1_2",
						"tls_cipher_suites": "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
						"verify_incoming": true
					},
					"internal_rpc": {
						"ca_file": "internal_rpc_ca_file"
					},
					"https": {
						"cert_file": "https_cert_file",
						"tls_min_version": "TLSv1_3"
					},
					"grpc": {
						"verify_incoming": false,
						"tls_cipher_suites": "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
					}
				}
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir

			rt.HTTPSPort = 4321
			rt.HTTPSAddrs = []net.Addr{tcpAddr("127.0.0.1:4321")}

			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"

			rt.TLS.InternalRPC.CAFile = "internal_rpc_ca_file"
			rt.TLS.InternalRPC.CAPath = "default_ca_path"
			rt.TLS.InternalRPC.CertFile = "default_cert_file"
			rt.TLS.InternalRPC.TLSMinVersion = "TLSv1_2"
			rt.TLS.InternalRPC.CipherSuites = []types.TLSCipherSuite{types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256}
			rt.TLS.InternalRPC.VerifyIncoming = true

			rt.TLS.HTTPS.CAFile = "default_ca_file"
			rt.TLS.HTTPS.CAPath = "default_ca_path"
			rt.TLS.HTTPS.CertFile = "https_cert_file"
			rt.TLS.HTTPS.TLSMinVersion = "TLSv1_3"
			rt.TLS.HTTPS.VerifyIncoming = true

			rt.TLS.GRPC.CAFile = "default_ca_file"
			rt.TLS.GRPC.CAPath = "default_ca_path"
			rt.TLS.GRPC.CertFile = "default_cert_file"
			rt.TLS.GRPC.TLSMinVersion = "TLSv1_2"
			rt.TLS.GRPC.CipherSuites = []types.TLSCipherSuite{types.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA}
			rt.TLS.GRPC.VerifyIncoming = false
		},
	})
	run(t, testCase{
		desc: "tls.internal_rpc.verify_server_hostname implies tls.internal_rpc.verify_outgoing",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"tls": {
					"internal_rpc": {
						"verify_server_hostname": true
					}
				}
			}
		`},
		hcl: []string{`
			tls {
				internal_rpc {
					verify_server_hostname = true
				}
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir

			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"

			rt.TLS.InternalRPC.VerifyServerHostname = true
			rt.TLS.InternalRPC.VerifyOutgoing = true
		},
	})
	run(t, testCase{
		desc: "tls.grpc.use_auto_cert defaults to false",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"tls": {
					"grpc": {}
				}
			}
		`},
		hcl: []string{`
			tls {
				grpc {}
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"
			rt.TLS.GRPC.UseAutoCert = false
		},
	})
	run(t, testCase{
		desc: "tls.grpc.use_auto_cert defaults to false (II)",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"tls": {}
			}
		`},
		hcl: []string{`
			tls {
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"
			rt.TLS.GRPC.UseAutoCert = false
		},
	})
	run(t, testCase{
		desc: "tls.grpc.use_auto_cert defaults to false (III)",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
			}
		`},
		hcl: []string{`
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"
			rt.TLS.GRPC.UseAutoCert = false
		},
	})
	run(t, testCase{
		desc: "tls.grpc.use_auto_cert enabled when true",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"tls": {
					"grpc": {
						"use_auto_cert": true
					}
				}
			}
		`},
		hcl: []string{`
			tls {
				grpc {
					use_auto_cert = true
				}
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"
			rt.TLS.GRPC.UseAutoCert = true
		},
	})
	run(t, testCase{
		desc: "tls.grpc.use_auto_cert disabled when false",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"tls": {
					"grpc": {
						"use_auto_cert": false
					}
				}
			}
		`},
		hcl: []string{`
			tls {
				grpc {
					use_auto_cert = false
				}
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.TLS.Domain = "consul."
			rt.TLS.NodeName = "thehostname"
			rt.TLS.GRPC.UseAutoCert = false
		},
	})
	run(t, testCase{
		desc: "logstore defaults",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{``},
		hcl:  []string{``},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.RaftLogStoreConfig.Backend = consul.LogStoreBackendBoltDB
			rt.RaftLogStoreConfig.WAL.SegmentSize = 64 * 1024 * 1024
		},
	})
	run(t, testCase{
		// this was a bug in the initial config commit. Specifying part of this
		// stanza should still result in sensible defaults for the other parts.
		desc: "wal defaults",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`{
			"raft_logstore": {
				"backend": "boltdb"
			}
		}`},
		hcl: []string{`
			raft_logstore {
				backend = "boltdb"
			}
		`},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			rt.RaftLogStoreConfig.Backend = consul.LogStoreBackendBoltDB
			rt.RaftLogStoreConfig.WAL.SegmentSize = 64 * 1024 * 1024
		},
	})
	run(t, testCase{
		desc: "wal segment size lower bound",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"server": true,
				"raft_logstore": {
					"wal":{
						"segment_size_mb": 0
					}
				}
			}`},
		hcl: []string{`
			server = true
			raft_logstore {
				wal {
					segment_size_mb = 0
				}
			}`},
		expectedErr: "raft_logstore.wal.segment_size_mb cannot be less than",
	})
	run(t, testCase{
		desc: "wal segment size upper bound",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"server": true,
				"raft_logstore": {
					"wal":{
						"segment_size_mb": 1025
					}
				}
			}`},
		hcl: []string{`
			server = true
			raft_logstore {
				wal {
					segment_size_mb = 1025
				}
			}`},
		expectedErr: "raft_logstore.wal.segment_size_mb cannot be greater than",
	})
	run(t, testCase{
		desc: "valid logstore backend",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{`
			{
				"server": true,
				"raft_logstore": {
					"backend": "thecloud"
				}
			}`},
		hcl: []string{`
			server = true
			raft_logstore {
				backend = "thecloud"
			}`},
		expectedErr: "raft_logstore.backend must be one of 'boltdb' or 'wal'",
	})
	run(t, testCase{
		desc: "raft_logstore merging",
		args: []string{
			`-data-dir=` + dataDir,
		},
		json: []string{
			// File 1 has logstore info
			`{
				"raft_logstore": {
					"backend": "wal"
				}
			}`,
			// File 2 doesn't have anything for logstore
			`{
				"enable_debug": true
			}`,
		},
		hcl: []string{
			// File 1 has logstore info
			`
			raft_logstore {
				backend = "wal"
			}`,
			// File 2 doesn't have anything for logstore
			`
			enable_debug = true
			`,
		},
		expected: func(rt *RuntimeConfig) {
			rt.DataDir = dataDir
			// The logstore settings from first file should not be overridden by a
			// later file with nothing to say about logstores!
			rt.RaftLogStoreConfig.Backend = consul.LogStoreBackendWAL
			rt.EnableDebug = true
		},
	})
}

func (tc testCase) run(format string, dataDir string) func(t *testing.T) {
	return func(t *testing.T) {
		// clean data dir before every test
		os.RemoveAll(dataDir)
		os.MkdirAll(dataDir, 0755)

		if tc.setup != nil {
			tc.setup()
		}

		opts := tc.opts

		fs := flag.NewFlagSet("", flag.ContinueOnError)
		AddFlags(fs, &opts)
		require.NoError(t, fs.Parse(tc.args))
		require.Len(t, fs.Args(), 0)

		for i, data := range tc.source(format) {
			opts.sources = append(opts.sources, FileSource{
				Name:   fmt.Sprintf("src-%d.%s", i, format),
				Format: format,
				Data:   data,
			})
		}

		patchLoadOptsShims(&opts)
		result, err := Load(opts)
		switch {
		case err == nil && tc.expectedErr != "":
			t.Fatalf("got nil want error to contain %q", tc.expectedErr)
		case err != nil && tc.expectedErr == "":
			t.Fatalf("got error %s want nil", err)
		case err != nil && tc.expectedErr != "" && !strings.Contains(err.Error(), tc.expectedErr):
			t.Fatalf("error %q does not contain %q", err.Error(), tc.expectedErr)
		}
		if tc.expectedErr != "" {
			return
		}
		require.Equal(t, tc.expectedWarnings, result.Warnings, "warnings")

		// build a default configuration, then patch the fields we expect to change
		// and compare it with the generated configuration. Since the expected
		// runtime config has been validated we do not need to validate it again.
		expectedOpts := LoadOpts{}
		patchLoadOptsShims(&expectedOpts)
		x, err := newBuilder(expectedOpts)
		require.NoError(t, err)

		expected, err := x.build()
		require.NoError(t, err)
		if tc.expected != nil {
			tc.expected(&expected)
		}

		actual := *result.RuntimeConfig
		// both DataDir fields should always be the same, so test for the
		// invariant, and than updated the expected, so that every test
		// case does not need to set this field.
		require.Equal(t, actual.DataDir, actual.ACLTokens.DataDir)
		expected.ACLTokens.DataDir = actual.ACLTokens.DataDir
		// These fields are always the same
		expected.ACLResolverSettings.Datacenter = expected.Datacenter
		expected.ACLResolverSettings.ACLsEnabled = expected.ACLsEnabled
		expected.ACLResolverSettings.NodeName = expected.NodeName
		expected.ACLResolverSettings.EnterpriseMeta = *structs.NodeEnterpriseMetaInPartition(expected.PartitionOrDefault())

		prototest.AssertDeepEqual(t, expected, actual, cmpopts.EquateEmpty())
	}
}

func runCase(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		t.Log("case:", name)
		fn(t)
	})
}

func TestLoad_InvalidConfigFormat(t *testing.T) {
	_, err := Load(LoadOpts{ConfigFormat: "yaml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "-config-format must be either 'hcl' or 'json'")
}

// TestFullConfig tests the conversion from a fully populated JSON or
// HCL config file to a RuntimeConfig structure. All fields must be set
// to a unique non-zero value.
func TestLoad_FullConfig(t *testing.T) {
	dataDir := testutil.TempDir(t, "consul")

	cidr := func(s string) *net.IPNet {
		_, n, _ := net.ParseCIDR(s)
		return n
	}

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()
	nodeEntMeta := structs.NodeEnterpriseMetaInDefaultPartition()
	expected := &RuntimeConfig{
		// non-user configurable values
		AEInterval:                     time.Minute,
		CheckDeregisterIntervalMin:     time.Minute,
		CheckReapInterval:              30 * time.Second,
		SegmentNameLimit:               64,
		SyncCoordinateIntervalMin:      15 * time.Second,
		SyncCoordinateRateTarget:       64,
		LocalProxyConfigResyncInterval: 30 * time.Second,

		Revision:          "JNtPSav3",
		Version:           "R909Hblt",
		VersionPrerelease: "ZT1JOQLn",
		VersionMetadata:   "GtTCa13",
		BuildDate:         time.Date(2019, 11, 20, 5, 0, 0, 0, time.UTC),

		// consul configuration
		ConsulCoordinateUpdateBatchSize:  128,
		ConsulCoordinateUpdateMaxBatches: 5,
		ConsulCoordinateUpdatePeriod:     5 * time.Second,
		ConsulRaftElectionTimeout:        5 * time.Second,
		ConsulRaftHeartbeatTimeout:       5 * time.Second,
		ConsulRaftLeaderLeaseTimeout:     2500 * time.Millisecond,
		GossipLANGossipInterval:          25252 * time.Second,
		GossipLANGossipNodes:             6,
		GossipLANProbeInterval:           101 * time.Millisecond,
		GossipLANProbeTimeout:            102 * time.Millisecond,
		GossipLANSuspicionMult:           1235,
		GossipLANRetransmitMult:          1234,
		GossipWANGossipInterval:          6966 * time.Second,
		GossipWANGossipNodes:             2,
		GossipWANProbeInterval:           103 * time.Millisecond,
		GossipWANProbeTimeout:            104 * time.Millisecond,
		GossipWANSuspicionMult:           16385,
		GossipWANRetransmitMult:          16384,
		ConsulServerHealthInterval:       2 * time.Second,

		// user configurable values

		ACLTokens: token.Config{
			EnablePersistence:     true,
			DataDir:               dataDir,
			ACLDefaultToken:       "418fdff1",
			ACLAgentToken:         "bed2377c",
			ACLAgentRecoveryToken: "1dba6aba",
			ACLReplicationToken:   "5795983a",
		},

		ACLsEnabled:       true,
		PrimaryDatacenter: "ejtmd43d",
		ACLResolverSettings: consul.ACLResolverSettings{
			ACLsEnabled:      true,
			Datacenter:       "rzo029wg",
			NodeName:         "otlLxGaI",
			EnterpriseMeta:   *nodeEntMeta,
			ACLDefaultPolicy: "72c2e7a0",
			ACLDownPolicy:    "03eb2aee",
			ACLTokenTTL:      3321 * time.Second,
			ACLPolicyTTL:     1123 * time.Second,
			ACLRoleTTL:       9876 * time.Second,
		},
		ACLEnableKeyListPolicy:           true,
		ACLInitialManagementToken:        "3820e09a",
		ACLTokenReplication:              true,
		AdvertiseAddrLAN:                 ipAddr("17.99.29.16"),
		AdvertiseAddrWAN:                 ipAddr("78.63.37.19"),
		AdvertiseReconnectTimeout:        0 * time.Second,
		AutopilotCleanupDeadServers:      true,
		AutopilotDisableUpgradeMigration: true,
		AutopilotLastContactThreshold:    12705 * time.Second,
		AutopilotMaxTrailingLogs:         17849,
		AutopilotMinQuorum:               3,
		AutopilotRedundancyZoneTag:       "3IsufDJf",
		AutopilotServerStabilizationTime: 23057 * time.Second,
		AutopilotUpgradeVersionTag:       "W9pDwFAL",
		BindAddr:                         ipAddr("16.99.34.17"),
		BootstrapExpect:                  53,
		Cache: cache.Options{
			EntryFetchMaxBurst: 42,
			EntryFetchRate:     0.334,
		},
		CheckOutputMaxSize: checks.DefaultBufSize,
		Checks: []*structs.CheckDefinition{
			{
				ID:         "uAjE6m9Z",
				Name:       "QsZRGpYr",
				Notes:      "VJ7Sk4BY",
				ServiceID:  "lSulPcyz",
				Token:      "toO59sh8",
				Status:     "9RlWsXMV",
				ScriptArgs: []string{"4BAJttck", "4D2NPtTQ"},
				HTTP:       "dohLcyQ2",
				Header: map[string][]string{
					"ZBfTin3L": {"1sDbEqYG", "lJGASsWK"},
					"Ui0nU99X": {"LMccm3Qe", "k5H5RggQ"},
				},
				Method:                         "aldrIQ4l",
				Body:                           "wSjTy7dg",
				DisableRedirects:               true,
				TCP:                            "RJQND605",
				H2PING:                         "9N1cSb5B",
				H2PingUseTLS:                   false,
				OSService:                      "aAjE6m9Z",
				Interval:                       22164 * time.Second,
				OutputMaxSize:                  checks.DefaultBufSize,
				DockerContainerID:              "ipgdFtjd",
				Shell:                          "qAeOYy0M",
				TLSServerName:                  "bdeb5f6a",
				TLSSkipVerify:                  true,
				Timeout:                        1813 * time.Second,
				DeregisterCriticalServiceAfter: 14232 * time.Second,
			},
			{
				ID:         "Cqq95BhP",
				Name:       "3qXpkS0i",
				Notes:      "sb5qLTex",
				ServiceID:  "CmUUcRna",
				Token:      "a3nQzHuy",
				Status:     "irj26nf3",
				ScriptArgs: []string{"9s526ogY", "gSlOHj1w"},
				HTTP:       "yzhgsQ7Y",
				Header: map[string][]string{
					"zcqwA8dO": {"qb1zx0DL", "sXCxPFsD"},
					"qxvdnSE9": {"6wBPUYdF", "YYh8wtSZ"},
				},
				Method:                         "gLrztrNw",
				Body:                           "0jkKgGUC",
				DisableRedirects:               false,
				OutputMaxSize:                  checks.DefaultBufSize,
				TCP:                            "4jG5casb",
				H2PING:                         "HCHU7gEb",
				H2PingUseTLS:                   false,
				OSService:                      "aqq95BhP",
				Interval:                       28767 * time.Second,
				DockerContainerID:              "THW6u7rL",
				Shell:                          "C1Zt3Zwh",
				TLSServerName:                  "6adc3bfb",
				TLSSkipVerify:                  true,
				Timeout:                        18506 * time.Second,
				DeregisterCriticalServiceAfter: 2366 * time.Second,
			},
			{
				ID:         "fZaCAXww",
				Name:       "OOM2eo0f",
				Notes:      "zXzXI9Gt",
				ServiceID:  "L8G0QNmR",
				Token:      "oo4BCTgJ",
				Status:     "qLykAl5u",
				ScriptArgs: []string{"f3BemRjy", "e5zgpef7"},
				HTTP:       "29B93haH",
				Header: map[string][]string{
					"hBq0zn1q": {"2a9o9ZKP", "vKwA5lR6"},
					"f3r6xFtM": {"RyuIdDWv", "QbxEcIUM"},
				},
				Method:                         "Dou0nGT5",
				Body:                           "5PBQd2OT",
				DisableRedirects:               true,
				OutputMaxSize:                  checks.DefaultBufSize,
				TCP:                            "JY6fTTcw",
				H2PING:                         "rQ8eyCSF",
				H2PingUseTLS:                   false,
				OSService:                      "aZaCAXww",
				Interval:                       18714 * time.Second,
				DockerContainerID:              "qF66POS9",
				Shell:                          "sOnDy228",
				TLSServerName:                  "7BdnzBYk",
				TLSSkipVerify:                  true,
				Timeout:                        5954 * time.Second,
				DeregisterCriticalServiceAfter: 13209 * time.Second,
			},
		},
		CheckUpdateInterval: 16507 * time.Second,
		ClientAddrs:         []*net.IPAddr{ipAddr("93.83.18.19")},
		ConfigEntryBootstrap: []structs.ConfigEntry{
			&structs.ProxyConfigEntry{
				Kind:           structs.ProxyDefaults,
				Name:           structs.ProxyConfigGlobal,
				EnterpriseMeta: *defaultEntMeta,
				Config: map[string]interface{}{
					"foo": "bar",
					// has to be a float due to being a map[string]interface
					"bar": float64(1),
				},
			},
		},
		AutoEncryptTLS:      false,
		AutoEncryptDNSSAN:   []string{"a.com", "b.com"},
		AutoEncryptIPSAN:    []net.IP{net.ParseIP("192.168.4.139"), net.ParseIP("192.168.4.140")},
		AutoEncryptAllowTLS: true,
		AutoConfig: AutoConfig{
			Enabled:         false,
			IntroToken:      "OpBPGRwt",
			IntroTokenFile:  "gFvAXwI8",
			DNSSANs:         []string{"6zdaWg9J"},
			IPSANs:          []net.IP{net.IPv4(198, 18, 99, 99)},
			ServerAddresses: []string{"198.18.100.1"},
			Authorizer: AutoConfigAuthorizer{
				Enabled:         true,
				AllowReuse:      true,
				ClaimAssertions: []string{"value.node == \"${node}\""},
				AuthMethod: structs.ACLAuthMethod{
					Name:           "Auto Config Authorizer",
					Type:           "jwt",
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					Config: map[string]interface{}{
						"JWTValidationPubKeys": []string{"-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAERVchfCZng4mmdvQz1+sJHRN40snC\nYt8NjYOnbnScEXMkyoUmASr88gb7jaVAVt3RYASAbgBjB2Z+EUizWkx5Tg==\n-----END PUBLIC KEY-----"},
						"ClaimMappings": map[string]string{
							"node": "node",
						},
						"BoundIssuer":    "consul",
						"BoundAudiences": []string{"consul-cluster-1"},
						"ListClaimMappings": map[string]string{
							"foo": "bar",
						},
						"OIDCDiscoveryURL":    "",
						"OIDCDiscoveryCACert": "",
						"JWKSURL":             "",
						"JWKSCACert":          "",
						"ExpirationLeeway":    0 * time.Second,
						"NotBeforeLeeway":     0 * time.Second,
						"ClockSkewLeeway":     0 * time.Second,
						"JWTSupportedAlgs":    []string(nil),
					},
				},
			},
		},
		ConnectEnabled:        true,
		ConnectSidecarMinPort: 8888,
		ConnectSidecarMaxPort: 9999,
		ExposeMinPort:         1111,
		ExposeMaxPort:         2222,
		ConnectCAProvider:     "consul",
		ConnectCAConfig: map[string]interface{}{
			"IntermediateCertTTL": "8760h",
			"LeafCertTTL":         "1h",
			"RootCertTTL":         "96360h",
			"CSRMaxPerSecond":     float64(100),
			"CSRMaxConcurrent":    float64(2),
		},
		ConnectMeshGatewayWANFederationEnabled: false,
		Cloud: hcpconfig.CloudConfig{
			ResourceID:   "N43DsscE",
			ClientID:     "6WvsDZCP",
			ClientSecret: "lCSMHOpB",
			Hostname:     "DH4bh7aC",
			AuthURL:      "332nCdR2",
			ScadaAddress: "aoeusth232",
		},
		DNSAddrs:                         []net.Addr{tcpAddr("93.95.95.81:7001"), udpAddr("93.95.95.81:7001")},
		DNSARecordLimit:                  29907,
		DNSAllowStale:                    true,
		DNSDisableCompression:            true,
		DNSDomain:                        "7W1xXSqd",
		DNSAltDomain:                     "1789hsd",
		DNSEnableTruncate:                true,
		DNSMaxStale:                      29685 * time.Second,
		DNSNodeTTL:                       7084 * time.Second,
		DNSOnlyPassing:                   true,
		DNSPort:                          7001,
		DNSRecursorStrategy:              "sequential",
		DNSRecursorTimeout:               4427 * time.Second,
		DNSRecursors:                     []string{"63.38.39.58", "92.49.18.18"},
		DNSSOA:                           RuntimeSOAConfig{Refresh: 3600, Retry: 600, Expire: 86400, Minttl: 0},
		DNSServiceTTL:                    map[string]time.Duration{"*": 32030 * time.Second},
		DNSUDPAnswerLimit:                29909,
		DNSNodeMetaTXT:                   true,
		DNSUseCache:                      true,
		DNSCacheMaxAge:                   5 * time.Minute,
		DataDir:                          dataDir,
		Datacenter:                       "rzo029wg",
		DefaultQueryTime:                 16743 * time.Second,
		DisableAnonymousSignature:        true,
		DisableCoordinates:               true,
		DisableHostNodeID:                true,
		DisableHTTPUnprintableCharFilter: true,
		DisableKeyringFile:               true,
		DisableRemoteExec:                true,
		DisableUpdateCheck:               true,
		DiscardCheckOutput:               true,
		DiscoveryMaxStale:                5 * time.Second,
		EnableAgentTLSForChecks:          true,
		EnableCentralServiceConfig:       false,
		EnableDebug:                      true,
		EnableRemoteScriptChecks:         true,
		EnableLocalScriptChecks:          true,
		EncryptKey:                       "A4wELWqH",
		StaticRuntimeConfig: StaticRuntimeConfig{
			EncryptVerifyIncoming: true,
			EncryptVerifyOutgoing: true,
		},

		GRPCPort:              4881,
		GRPCAddrs:             []net.Addr{tcpAddr("32.31.61.91:4881")},
		GRPCTLSPort:           5201,
		GRPCTLSAddrs:          []net.Addr{tcpAddr("23.14.88.19:5201")},
		HTTPAddrs:             []net.Addr{tcpAddr("83.39.91.39:7999")},
		HTTPBlockEndpoints:    []string{"RBvAFcGD", "fWOWFznh"},
		AllowWriteHTTPFrom:    []*net.IPNet{cidr("127.0.0.0/8"), cidr("22.33.44.55/32"), cidr("0.0.0.0/0")},
		HTTPPort:              7999,
		HTTPResponseHeaders:   map[string]string{"M6TKa9NP": "xjuxjOzQ", "JRCrHZed": "rl0mTx81"},
		HTTPSAddrs:            []net.Addr{tcpAddr("95.17.17.19:15127")},
		HTTPMaxConnsPerClient: 100,
		HTTPMaxHeaderBytes:    10,
		HTTPSHandshakeTimeout: 2391 * time.Millisecond,
		HTTPSPort:             15127,
		HTTPUseCache:          false,
		KVMaxValueSize:        1234567800,
		LeaveDrainTime:        8265 * time.Second,
		LeaveOnTerm:           true,
		Logging: logging.Config{
			LogLevel:       "k1zo9Spt",
			LogJSON:        true,
			EnableSyslog:   true,
			SyslogFacility: "hHv79Uia",
		},
		MaxQueryTime:            18237 * time.Second,
		NodeID:                  types.NodeID("AsUIlw99"),
		NodeMeta:                map[string]string{"5mgGQMBk": "mJLtVMSG", "A7ynFMJB": "0Nx6RGab"},
		NodeName:                "otlLxGaI",
		ReadReplica:             true,
		PeeringEnabled:          true,
		PidFile:                 "43xN80Km",
		PrimaryGateways:         []string{"aej8eeZo", "roh2KahS"},
		PrimaryGatewaysInterval: 18866 * time.Second,
		RPCAdvertiseAddr:        tcpAddr("17.99.29.16:3757"),
		RPCBindAddr:             tcpAddr("16.99.34.17:3757"),
		RPCHandshakeTimeout:     1932 * time.Millisecond,
		RPCClientTimeout:        62 * time.Second,
		RPCHoldTimeout:          15707 * time.Second,
		RPCProtocol:             30793,
		RPCRateLimit:            12029.43,
		RPCMaxBurst:             44848,
		RPCMaxConnsPerClient:    2954,
		RaftProtocol:            3,
		RaftSnapshotThreshold:   16384,
		RaftSnapshotInterval:    30 * time.Second,
		RaftTrailingLogs:        83749,
		ReconnectTimeoutLAN:     23739 * time.Second,
		ReconnectTimeoutWAN:     26694 * time.Second,
		RequestLimitsMode:       consulrate.ModePermissive,
		RequestLimitsReadRate:   99.0,
		RequestLimitsWriteRate:  101.0,
		RejoinAfterLeave:        true,
		RetryJoinIntervalLAN:    8067 * time.Second,
		RetryJoinIntervalWAN:    28866 * time.Second,
		RetryJoinLAN:            []string{"pbsSFY7U", "l0qLtWij", "LR3hGDoG", "MwVpZ4Up"},
		RetryJoinMaxAttemptsLAN: 913,
		RetryJoinMaxAttemptsWAN: 23160,
		RetryJoinWAN:            []string{"PFsR02Ye", "rJdQIhER", "EbFSc3nA", "kwXTh623"},
		RPCConfig:               consul.RPCConfig{EnableStreaming: true},
		SegmentLimit:            123,
		SerfPortLAN:             8301,
		SerfPortWAN:             8302,
		ServerMode:              true,
		ServerName:              "Oerr9n1G",
		ServerRejoinAgeMax:      604800 * time.Second,
		ServerPort:              3757,
		Services: []*structs.ServiceDefinition{
			{
				ID:      "wI1dzxS4",
				Name:    "7IszXMQ1",
				Tags:    []string{"0Zwg8l6v", "zebELdN5"},
				Address: "9RhqPSPB",
				Token:   "myjKJkWH",
				Port:    72219,
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
				EnableTagOverride: true,
				Checks: []*structs.CheckType{
					{
						CheckID:    "qmfeO5if",
						Name:       "atDGP7n5",
						Status:     "pDQKEhWL",
						Notes:      "Yt8EDLev",
						ScriptArgs: []string{"81EDZLPa", "bPY5X8xd"},
						HTTP:       "qzHYvmJO",
						Header: map[string][]string{
							"UkpmZ3a3": {"2dfzXuxZ"},
							"cVFpko4u": {"gGqdEB6k", "9LsRo22u"},
						},
						Method:                         "X5DrovFc",
						Body:                           "WeikigLh",
						DisableRedirects:               true,
						OutputMaxSize:                  checks.DefaultBufSize,
						TCP:                            "ICbxkpSF",
						H2PING:                         "7s7BbMyb",
						H2PingUseTLS:                   false,
						OSService:                      "amfeO5if",
						Interval:                       24392 * time.Second,
						DockerContainerID:              "ZKXr68Yb",
						Shell:                          "CEfzx0Fo",
						TLSServerName:                  "4f191d4F",
						TLSSkipVerify:                  true,
						Timeout:                        38333 * time.Second,
						DeregisterCriticalServiceAfter: 44214 * time.Second,
					},
				},
				// Note that although this SidecarService is only syntax sugar for
				// registering another service, that has to happen in the agent code so
				// it can make intelligent decisions about automatic port assignments
				// etc. So we expect config just to pass it through verbatim.
				Connect: &structs.ServiceConnect{
					SidecarService: &structs.ServiceDefinition{
						Weights: &structs.Weights{
							Passing: 1,
							Warning: 1,
						},
					},
				},
			},
			{
				ID:      "MRHVMZuD",
				Name:    "6L6BVfgH",
				Tags:    []string{"7Ale4y6o", "PMBW08hy"},
				Address: "R6H6g8h0",
				Token:   "ZgY8gjMI",
				Port:    38292,
				Weights: &structs.Weights{
					Passing: 1979,
					Warning: 6,
				},
				EnableTagOverride: true,
				Checks: structs.CheckTypes{
					&structs.CheckType{
						CheckID:    "GTti9hCo",
						Name:       "9OOS93ne",
						Notes:      "CQy86DH0",
						Status:     "P0SWDvrk",
						ScriptArgs: []string{"EXvkYIuG", "BATOyt6h"},
						HTTP:       "u97ByEiW",
						Header: map[string][]string{
							"MUlReo8L": {"AUZG7wHG", "gsN0Dc2N"},
							"1UJXjVrT": {"OJgxzTfk", "xZZrFsq7"},
						},
						Method:                         "5wkAxCUE",
						Body:                           "7CRjCJyz",
						OutputMaxSize:                  checks.DefaultBufSize,
						TCP:                            "MN3oA9D2",
						H2PING:                         "OV6Q2XEg",
						H2PingUseTLS:                   false,
						OSService:                      "GTti9hCA",
						Interval:                       32718 * time.Second,
						DockerContainerID:              "cU15LMet",
						Shell:                          "nEz9qz2l",
						TLSServerName:                  "f43ouY7a",
						TLSSkipVerify:                  true,
						Timeout:                        34738 * time.Second,
						DeregisterCriticalServiceAfter: 84282 * time.Second,
					},
					&structs.CheckType{
						CheckID:                        "UHsDeLxG",
						Name:                           "PQSaPWlT",
						Notes:                          "jKChDOdl",
						Status:                         "5qFz6OZn",
						OutputMaxSize:                  checks.DefaultBufSize,
						Timeout:                        4868 * time.Second,
						TTL:                            11222 * time.Second,
						DeregisterCriticalServiceAfter: 68482 * time.Second,
					},
				},
				Connect: &structs.ServiceConnect{},
			},
			{
				ID:   "Kh81CPF6",
				Name: "Kh81CPF6-proxy",
				Port: 31471,
				Kind: "connect-proxy",
				Proxy: &structs.ConnectProxyConfig{
					DestinationServiceName: "6L6BVfgH",
					DestinationServiceID:   "6L6BVfgH-id",
					LocalServiceAddress:    "127.0.0.2",
					LocalServicePort:       23759,
					Config: map[string]interface{}{
						"cedGGtZf": "pWrUNiWw",
					},
					Upstreams: structs.Upstreams{
						{
							DestinationType:      "service", // Default should be explicitly filled
							DestinationName:      "KPtAj2cb",
							DestinationPartition: defaultEntMeta.PartitionOrEmpty(),
							DestinationNamespace: defaultEntMeta.NamespaceOrEmpty(),
							LocalBindPort:        4051,
							Config: map[string]interface{}{
								"kzRnZOyd": "nUNKoL8H",
							},
						},
						{
							DestinationType:      "prepared_query",
							DestinationNamespace: "9nakw0td",
							DestinationPartition: "part-9nakw0td",
							DestinationName:      "KSd8HsRl",
							LocalBindPort:        11884,
							LocalBindAddress:     "127.24.88.0",
						},
						{
							DestinationType:      "prepared_query",
							DestinationNamespace: "9nakw0td",
							DestinationPartition: "part-9nakw0td",
							DestinationName:      "placeholder",
							LocalBindSocketPath:  "/foo/bar/upstream",
							LocalBindSocketMode:  "0600",
						},
					},
					Expose: structs.ExposeConfig{
						Checks: true,
						Paths: []structs.ExposePath{
							{
								Path:          "/health",
								LocalPathPort: 8080,
								ListenerPort:  21500,
								Protocol:      "http",
							},
						},
					},
					Mode: structs.ProxyModeTransparent,
					TransparentProxy: structs.TransparentProxyConfig{
						OutboundListenerPort: 10101,
						DialedDirectly:       true,
					},
				},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
			},
			{
				ID:   "kvVqbwSE",
				Kind: "mesh-gateway",
				Name: "gw-primary-dc",
				Port: 27147,
				Proxy: &structs.ConnectProxyConfig{
					Config: map[string]interface{}{
						"1CuJHVfw": "Kzqsa7yc",
					},
					Upstreams: structs.Upstreams{},
				},
				Weights: &structs.Weights{
					Passing: 1,
					Warning: 1,
				},
			},
			{
				ID:   "dLOXpSCI",
				Name: "o1ynPkp0",
				TaggedAddresses: map[string]structs.ServiceAddress{
					"lan": {
						Address: "2d79888a",
						Port:    2143,
					},
					"wan": {
						Address: "d4db85e2",
						Port:    6109,
					},
				},
				Tags:    []string{"nkwshvM5", "NTDWn3ek"},
				Address: "cOlSOhbp",
				Token:   "msy7iWER",
				Meta:    map[string]string{"mymeta": "data"},
				Port:    24237,
				Weights: &structs.Weights{
					Passing: 100,
					Warning: 1,
				},
				EnableTagOverride: true,
				Connect: &structs.ServiceConnect{
					Native: true,
				},
				Checks: structs.CheckTypes{
					&structs.CheckType{
						CheckID:    "Zv99e9Ka",
						Name:       "sgV4F7Pk",
						Notes:      "yP5nKbW0",
						Status:     "7oLMEyfu",
						ScriptArgs: []string{"5wEZtZpv", "0Ihyk8cS"},
						HTTP:       "KyDjGY9H",
						Header: map[string][]string{
							"gv5qefTz": {"5Olo2pMG", "PvvKWQU5"},
							"SHOVq1Vv": {"jntFhyym", "GYJh32pp"},
						},
						Method:                         "T66MFBfR",
						Body:                           "OwGjTFQi",
						DisableRedirects:               true,
						OutputMaxSize:                  checks.DefaultBufSize,
						TCP:                            "bNnNfx2A",
						H2PING:                         "qC1pidiW",
						H2PingUseTLS:                   false,
						OSService:                      "ZA99e9Ka",
						Interval:                       22224 * time.Second,
						DockerContainerID:              "ipgdFtjd",
						Shell:                          "omVZq7Sz",
						TLSServerName:                  "axw5QPL5",
						TLSSkipVerify:                  true,
						Timeout:                        18913 * time.Second,
						DeregisterCriticalServiceAfter: 8482 * time.Second,
					},
					&structs.CheckType{
						CheckID:    "G79O6Mpr",
						Name:       "IEqrzrsd",
						Notes:      "SVqApqeM",
						Status:     "XXkVoZXt",
						ScriptArgs: []string{"wD05Bvao", "rLYB7kQC"},
						HTTP:       "kyICZsn8",
						Header: map[string][]string{
							"4ebP5vL4": {"G20SrL5Q", "DwPKlMbo"},
							"p2UI34Qz": {"UsG1D0Qh", "NHhRiB6s"},
						},
						Method:                         "ciYHWors",
						Body:                           "lUVLGYU7",
						DisableRedirects:               false,
						OutputMaxSize:                  checks.DefaultBufSize,
						TCP:                            "FfvCwlqH",
						H2PING:                         "spI3muI3",
						H2PingUseTLS:                   false,
						OSService:                      "GAaO6Mpr",
						Interval:                       12356 * time.Second,
						DockerContainerID:              "HBndBU6R",
						Shell:                          "hVI33JjA",
						TLSServerName:                  "7uwWOnUS",
						TLSSkipVerify:                  true,
						Timeout:                        38282 * time.Second,
						DeregisterCriticalServiceAfter: 4992 * time.Second,
					},
					&structs.CheckType{
						CheckID:    "RMi85Dv8",
						Name:       "iehanzuq",
						Status:     "rCvn53TH",
						Notes:      "fti5lfF3",
						ScriptArgs: []string{"16WRUmwS", "QWk7j7ae"},
						HTTP:       "dl3Fgme3",
						Header: map[string][]string{
							"rjm4DEd3": {"2m3m2Fls"},
							"l4HwQ112": {"fk56MNlo", "dhLK56aZ"},
						},
						Method:                         "9afLm3Mj",
						Body:                           "wVVL2V6f",
						DisableRedirects:               true,
						OutputMaxSize:                  checks.DefaultBufSize,
						TCP:                            "fjiLFqVd",
						H2PING:                         "5NbNWhan",
						H2PingUseTLS:                   false,
						OSService:                      "RAa85Dv8",
						Interval:                       23926 * time.Second,
						DockerContainerID:              "dO5TtRHk",
						Shell:                          "e6q2ttES",
						TLSServerName:                  "ECSHk8WF",
						TLSSkipVerify:                  true,
						Timeout:                        38483 * time.Second,
						DeregisterCriticalServiceAfter: 68787 * time.Second,
					},
				},
			},
		},
		UseStreamingBackend:  true,
		SerfAdvertiseAddrLAN: tcpAddr("17.99.29.16:8301"),
		SerfAdvertiseAddrWAN: tcpAddr("78.63.37.19:8302"),
		SerfBindAddrLAN:      tcpAddr("99.43.63.15:8301"),
		SerfBindAddrWAN:      tcpAddr("67.88.33.19:8302"),
		SerfAllowedCIDRsLAN:  []net.IPNet{},
		SerfAllowedCIDRsWAN:  []net.IPNet{},
		SessionTTLMin:        26627 * time.Second,
		SkipLeaveOnInt:       true,
		Telemetry: lib.TelemetryConfig{
			CirconusAPIApp:                     "p4QOTe9j",
			CirconusAPIToken:                   "E3j35V23",
			CirconusAPIURL:                     "mEMjHpGg",
			CirconusBrokerID:                   "BHlxUhed",
			CirconusBrokerSelectTag:            "13xy1gHm",
			CirconusCheckDisplayName:           "DRSlQR6n",
			CirconusCheckForceMetricActivation: "Ua5FGVYf",
			CirconusCheckID:                    "kGorutad",
			CirconusCheckInstanceID:            "rwoOL6R4",
			CirconusCheckSearchTag:             "ovT4hT4f",
			CirconusCheckTags:                  "prvO4uBl",
			CirconusSubmissionInterval:         "DolzaflP",
			CirconusSubmissionURL:              "gTcbS93G",
			DisableHostname:                    true,
			DogstatsdAddr:                      "0wSndumK",
			DogstatsdTags:                      []string{"3N81zSUB", "Xtj8AnXZ"},
			RetryFailedConfiguration:           true,
			FilterDefault:                      true,
			AllowedPrefixes:                    []string{"oJotS8XJ"},
			BlockedPrefixes:                    []string{"cazlEhGn", "ftO6DySn.rpc.server.call"},
			MetricsPrefix:                      "ftO6DySn",
			StatsdAddr:                         "drce87cy",
			StatsiteAddr:                       "HpFwKB8R",
			PrometheusOpts: prometheus.PrometheusOpts{
				Expiration: 15 * time.Second,
				Name:       "ftO6DySn", // notice this is the same as the metrics prefix
			},
		},
		TLS: tlsutil.Config{
			InternalRPC: tlsutil.ProtocolConfig{
				VerifyIncoming:       true,
				CAFile:               "mKl19Utl",
				CAPath:               "lOp1nhPa",
				CertFile:             "dfJ4oPln",
				KeyFile:              "aL1Knkpo",
				TLSMinVersion:        types.TLSv1_1,
				CipherSuites:         []types.TLSCipherSuite{types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, types.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA},
				VerifyOutgoing:       true,
				VerifyServerHostname: true,
			},
			GRPC: tlsutil.ProtocolConfig{
				VerifyIncoming: true,
				CAFile:         "lOp1nhJk",
				CAPath:         "fLponKpl",
				CertFile:       "a674klPn",
				KeyFile:        "1y4prKjl",
				TLSMinVersion:  types.TLSv1_0,
				CipherSuites:   []types.TLSCipherSuite{types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, types.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA},
				VerifyOutgoing: false,
				UseAutoCert:    true,
			},
			HTTPS: tlsutil.ProtocolConfig{
				VerifyIncoming: true,
				CAFile:         "7Yu1PolM",
				CAPath:         "nu4PlHzn",
				CertFile:       "1yrhPlMk",
				KeyFile:        "1bHapOkL",
				TLSMinVersion:  types.TLSv1_3,
				VerifyOutgoing: true,
			},
			NodeName:                "otlLxGaI",
			ServerName:              "Oerr9n1G",
			ServerMode:              true,
			Domain:                  "7W1xXSqd",
			EnableAgentTLSForChecks: true,
		},
		TaggedAddresses: map[string]string{
			"7MYgHrYH": "dALJAhLD",
			"h6DdBy6K": "ebrr9zZ8",
			"lan":      "17.99.29.16",
			"lan_ipv4": "17.99.29.16",
			"wan":      "78.63.37.19",
			"wan_ipv4": "78.63.37.19",
		},
		TranslateWANAddrs: true,
		TxnMaxReqLen:      567800000,
		UIConfig: UIConfig{
			Dir:                        "pVncV4Ey",
			ContentPath:                "/qp1WRhYH/", // slashes are added in parsing
			MetricsProvider:            "sgnaoa_lower_case",
			MetricsProviderFiles:       []string{"sgnaMFoa", "dicnwkTH"},
			MetricsProviderOptionsJSON: "{\"DIbVQadX\": 1}",
			MetricsProxy: UIMetricsProxy{
				BaseURL: "http://foo.bar",
				AddHeaders: []UIMetricsProxyAddHeader{
					{
						Name:  "p3nynwc9",
						Value: "TYBgnN2F",
					},
				},
				PathAllowlist: []string{"/aSh3cu", "/eiK/2Th"},
			},
			DashboardURLTemplates: map[string]string{"u2eziu2n_lower_case": "http://lkjasd.otr"},
		},
		UnixSocketUser:  "E0nB1DwA",
		UnixSocketGroup: "8pFodrV8",
		UnixSocketMode:  "E8sAwOv4",
		Watches: []map[string]interface{}{
			{
				"type":       "key",
				"datacenter": "GyE6jpeW",
				"key":        "j9lF1Tve",
				"handler":    "90N7S4LN",
			},
			{
				"type":       "keyprefix",
				"datacenter": "fYrl3F5d",
				"key":        "sl3Dffu7",
				"args":       []interface{}{"dltjDJ2a", "flEa7C2d"},
			},
		},
		XDSUpdateRateLimit: 9526.2,
		RaftLogStoreConfig: consul.RaftLogStoreConfig{
			Backend:         consul.LogStoreBackendWAL,
			DisableLogCache: true,
			Verification: consul.RaftLogStoreVerificationConfig{
				Enabled:  true,
				Interval: 12345 * time.Second,
			},
			BoltDB: consul.RaftBoltDBConfig{NoFreelistSync: true},
			WAL:    consul.WALConfig{SegmentSize: 15 * 1024 * 1024},
		},
		AutoReloadConfigCoalesceInterval: 1 * time.Second,
	}
	entFullRuntimeConfig(expected)

	expectedWarns := []string{
		deprecationWarning("acl_datacenter", "primary_datacenter"),
		deprecationWarning("acl_agent_master_token", "acl.tokens.agent_recovery"),
		deprecationWarning("acl.tokens.agent_master", "acl.tokens.agent_recovery"),
		deprecationWarning("acl_agent_token", "acl.tokens.agent"),
		deprecationWarning("acl_token", "acl.tokens.default"),
		deprecationWarning("acl_master_token", "acl.tokens.initial_management"),
		deprecationWarning("acl.tokens.master", "acl.tokens.initial_management"),
		deprecationWarning("acl_replication_token", "acl.tokens.replication"),
		deprecationWarning("enable_acl_replication", "acl.enable_token_replication"),
		deprecationWarning("acl_default_policy", "acl.default_policy"),
		deprecationWarning("acl_down_policy", "acl.down_policy"),
		deprecationWarning("acl_ttl", "acl.token_ttl"),
		deprecationWarning("acl_enable_key_list_policy", "acl.enable_key_list_policy"),
		`bootstrap_expect > 0: expecting 53 servers`,
		deprecationWarning("ca_file", "tls.defaults.ca_file"),
		deprecationWarning("ca_path", "tls.defaults.ca_path"),
		deprecationWarning("cert_file", "tls.defaults.cert_file"),
		deprecationWarning("key_file", "tls.defaults.key_file"),
		deprecationWarning("tls_cipher_suites", "tls.defaults.tls_cipher_suites"),
		deprecationWarning("tls_min_version", "tls.defaults.tls_min_version"),
		deprecationWarning("verify_incoming", "tls.defaults.verify_incoming"),
		deprecationWarning("verify_incoming_https", "tls.https.verify_incoming"),
		deprecationWarning("verify_incoming_rpc", "tls.internal_rpc.verify_incoming"),
		deprecationWarning("verify_outgoing", "tls.defaults.verify_outgoing"),
		deprecationWarning("verify_server_hostname", "tls.internal_rpc.verify_server_hostname"),
		"The 'tls_prefer_server_cipher_suites' field is deprecated and will be ignored.",
		deprecationWarning("start_join", "retry_join"),
		deprecationWarning("start_join_wan", "retry_join_wan"),
	}
	expectedWarns = append(expectedWarns, enterpriseConfigKeyWarnings...)

	// FIXME: ensure that all fields are set to unique non-zero values.
	// There are many fields that are not set to non-zero values, so this nonZero
	// check does not actually work.
	if err := nonZero("RuntimeConfig", nil, expected); err != nil {
		t.Log(err)
	}

	for _, format := range []string{"json", "hcl"} {
		t.Run(format, func(t *testing.T) {
			opts := LoadOpts{
				ConfigFiles: []string{"testdata/full-config." + format},
				HCL:         []string{fmt.Sprintf(`data_dir = "%s"`, dataDir)},
			}
			opts.Overrides = append(opts.Overrides, versionSource("JNtPSav3", "R909Hblt", "ZT1JOQLn", "GtTCa13",
				time.Date(2019, 11, 20, 5, 0, 0, 0, time.UTC)))
			r, err := Load(opts)
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, expected, r.RuntimeConfig)
			require.ElementsMatch(t, expectedWarns, r.Warnings, "Warnings: %#v", r.Warnings)
		})
	}
}

// nonZero verifies recursively that all fields are set to unique,
// non-zero and non-nil values.
//
// struct: check all fields recursively
// slice: check len > 0 and all values recursively
// ptr: check not nil
// bool: check not zero (cannot check uniqueness)
// string, int, uint: check not zero and unique
// other: error
func nonZero(name string, uniq map[interface{}]string, v interface{}) error {
	if v == nil {
		return fmt.Errorf("%q is nil", name)
	}

	if uniq == nil {
		uniq = map[interface{}]string{}
	}

	isUnique := func(v interface{}) error {
		if other := uniq[v]; other != "" {
			return fmt.Errorf("%q and %q both use value %q", name, other, v)
		}
		uniq[v] = name
		return nil
	}

	val, typ := reflect.ValueOf(v), reflect.TypeOf(v)
	// fmt.Printf("%s: %T\n", name, v)
	switch typ.Kind() {
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			fieldname := fmt.Sprintf("%s.%s", name, f.Name)
			err := nonZero(fieldname, uniq, val.Field(i).Interface())
			if err != nil {
				return err
			}
		}

	case reflect.Slice:
		if val.Len() == 0 {
			return fmt.Errorf("%q is empty slice", name)
		}
		for i := 0; i < val.Len(); i++ {
			elemname := fmt.Sprintf("%s[%d]", name, i)
			err := nonZero(elemname, uniq, val.Index(i).Interface())
			if err != nil {
				return err
			}
		}

	case reflect.Map:
		if val.Len() == 0 {
			return fmt.Errorf("%q is empty map", name)
		}
		for _, key := range val.MapKeys() {
			keyname := fmt.Sprintf("%s[%s]", name, key.String())
			if err := nonZero(keyname, uniq, key.Interface()); err != nil {
				if strings.Contains(err.Error(), "is zero value") {
					return fmt.Errorf("%q has zero value map key", name)
				}
				return err
			}
			if err := nonZero(keyname, uniq, val.MapIndex(key).Interface()); err != nil {
				return err
			}
		}

	case reflect.Bool:
		if val.Bool() != true {
			return fmt.Errorf("%q is zero value", name)
		}
		// do not test bool for uniqueness since there are only two values

	case reflect.String:
		if val.Len() == 0 {
			return fmt.Errorf("%q is zero value", name)
		}
		return isUnique(v)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val.Int() == 0 {
			return fmt.Errorf("%q is zero value", name)
		}
		return isUnique(v)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val.Uint() == 0 {
			return fmt.Errorf("%q is zero value", name)
		}
		return isUnique(v)

	case reflect.Float32, reflect.Float64:
		if val.Float() == 0 {
			return fmt.Errorf("%q is zero value", name)
		}
		return isUnique(v)

	case reflect.Ptr:
		if val.IsNil() {
			return fmt.Errorf("%q is nil", name)
		}
		return nonZero("*"+name, uniq, val.Elem().Interface())

	default:
		return fmt.Errorf("%T is not supported", v)
	}
	return nil
}

func TestNonZero(t *testing.T) {
	tests := []struct {
		desc string
		v    interface{}
		err  error
	}{
		{"nil", nil, errors.New(`"x" is nil`)},
		{"zero bool", false, errors.New(`"x" is zero value`)},
		{"zero string", "", errors.New(`"x" is zero value`)},
		{"zero int", int(0), errors.New(`"x" is zero value`)},
		{"zero int8", int8(0), errors.New(`"x" is zero value`)},
		{"zero int16", int16(0), errors.New(`"x" is zero value`)},
		{"zero int32", int32(0), errors.New(`"x" is zero value`)},
		{"zero int64", int64(0), errors.New(`"x" is zero value`)},
		{"zero uint", uint(0), errors.New(`"x" is zero value`)},
		{"zero uint8", uint8(0), errors.New(`"x" is zero value`)},
		{"zero uint16", uint16(0), errors.New(`"x" is zero value`)},
		{"zero uint32", uint32(0), errors.New(`"x" is zero value`)},
		{"zero uint64", uint64(0), errors.New(`"x" is zero value`)},
		{"zero float32", float32(0), errors.New(`"x" is zero value`)},
		{"zero float64", float64(0), errors.New(`"x" is zero value`)},
		{"ptr to zero value", pString(""), errors.New(`"*x" is zero value`)},
		{"empty slice", []string{}, errors.New(`"x" is empty slice`)},
		{"slice with zero value", []string{""}, errors.New(`"x[0]" is zero value`)},
		{"empty map", map[string]string{}, errors.New(`"x" is empty map`)},
		{"map with zero value key", map[string]string{"": "y"}, errors.New(`"x" has zero value map key`)},
		{"map with zero value elem", map[string]string{"y": ""}, errors.New(`"x[y]" is zero value`)},
		{"struct with nil field", struct{ Y *int }{}, errors.New(`"x.Y" is nil`)},
		{"struct with zero value field", struct{ Y string }{}, errors.New(`"x.Y" is zero value`)},
		{"struct with empty array", struct{ Y []string }{}, errors.New(`"x.Y" is empty slice`)},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.err, nonZero("x", nil, tt.v))
		})
	}
}

func TestConfigDecodeBytes(t *testing.T) {
	// Test with some input
	src := []byte("abc")
	key := base64.StdEncoding.EncodeToString(src)

	result, err := decodeBytes(key)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(src, result) {
		t.Fatalf("bad: %#v", result)
	}

	// Test with no input
	result, err = decodeBytes("")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) > 0 {
		t.Fatalf("bad: %#v", result)
	}
}

func parseCIDR(t *testing.T, cidr string) *net.IPNet {
	_, x, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("CIDRParse: %v", err)
	}
	return x
}

func TestRuntimeConfig_Sanitize(t *testing.T) {
	rt := RuntimeConfig{
		BindAddr:             &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
		BuildDate:            time.Date(2019, 11, 20, 5, 0, 0, 0, time.UTC),
		CheckOutputMaxSize:   checks.DefaultBufSize,
		SerfAdvertiseAddrLAN: &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
		DNSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
			&net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
		},
		DNSSOA: RuntimeSOAConfig{Refresh: 3600, Retry: 600, Expire: 86400, Minttl: 0},
		AllowWriteHTTPFrom: []*net.IPNet{
			parseCIDR(t, "127.0.0.0/8"),
			parseCIDR(t, "::1/128"),
		},
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678},
			&net.UnixAddr{Name: "/var/run/foo"},
		},
		Cache: cache.Options{
			EntryFetchMaxBurst: 42,
			EntryFetchRate:     0.334,
		},
		Cloud: hcpconfig.CloudConfig{
			ResourceID:   "cluster1",
			ClientID:     "id",
			ClientSecret: "secret",
		},
		ConsulCoordinateUpdatePeriod: 15 * time.Second,
		RaftProtocol:                 3,
		RetryJoinLAN: []string{
			"foo=bar key=baz secret=boom bang=bar",
		},
		RetryJoinWAN: []string{
			"wan_foo=bar wan_key=baz wan_secret=boom wan_bang=bar",
		},
		PrimaryGateways: []string{
			"pmgw_foo=bar pmgw_key=baz pmgw_secret=boom pmgw_bang=bar",
		},
		Services: []*structs.ServiceDefinition{
			{
				Name:  "foo",
				Token: "bar",
				Check: structs.CheckType{
					Name:          "blurb",
					OutputMaxSize: checks.DefaultBufSize,
				},
				Weights: &structs.Weights{
					Passing: 67,
					Warning: 3,
				},
			},
		},
		Checks: []*structs.CheckDefinition{
			{
				Name:          "zoo",
				Token:         "zope",
				OutputMaxSize: checks.DefaultBufSize,
			},
		},
		KVMaxValueSize: 1234567800000000,
		SerfAllowedCIDRsLAN: []net.IPNet{
			*parseCIDR(t, "192.168.1.0/24"),
			*parseCIDR(t, "127.0.0.0/8"),
		},
		TxnMaxReqLen: 5678000000000000,
		UIConfig: UIConfig{
			MetricsProxy: UIMetricsProxy{
				AddHeaders: []UIMetricsProxyAddHeader{
					{Name: "foo", Value: "secret"},
				},
			},
		},
		ServerRejoinAgeMax: 24 * 7 * time.Hour,
	}

	b, err := json.MarshalIndent(rt.Sanitized(), "", "    ")
	require.NoError(t, err)
	actual := string(b)
	require.JSONEq(t, golden(t, actual, testRuntimeConfigSanitizeExpectedFilename), actual)
}

func TestRuntime_apiAddresses(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("198.18.0.1"), Port: 5678},
			&net.UnixAddr{Name: "/var/run/foo"},
		},
		HTTPSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("198.18.0.2"), Port: 5678},
		}}

	unixAddrs, httpAddrs, httpsAddrs := rt.apiAddresses(1)

	require.Len(t, unixAddrs, 1)
	require.Len(t, httpAddrs, 1)
	require.Len(t, httpsAddrs, 1)

	require.Equal(t, "/var/run/foo", unixAddrs[0])
	require.Equal(t, "198.18.0.1:5678", httpAddrs[0])
	require.Equal(t, "198.18.0.2:5678", httpsAddrs[0])
}

func TestRuntime_APIConfigHTTPS(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("198.18.0.1"), Port: 5678},
			&net.UnixAddr{Name: "/var/run/foo"},
		},
		HTTPSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("198.18.0.2"), Port: 5678},
		},
		Datacenter: "dc-test",
		TLS: tlsutil.Config{
			HTTPS: tlsutil.ProtocolConfig{
				CAFile:         "/etc/consul/ca.crt",
				CAPath:         "/etc/consul/ca.dir",
				CertFile:       "/etc/consul/server.crt",
				KeyFile:        "/etc/consul/ssl/server.key",
				VerifyOutgoing: false,
			},
		},
	}

	cfg, err := rt.APIConfig(false)
	require.NoError(t, err)
	require.Equal(t, "198.18.0.2:5678", cfg.Address)
	require.Equal(t, "https", cfg.Scheme)
	require.Equal(t, rt.TLS.HTTPS.CAFile, cfg.TLSConfig.CAFile)
	require.Equal(t, rt.TLS.HTTPS.CAPath, cfg.TLSConfig.CAPath)
	require.Equal(t, "", cfg.TLSConfig.CertFile)
	require.Equal(t, "", cfg.TLSConfig.KeyFile)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, true, cfg.TLSConfig.InsecureSkipVerify)

	rt.TLS.HTTPS.VerifyOutgoing = true
	cfg, err = rt.APIConfig(true)
	require.NoError(t, err)
	require.Equal(t, "198.18.0.2:5678", cfg.Address)
	require.Equal(t, "https", cfg.Scheme)
	require.Equal(t, rt.TLS.HTTPS.CAFile, cfg.TLSConfig.CAFile)
	require.Equal(t, rt.TLS.HTTPS.CAPath, cfg.TLSConfig.CAPath)
	require.Equal(t, rt.TLS.HTTPS.CertFile, cfg.TLSConfig.CertFile)
	require.Equal(t, rt.TLS.HTTPS.KeyFile, cfg.TLSConfig.KeyFile)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, false, cfg.TLSConfig.InsecureSkipVerify)
}

func TestRuntime_APIConfigHTTP(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.UnixAddr{Name: "/var/run/foo"},
			&net.TCPAddr{IP: net.ParseIP("198.18.0.1"), Port: 5678},
		},
		Datacenter:          "dc-test",
		StaticRuntimeConfig: StaticRuntimeConfig{},
	}

	cfg, err := rt.APIConfig(false)
	require.NoError(t, err)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, "198.18.0.1:5678", cfg.Address)
	require.Equal(t, "http", cfg.Scheme)
	require.Equal(t, "", cfg.TLSConfig.CAFile)
	require.Equal(t, "", cfg.TLSConfig.CAPath)
	require.Equal(t, "", cfg.TLSConfig.CertFile)
	require.Equal(t, "", cfg.TLSConfig.KeyFile)
}

func TestRuntime_APIConfigUNIX(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.UnixAddr{Name: "/var/run/foo"},
		},
		Datacenter: "dc-test",
	}

	cfg, err := rt.APIConfig(false)
	require.NoError(t, err)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, "unix:///var/run/foo", cfg.Address)
	require.Equal(t, "http", cfg.Scheme)
	require.Equal(t, "", cfg.TLSConfig.CAFile)
	require.Equal(t, "", cfg.TLSConfig.CAPath)
	require.Equal(t, "", cfg.TLSConfig.CertFile)
	require.Equal(t, "", cfg.TLSConfig.KeyFile)
}

func TestRuntime_APIConfigANYAddrV4(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 5678},
		},
		Datacenter: "dc-test",
	}

	cfg, err := rt.APIConfig(false)
	require.NoError(t, err)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, "127.0.0.1:5678", cfg.Address)
	require.Equal(t, "http", cfg.Scheme)
	require.Equal(t, "", cfg.TLSConfig.CAFile)
	require.Equal(t, "", cfg.TLSConfig.CAPath)
	require.Equal(t, "", cfg.TLSConfig.CertFile)
	require.Equal(t, "", cfg.TLSConfig.KeyFile)
}

func TestRuntime_APIConfigANYAddrV6(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("::"), Port: 5678},
		},
		Datacenter: "dc-test",
	}

	cfg, err := rt.APIConfig(false)
	require.NoError(t, err)
	require.Equal(t, rt.Datacenter, cfg.Datacenter)
	require.Equal(t, "[::1]:5678", cfg.Address)
	require.Equal(t, "http", cfg.Scheme)
	require.Equal(t, "", cfg.TLSConfig.CAFile)
	require.Equal(t, "", cfg.TLSConfig.CAPath)
	require.Equal(t, "", cfg.TLSConfig.CertFile)
	require.Equal(t, "", cfg.TLSConfig.KeyFile)
}

func TestRuntime_ClientAddress(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("::"), Port: 5678},
			&net.TCPAddr{IP: net.ParseIP("198.18.0.1"), Port: 5679},
			&net.UnixAddr{Name: "/var/run/foo", Net: "unix"},
		},
		HTTPSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("::"), Port: 5688},
			&net.TCPAddr{IP: net.ParseIP("198.18.0.1"), Port: 5689},
		},
	}

	unix, http, https := rt.ClientAddress()

	require.Equal(t, "unix:///var/run/foo", unix)
	require.Equal(t, "198.18.0.1:5679", http)
	require.Equal(t, "198.18.0.1:5689", https)
}

func TestRuntime_ClientAddressAnyV4(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 5678},
			&net.UnixAddr{Name: "/var/run/foo", Net: "unix"},
		},
		HTTPSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 5688},
		},
	}

	unix, http, https := rt.ClientAddress()

	require.Equal(t, "unix:///var/run/foo", unix)
	require.Equal(t, "127.0.0.1:5678", http)
	require.Equal(t, "127.0.0.1:5688", https)
}

func TestRuntime_ClientAddressAnyV6(t *testing.T) {
	rt := RuntimeConfig{
		HTTPAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("::"), Port: 5678},
			&net.UnixAddr{Name: "/var/run/foo", Net: "unix"},
		},
		HTTPSAddrs: []net.Addr{
			&net.TCPAddr{IP: net.ParseIP("::"), Port: 5688},
		},
	}

	unix, http, https := rt.ClientAddress()

	require.Equal(t, "unix:///var/run/foo", unix)
	require.Equal(t, "[::1]:5678", http)
	require.Equal(t, "[::1]:5688", https)
}

func Test_UIPathBuilder(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		expected string
	}{
		{
			"Letters only string",
			"hello",
			"/hello/",
		},
		{
			"Alphanumeric",
			"Hello1",
			"/Hello1/",
		},
		{
			"Hyphen and underscore",
			"-_",
			"/-_/",
		},
		{
			"Many slashes",
			"/hello/ui/1/",
			"/hello/ui/1/",
		},
	}

	for _, tt := range cases {
		actual := UIPathBuilder(tt.path)
		require.Equal(t, tt.expected, actual)

	}
}

func splitIPPort(hostport string) (net.IP, int) {
	h, p, err := net.SplitHostPort(hostport)
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		panic(err)
	}
	return net.ParseIP(h), port
}

func ipAddr(addr string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(addr)}
}

func tcpAddr(addr string) *net.TCPAddr {
	ip, port := splitIPPort(addr)
	return &net.TCPAddr{IP: ip, Port: port}
}

func udpAddr(addr string) *net.UDPAddr {
	ip, port := splitIPPort(addr)
	return &net.UDPAddr{IP: ip, Port: port}
}

func unixAddr(addr string) *net.UnixAddr {
	if !strings.HasPrefix(addr, "unix://") {
		panic("not a unix socket addr: " + addr)
	}
	return &net.UnixAddr{Net: "unix", Name: addr[len("unix://"):]}
}

func writeFile(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, data, 0640); err != nil {
		panic(err)
	}
}

func randomString(n int) string {
	s := ""
	for ; n > 0; n-- {
		s += "x"
	}
	return s
}

func metaPairs(n int, format string) string {
	var s []string
	for i := 0; i < n; i++ {
		switch format {
		case "json":
			s = append(s, fmt.Sprintf(`"%d":"%d"`, i, i))
		case "hcl":
			s = append(s, fmt.Sprintf(`"%d"="%d"`, i, i))
		default:
			panic("invalid format: " + format)
		}
	}
	switch format {
	case "json":
		return strings.Join(s, ",")
	case "hcl":
		return strings.Join(s, " ")
	default:
		panic("invalid format: " + format)
	}
}

func TestConnectCAConfiguration(t *testing.T) {
	type testCase struct {
		config   RuntimeConfig
		expected *structs.CAConfiguration
		err      string
	}

	cases := map[string]testCase{
		"defaults": {
			config: RuntimeConfig{
				ConnectEnabled: true,
			},
			expected: &structs.CAConfiguration{
				Provider: "consul",
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "8760h",  // 365 * 24h
					"RootCertTTL":         "87600h", // 365 * 10 * 24h
				},
			},
		},
		"cluster-id-override": {
			config: RuntimeConfig{
				ConnectEnabled: true,
				ConnectCAConfig: map[string]interface{}{
					"cluster_id": "adfe7697-09b4-413a-ac0a-fa81ed3a3001",
				},
			},
			expected: &structs.CAConfiguration{
				Provider:  "consul",
				ClusterID: "adfe7697-09b4-413a-ac0a-fa81ed3a3001",
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "8760h",  // 365 * 24h
					"RootCertTTL":         "87600h", // 365 * 10 * 24h
					"cluster_id":          "adfe7697-09b4-413a-ac0a-fa81ed3a3001",
				},
			},
		},
		"cluster-id-non-uuid": {
			config: RuntimeConfig{
				ConnectEnabled: true,
				ConnectCAConfig: map[string]interface{}{
					"cluster_id": "foo",
				},
			},
			err: "cluster_id was supplied but was not a valid UUID",
		},
		"provider-override": {
			config: RuntimeConfig{
				ConnectEnabled:    true,
				ConnectCAProvider: "vault",
			},
			expected: &structs.CAConfiguration{
				Provider: "vault",
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "8760h",  // 365 * 24h
					"RootCertTTL":         "87600h", // 365 * 10 * 24h
				},
			},
		},
		"other-config": {
			config: RuntimeConfig{
				ConnectEnabled: true,
				ConnectCAConfig: map[string]interface{}{
					"foo":         "bar",
					"RootCertTTL": "8761h", // 365 * 24h + 1
				},
			},
			expected: &structs.CAConfiguration{
				Provider: "consul",
				Config: map[string]interface{}{
					"LeafCertTTL":         "72h",
					"IntermediateCertTTL": "8760h", // 365 * 24h
					"RootCertTTL":         "8761h", // 365 * 24h + 1
					"foo":                 "bar",
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := tcase.config.ConnectCAConfiguration()
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tcase.expected, actual)
			}
		})
	}
}
