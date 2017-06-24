package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/raft"
	"github.com/pascaldekloe/goe/verify"
)

func init() {
	version.Version = "0.8.0"
}

func externalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("Unable to lookup network interfaces: %v", err)
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("Unable to find a non-loopback interface")
}

func TestAgent_MultiStartStop(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := NewTestAgent(t.Name(), nil)
			time.Sleep(250 * time.Millisecond)
			a.Shutdown()
		})
	}
}

func TestAgent_StartStop(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	// defer a.Shutdown()

	if err := a.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := a.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-a.ShutdownCh():
	default:
		t.Fatalf("should be closed")
	}
}

func TestAgent_RPCPing(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	var out struct{}
	if err := a.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_CheckSerfBindAddrsSettings(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "darwin" {
		t.Skip("skip test on macOS to avoid firewall warning dialog")
	}

	cfg := TestConfig()
	ip, err := externalIP()
	if err != nil {
		t.Fatalf("Unable to get a non-loopback IP: %v", err)
	}
	cfg.SerfLanBindAddr = ip
	cfg.SerfWanBindAddr = ip
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	serfWanBind := a.consulConfig().SerfWANConfig.MemberlistConfig.BindAddr
	if serfWanBind != ip {
		t.Fatalf("SerfWanBindAddr is should be a non-loopback IP not %s", serfWanBind)
	}

	serfLanBind := a.consulConfig().SerfLANConfig.MemberlistConfig.BindAddr
	if serfLanBind != ip {
		t.Fatalf("SerfLanBindAddr is should be a non-loopback IP not %s", serfWanBind)
	}
}
func TestAgent_CheckAdvertiseAddrsSettings(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.AdvertiseAddrs.SerfLan, _ = net.ResolveTCPAddr("tcp", "127.0.0.42:1233")
	cfg.AdvertiseAddrs.SerfWan, _ = net.ResolveTCPAddr("tcp", "127.0.0.43:1234")
	cfg.AdvertiseAddrs.RPC, _ = net.ResolveTCPAddr("tcp", "127.0.0.44:1235")
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	serfLanAddr := a.consulConfig().SerfLANConfig.MemberlistConfig.AdvertiseAddr
	if serfLanAddr != "127.0.0.42" {
		t.Fatalf("SerfLan is not properly set to '127.0.0.42': %s", serfLanAddr)
	}
	serfLanPort := a.consulConfig().SerfLANConfig.MemberlistConfig.AdvertisePort
	if serfLanPort != 1233 {
		t.Fatalf("SerfLan is not properly set to '1233': %d", serfLanPort)
	}
	serfWanAddr := a.consulConfig().SerfWANConfig.MemberlistConfig.AdvertiseAddr
	if serfWanAddr != "127.0.0.43" {
		t.Fatalf("SerfWan is not properly set to '127.0.0.43': %s", serfWanAddr)
	}
	serfWanPort := a.consulConfig().SerfWANConfig.MemberlistConfig.AdvertisePort
	if serfWanPort != 1234 {
		t.Fatalf("SerfWan is not properly set to '1234': %d", serfWanPort)
	}
	rpc := a.consulConfig().RPCAdvertise
	if rpc != cfg.AdvertiseAddrs.RPC {
		t.Fatalf("RPC is not properly set to %v: %s", cfg.AdvertiseAddrs.RPC, rpc)
	}
	expected := map[string]string{
		"lan": a.Config.AdvertiseAddr,
		"wan": a.Config.AdvertiseAddrWan,
	}
	if !reflect.DeepEqual(a.Config.TaggedAddresses, expected) {
		t.Fatalf("Tagged addresses not set up properly: %v", a.Config.TaggedAddresses)
	}
}

func TestAgent_CheckPerformanceSettings(t *testing.T) {
	t.Parallel()
	// Try a default config.
	{
		cfg := TestConfig()
		cfg.Bootstrap = false
		cfg.ConsulConfig = nil
		a := NewTestAgent(t.Name(), cfg)
		defer a.Shutdown()

		raftMult := time.Duration(consul.DefaultRaftMultiplier)
		r := a.consulConfig().RaftConfig
		def := raft.DefaultConfig()
		if r.HeartbeatTimeout != raftMult*def.HeartbeatTimeout ||
			r.ElectionTimeout != raftMult*def.ElectionTimeout ||
			r.LeaderLeaseTimeout != raftMult*def.LeaderLeaseTimeout {
			t.Fatalf("bad: %#v", *r)
		}
	}

	// Try a multiplier.
	{
		cfg := TestConfig()
		cfg.Bootstrap = false
		cfg.Performance.RaftMultiplier = 99
		a := NewTestAgent(t.Name(), cfg)
		defer a.Shutdown()

		const raftMult time.Duration = 99
		r := a.consulConfig().RaftConfig
		def := raft.DefaultConfig()
		if r.HeartbeatTimeout != raftMult*def.HeartbeatTimeout ||
			r.ElectionTimeout != raftMult*def.ElectionTimeout ||
			r.LeaderLeaseTimeout != raftMult*def.LeaderLeaseTimeout {
			t.Fatalf("bad: %#v", *r)
		}
	}
}

func TestAgent_ReconnectConfigSettings(t *testing.T) {
	t.Parallel()
	func() {
		a := NewTestAgent(t.Name(), nil)
		defer a.Shutdown()

		lan := a.consulConfig().SerfLANConfig.ReconnectTimeout
		if lan != 3*24*time.Hour {
			t.Fatalf("bad: %s", lan.String())
		}

		wan := a.consulConfig().SerfWANConfig.ReconnectTimeout
		if wan != 3*24*time.Hour {
			t.Fatalf("bad: %s", wan.String())
		}
	}()

	func() {
		cfg := TestConfig()
		cfg.ReconnectTimeoutLan = 24 * time.Hour
		cfg.ReconnectTimeoutWan = 36 * time.Hour
		a := NewTestAgent(t.Name(), cfg)
		defer a.Shutdown()

		lan := a.consulConfig().SerfLANConfig.ReconnectTimeout
		if lan != 24*time.Hour {
			t.Fatalf("bad: %s", lan.String())
		}

		wan := a.consulConfig().SerfWANConfig.ReconnectTimeout
		if wan != 36*time.Hour {
			t.Fatalf("bad: %s", wan.String())
		}
	}()
}

func TestAgent_setupNodeID(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.NodeID = ""
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// The auto-assigned ID should be valid.
	id := a.consulConfig().NodeID
	if _, err := uuid.ParseUUID(string(id)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Running again should get the same ID (persisted in the file).
	cfg.NodeID = ""
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if newID := a.consulConfig().NodeID; id != newID {
		t.Fatalf("bad: %q vs %q", id, newID)
	}

	// Set an invalid ID via.Config.
	cfg.NodeID = types.NodeID("nope")
	err := a.setupNodeID(cfg)
	if err == nil || !strings.Contains(err.Error(), "uuid string is wrong length") {
		t.Fatalf("err: %v", err)
	}

	// Set a valid ID via.Config.
	newID, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = types.NodeID(strings.ToUpper(newID))
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if id := a.consulConfig().NodeID; string(id) != newID {
		t.Fatalf("bad: %q vs. %q", id, newID)
	}

	// Set an invalid ID via the file.
	fileID := filepath.Join(cfg.DataDir, "node-id")
	if err := ioutil.WriteFile(fileID, []byte("adf4238a!882b!9ddc!4a9d!5b6758e4159e"), 0600); err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = ""
	err = a.setupNodeID(cfg)
	if err == nil || !strings.Contains(err.Error(), "uuid is improperly formatted") {
		t.Fatalf("err: %v", err)
	}

	// Set a valid ID via the file.
	if err := ioutil.WriteFile(fileID, []byte("ADF4238a-882b-9ddc-4a9d-5b6758e4159e"), 0600); err != nil {
		t.Fatalf("err: %v", err)
	}
	cfg.NodeID = ""
	if err := a.setupNodeID(cfg); err != nil {
		t.Fatalf("err: %v", err)
	}
	if id := a.consulConfig().NodeID; string(id) != "adf4238a-882b-9ddc-4a9d-5b6758e4159e" {
		t.Fatalf("bad: %q vs. %q", id, newID)
	}
}

func TestAgent_makeNodeID(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.NodeID = ""
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// We should get a valid host-based ID initially.
	id, err := a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := uuid.ParseUUID(string(id)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Calling again should yield a random ID by default.
	another, err := a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == another {
		t.Fatalf("bad: %s vs %s", id, another)
	}

	// Turn on host-based IDs and try again. We should get the same ID
	// each time (and a different one from the random one above).
	a.Config.DisableHostNodeID = Bool(false)
	id, err = a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id == another {
		t.Fatalf("bad: %s vs %s", id, another)
	}

	// Calling again should yield the host-based ID.
	another, err = a.makeNodeID()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if id != another {
		t.Fatalf("bad: %s vs %s", id, another)
	}
}

func TestAgent_AddService(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.NodeName = "node1"
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	tests := []struct {
		desc       string
		srv        *structs.NodeService
		chkTypes   []*structs.CheckType
		healthChks map[string]*structs.HealthCheck
	}{
		{
			"one check",
			&structs.NodeService{
				ID:      "svcid1",
				Service: "svcname1",
				Tags:    []string{"tag1"},
				Port:    8100,
			},
			[]*structs.CheckType{
				&structs.CheckType{
					CheckID: "check1",
					Name:    "name1",
					TTL:     time.Minute,
					Notes:   "note1",
				},
			},
			map[string]*structs.HealthCheck{
				"check1": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check1",
					Name:        "name1",
					Status:      "critical",
					Notes:       "note1",
					ServiceID:   "svcid1",
					ServiceName: "svcname1",
				},
			},
		},
		{
			"multiple checks",
			&structs.NodeService{
				ID:      "svcid2",
				Service: "svcname2",
				Tags:    []string{"tag2"},
				Port:    8200,
			},
			[]*structs.CheckType{
				&structs.CheckType{
					CheckID: "check1",
					Name:    "name1",
					TTL:     time.Minute,
					Notes:   "note1",
				},
				&structs.CheckType{
					CheckID: "check-noname",
					TTL:     time.Minute,
				},
				&structs.CheckType{
					Name: "check-noid",
					TTL:  time.Minute,
				},
				&structs.CheckType{
					TTL: time.Minute,
				},
			},
			map[string]*structs.HealthCheck{
				"check1": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check1",
					Name:        "name1",
					Status:      "critical",
					Notes:       "note1",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
				},
				"check-noname": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check-noname",
					Name:        "Service 'svcname2' check",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
				},
				"service:svcid2:3": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "service:svcid2:3",
					Name:        "check-noid",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
				},
				"service:svcid2:4": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "service:svcid2:4",
					Name:        "Service 'svcname2' check",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// check the service registration
			t.Run(tt.srv.ID, func(t *testing.T) {
				err := a.AddService(tt.srv, tt.chkTypes, false, "")
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				got, want := a.state.Services()[tt.srv.ID], tt.srv
				verify.Values(t, "", got, want)
			})

			// check the health checks
			for k, v := range tt.healthChks {
				t.Run(k, func(t *testing.T) {
					got, want := a.state.Checks()[types.CheckID(k)], v
					verify.Values(t, k, got, want)
				})
			}

			// check the ttl checks
			for k := range tt.healthChks {
				t.Run(k+" ttl", func(t *testing.T) {
					chk := a.checkTTLs[types.CheckID(k)]
					if chk == nil {
						t.Fatal("got nil want TTL check")
					}
					if got, want := string(chk.CheckID), k; got != want {
						t.Fatalf("got CheckID %v want %v", got, want)
					}
					if got, want := chk.TTL, time.Minute; got != want {
						t.Fatalf("got TTL %v want %v", got, want)
					}
				})
			}
		})
	}
}

func TestAgent_RemoveService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Remove a service that doesn't exist
	if err := a.RemoveService("redis", false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove the consul service
	if err := a.RemoveService("consul", false); err == nil {
		t.Fatalf("should have errored")
	}

	// Remove without an ID
	if err := a.RemoveService("", false); err == nil {
		t.Fatalf("should have errored")
	}

	// Removing a service with a single check works
	{
		srv := &structs.NodeService{
			ID:      "memcache",
			Service: "memcache",
			Port:    8000,
		}
		chkTypes := []*structs.CheckType{&structs.CheckType{TTL: time.Minute}}

		if err := a.AddService(srv, chkTypes, false, ""); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Add a check after the fact with a specific check ID
		check := &structs.CheckDefinition{
			ID:        "check2",
			Name:      "check2",
			ServiceID: "memcache",
			TTL:       time.Minute,
		}
		hc := check.HealthCheck("node1")
		if err := a.AddCheck(hc, check.CheckType(), false, ""); err != nil {
			t.Fatalf("err: %s", err)
		}

		if err := a.RemoveService("memcache", false); err != nil {
			t.Fatalf("err: %s", err)
		}
		if _, ok := a.state.Checks()["service:memcache"]; ok {
			t.Fatalf("have memcache check")
		}
		if _, ok := a.state.Checks()["check2"]; ok {
			t.Fatalf("have check2 check")
		}
	}

	// Removing a service with multiple checks works
	{
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Port:    8000,
		}
		chkTypes := []*structs.CheckType{
			&structs.CheckType{TTL: time.Minute},
			&structs.CheckType{TTL: 30 * time.Second},
		}
		if err := a.AddService(srv, chkTypes, false, ""); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Remove the service
		if err := a.RemoveService("redis", false); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Ensure we have a state mapping
		if _, ok := a.state.Services()["redis"]; ok {
			t.Fatalf("have redis service")
		}

		// Ensure checks were removed
		if _, ok := a.state.Checks()["service:redis:1"]; ok {
			t.Fatalf("check redis:1 should be removed")
		}
		if _, ok := a.state.Checks()["service:redis:2"]; ok {
			t.Fatalf("check redis:2 should be removed")
		}

		// Ensure a TTL is setup
		if _, ok := a.checkTTLs["service:redis:1"]; ok {
			t.Fatalf("check ttl for redis:1 should be removed")
		}
		if _, ok := a.checkTTLs["service:redis:2"]; ok {
			t.Fatalf("check ttl for redis:2 should be removed")
		}
	}
}

func TestAgent_RemoveServiceRemovesAllChecks(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.NodeName = "node1"
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	svc := &structs.NodeService{ID: "redis", Service: "redis", Port: 8000}
	chk1 := &structs.CheckType{CheckID: "chk1", Name: "chk1", TTL: time.Minute}
	chk2 := &structs.CheckType{CheckID: "chk2", Name: "chk2", TTL: 2 * time.Minute}
	hchk1 := &structs.HealthCheck{Node: "node1", CheckID: "chk1", Name: "chk1", Status: "critical", ServiceID: "redis", ServiceName: "redis"}
	hchk2 := &structs.HealthCheck{Node: "node1", CheckID: "chk2", Name: "chk2", Status: "critical", ServiceID: "redis", ServiceName: "redis"}

	// register service with chk1
	if err := a.AddService(svc, []*structs.CheckType{chk1}, false, ""); err != nil {
		t.Fatal("Failed to register service", err)
	}

	// verify chk1 exists
	if a.state.Checks()["chk1"] == nil {
		t.Fatal("Could not find health check chk1")
	}

	// update the service with chk2
	if err := a.AddService(svc, []*structs.CheckType{chk2}, false, ""); err != nil {
		t.Fatal("Failed to update service", err)
	}

	// check that both checks are there
	if got, want := a.state.Checks()["chk1"], hchk1; !verify.Values(t, "", got, want) {
		t.FailNow()
	}
	if got, want := a.state.Checks()["chk2"], hchk2; !verify.Values(t, "", got, want) {
		t.FailNow()
	}

	// Remove service
	if err := a.RemoveService("redis", false); err != nil {
		t.Fatal("Failed to remove service", err)
	}

	// Check that both checks are gone
	if a.state.Checks()["chk1"] != nil {
		t.Fatal("Found health check chk1 want nil")
	}
	if a.state.Checks()["chk2"] != nil {
		t.Fatal("Found health check chk2 want nil")
	}
}

func TestAgent_AddCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		Script:   "exit 0",
		Interval: 15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk, ok := a.state.Checks()["mem"]
	if !ok {
		t.Fatalf("missing mem check")
	}

	// Ensure our check is in the right state
	if sChk.Status != api.HealthCritical {
		t.Fatalf("check not critical")
	}

	// Ensure a TTL is setup
	if _, ok := a.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	}
}

func TestAgent_AddCheck_StartPassing(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthPassing,
	}
	chk := &structs.CheckType{
		Script:   "exit 0",
		Interval: 15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk, ok := a.state.Checks()["mem"]
	if !ok {
		t.Fatalf("missing mem check")
	}

	// Ensure our check is in the right state
	if sChk.Status != api.HealthPassing {
		t.Fatalf("check not passing")
	}

	// Ensure a TTL is setup
	if _, ok := a.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	}
}

func TestAgent_AddCheck_MinInterval(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		Script:   "exit 0",
		Interval: time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	if _, ok := a.state.Checks()["mem"]; !ok {
		t.Fatalf("missing mem check")
	}

	// Ensure a TTL is setup
	if mon, ok := a.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	} else if mon.Interval != MinInterval {
		t.Fatalf("bad mem monitor interval")
	}
}

func TestAgent_AddCheck_MissingService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "baz",
		Name:      "baz check 1",
		ServiceID: "baz",
	}
	chk := &structs.CheckType{
		Script:   "exit 0",
		Interval: time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "")
	if err == nil || err.Error() != `ServiceID "baz" does not exist` {
		t.Fatalf("expected service id error, got: %v", err)
	}
}

func TestAgent_AddCheck_RestoreState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Create some state and persist it
	ttl := &CheckTTL{
		CheckID: "baz",
		TTL:     time.Minute,
	}
	err := a.persistCheckState(ttl, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Build and register the check definition and initial state
	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "baz",
		Name:    "baz check 1",
	}
	chk := &structs.CheckType{
		TTL: time.Minute,
	}
	err = a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check status was restored during registration
	checks := a.state.Checks()
	check, ok := checks["baz"]
	if !ok {
		t.Fatalf("missing check")
	}
	if check.Status != api.HealthPassing {
		t.Fatalf("bad: %#v", check)
	}
	if check.Output != "yup" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_RemoveCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Remove check that doesn't exist
	if err := a.RemoveCheck("mem", false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove without an ID
	if err := a.RemoveCheck("", false); err == nil {
		t.Fatalf("should have errored")
	}

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		Script:   "exit 0",
		Interval: 15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove check
	if err := a.RemoveCheck("mem", false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	if _, ok := a.state.Checks()["mem"]; ok {
		t.Fatalf("have mem check")
	}

	// Ensure a TTL is setup
	if _, ok := a.checkMonitors["mem"]; ok {
		t.Fatalf("have mem monitor")
	}
}

func TestAgent_updateTTLCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		TTL: 15 * time.Second,
	}

	// Add check and update it.
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := a.updateTTLCheck("mem", api.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping.
	status := a.state.Checks()["mem"]
	if status.Status != api.HealthPassing {
		t.Fatalf("bad: %v", status)
	}
	if status.Output != "foo" {
		t.Fatalf("bad: %v", status)
	}
}

func TestAgent_ConsulService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Consul service is registered
	services := a.state.Services()
	if _, ok := services[consul.ConsulServiceID]; !ok {
		t.Fatalf("%s service should be registered", consul.ConsulServiceID)
	}

	// Perform anti-entropy on consul service
	if err := a.state.syncService(consul.ConsulServiceID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Consul service should be in sync
	if !a.state.serviceStatus[consul.ConsulServiceID].inSync {
		t.Fatalf("%s service should be in sync", consul.ConsulServiceID)
	}
}

func TestAgent_PersistService(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Server = false
	cfg.DataDir = testutil.TempDir(t, "agent") // we manage the data dir
	a := NewTestAgent(t.Name(), cfg)
	defer os.RemoveAll(cfg.DataDir)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc.ID))

	// Check is not persisted unless requested
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Persists to file if requested
	if err := a.AddService(svc, nil, true, "mytoken"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}
	expected, err := json.Marshal(persistedService{
		Token:   "mytoken",
		Service: svc,
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}

	// Updates service definition on disk
	svc.Port = 8001
	if err := a.AddService(svc, nil, true, "mytoken"); err != nil {
		t.Fatalf("err: %v", err)
	}
	expected, err = json.Marshal(persistedService{
		Token:   "mytoken",
		Service: svc,
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err = ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}
	a.Shutdown()

	// Should load it back during later start
	a2 := NewTestAgent(t.Name()+"-a2", cfg)
	defer a2.Shutdown()

	restored, ok := a2.state.services[svc.ID]
	if !ok {
		t.Fatalf("bad: %#v", a2.state.services)
	}
	if a2.state.serviceTokens[svc.ID] != "mytoken" {
		t.Fatalf("bad: %#v", a2.state.services[svc.ID])
	}
	if restored.Port != 8001 {
		t.Fatalf("bad: %#v", restored)
	}
}

func TestAgent_persistedService_compat(t *testing.T) {
	t.Parallel()
	// Tests backwards compatibility of persisted services from pre-0.5.1
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Encode the NodeService directly. This is what previous versions
	// would serialize to the file (without the wrapper)
	encoded, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Write the content to the file
	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc.ID))
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := ioutil.WriteFile(file, encoded, 0600); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Load the services
	if err := a.loadServices(a.Config); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the service was restored
	services := a.state.Services()
	result, ok := services["redis"]
	if !ok {
		t.Fatalf("missing service")
	}
	if !reflect.DeepEqual(result, svc) {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_PurgeService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc.ID))
	if err := a.AddService(svc, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Not removed
	if err := a.RemoveService(svc.ID, false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-add the service
	if err := a.AddService(svc, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Removed
	if err := a.RemoveService(svc.ID, true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeServiceOnDuplicate(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Server = false
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// First persist the service
	if err := a.AddService(svc1, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	a.Shutdown()

	// Try bringing the agent back up with the service already
	// existing in the config
	svc2 := &structs.ServiceDefinition{
		ID:   "redis",
		Name: "redis",
		Tags: []string{"bar"},
		Port: 9000,
	}

	cfg.Services = []*structs.ServiceDefinition{svc2}
	a2 := NewTestAgent(t.Name()+"-a2", cfg)
	defer a2.Shutdown()

	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc1.ID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted service")
	}
	result, ok := a2.state.services[svc2.ID]
	if !ok {
		t.Fatalf("missing service registration")
	}
	if !reflect.DeepEqual(result.Tags, svc2.Tags) || result.Port != svc2.Port {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_PersistCheck(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Server = false
	cfg.DataDir = testutil.TempDir(t, "agent") // we manage the data dir
	a := NewTestAgent(t.Name(), cfg)
	defer os.RemoveAll(cfg.DataDir)
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    cfg.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}
	chkType := &structs.CheckType{
		Script:   "/bin/true",
		Interval: 10 * time.Second,
	}

	file := filepath.Join(a.Config.DataDir, checksDir, checkIDHash(check.CheckID))

	// Not persisted if not requested
	if err := a.AddCheck(check, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Should persist if requested
	if err := a.AddCheck(check, chkType, true, "mytoken"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}
	expected, err := json.Marshal(persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   "mytoken",
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}

	// Updates the check definition on disk
	check.Name = "mem1"
	if err := a.AddCheck(check, chkType, true, "mytoken"); err != nil {
		t.Fatalf("err: %v", err)
	}
	expected, err = json.Marshal(persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   "mytoken",
	})
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err = ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}
	a.Shutdown()

	// Should load it back during later start
	a2 := NewTestAgent(t.Name()+"-a2", cfg)
	defer a2.Shutdown()

	result, ok := a2.state.checks[check.CheckID]
	if !ok {
		t.Fatalf("bad: %#v", a2.state.checks)
	}
	if result.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", result)
	}
	if result.Name != "mem1" {
		t.Fatalf("bad: %#v", result)
	}

	// Should have restored the monitor
	if _, ok := a2.checkMonitors[check.CheckID]; !ok {
		t.Fatalf("bad: %#v", a2.checkMonitors)
	}
	if a2.state.checkTokens[check.CheckID] != "mytoken" {
		t.Fatalf("bad: %s", a2.state.checkTokens[check.CheckID])
	}
}

func TestAgent_PurgeCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}

	file := filepath.Join(a.Config.DataDir, checksDir, checkIDHash(check.CheckID))
	if err := a.AddCheck(check, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Not removed
	if err := a.RemoveCheck(check.CheckID, false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed
	if err := a.RemoveCheck(check.CheckID, true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeCheckOnDuplicate(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Server = false
	cfg.DataDir = testutil.TempDir(t, "agent") // we manage the data dir
	a := NewTestAgent(t.Name(), cfg)
	defer os.RemoveAll(cfg.DataDir)
	defer a.Shutdown()

	check1 := &structs.HealthCheck{
		Node:    cfg.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}

	// First persist the check
	if err := a.AddCheck(check1, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	a.Shutdown()

	// Start again with the check registered in config
	check2 := &structs.CheckDefinition{
		ID:       "mem",
		Name:     "memory check",
		Notes:    "my cool notes",
		Script:   "/bin/check-redis.py",
		Interval: 30 * time.Second,
	}

	cfg.Checks = []*structs.CheckDefinition{check2}
	a2 := NewTestAgent(t.Name()+"-a2", cfg)
	defer a2.Shutdown()

	file := filepath.Join(a.Config.DataDir, checksDir, checkIDHash(check1.CheckID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted check")
	}
	result, ok := a2.state.checks[check2.ID]
	if !ok {
		t.Fatalf("missing check registration")
	}
	expected := check2.HealthCheck(cfg.NodeName)
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_loadChecks_token(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Checks = append(cfg.Checks, &structs.CheckDefinition{
		ID:    "rabbitmq",
		Name:  "rabbitmq",
		Token: "abc123",
		TTL:   10 * time.Second,
	})
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	checks := a.state.Checks()
	if _, ok := checks["rabbitmq"]; !ok {
		t.Fatalf("missing check")
	}
	if token := a.state.CheckToken("rabbitmq"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_unloadChecks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, false, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for check := range a.state.Checks() {
		if check == check1.CheckID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("check should have been registered")
	}

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure it was unloaded
	for check := range a.state.Checks() {
		if check == check1.CheckID {
			t.Fatalf("should have unloaded checks")
		}
	}
}

func TestAgent_loadServices_token(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.Services = append(cfg.Services, &structs.ServiceDefinition{
		ID:    "rabbitmq",
		Name:  "rabbitmq",
		Port:  5672,
		Token: "abc123",
	})
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	services := a.state.Services()
	if _, ok := services["rabbitmq"]; !ok {
		t.Fatalf("missing service")
	}
	if token := a.state.ServiceToken("rabbitmq"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_unloadServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for id := range a.state.Services() {
		if id == svc.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("should have registered service")
	}

	// Unload all services
	if err := a.unloadServices(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure it was unloaded and the consul service remains
	found = false
	for id := range a.state.Services() {
		if id == svc.ID {
			t.Fatalf("should have unloaded services")
		}
		if id == consul.ConsulServiceID {
			found = true
		}
	}
	if !found {
		t.Fatalf("consul service should not be removed")
	}
}

func TestAgent_Service_MaintenanceMode(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Enter maintenance mode for the service
	if err := a.EnableServiceMaintenance("redis", "broken", "mytoken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the critical health check was added
	checkID := serviceMaintCheckID("redis")
	check, ok := a.state.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical maintenance check")
	}

	// Check that the token was used to register the check
	if token := a.state.CheckToken(checkID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got: '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	if err := a.DisableServiceMaintenance("redis"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was deregistered
	if _, ok := a.state.Checks()[checkID]; ok {
		t.Fatalf("should have deregistered maintenance check")
	}

	// Enter service maintenance mode without providing a reason
	if err := a.EnableServiceMaintenance("redis", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was registered with the default notes
	check, ok = a.state.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical check")
	}
	if check.Notes != defaultServiceMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_Service_Reap(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.CheckReapInterval = time.Millisecond
	cfg.CheckDeregisterIntervalMin = 0
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chkTypes := []*structs.CheckType{
		&structs.CheckType{
			Status: api.HealthPassing,
			TTL:    10 * time.Millisecond,
			DeregisterCriticalServiceAfter: 100 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.AddService(svc, chkTypes, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail.
	time.Sleep(30 * time.Millisecond)
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Pass the TTL.
	if err := a.updateTTLCheck("service:redis", api.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail again.
	time.Sleep(30 * time.Millisecond)
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Wait for the reap.
	time.Sleep(300 * time.Millisecond)
	if _, ok := a.state.Services()["redis"]; ok {
		t.Fatalf("redis service should have been reaped")
	}
	if checks := a.state.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}
}

func TestAgent_Service_NoReap(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.CheckReapInterval = time.Millisecond
	cfg.CheckDeregisterIntervalMin = 0
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chkTypes := []*structs.CheckType{
		&structs.CheckType{
			Status: api.HealthPassing,
			TTL:    10 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.AddService(svc, chkTypes, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail.
	time.Sleep(30 * time.Millisecond)
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Wait a while and make sure it doesn't reap.
	time.Sleep(300 * time.Millisecond)
	if _, ok := a.state.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.state.CriticalChecks(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}
}

func TestAgent_addCheck_restoresSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, false, ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-registering the service preserves the state of the check
	chkTypes := []*structs.CheckType{&structs.CheckType{TTL: 30 * time.Second}}
	if err := a.AddService(svc, chkTypes, false, ""); err != nil {
		t.Fatalf("err: %s", err)
	}
	check, ok := a.state.Checks()["service:redis"]
	if !ok {
		t.Fatalf("missing check")
	}
	if check.Status != api.HealthPassing {
		t.Fatalf("bad: %s", check.Status)
	}
}

func TestAgent_NodeMaintenanceMode(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Enter maintenance mode for the node
	a.EnableNodeMaintenance("broken", "mytoken")

	// Make sure the critical health check was added
	check, ok := a.state.Checks()[structs.NodeMaint]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}

	// Check that the token was used to register the check
	if token := a.state.CheckToken(structs.NodeMaint); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got: '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	a.DisableNodeMaintenance()

	// Ensure the check was deregistered
	if _, ok := a.state.Checks()[structs.NodeMaint]; ok {
		t.Fatalf("should have deregistered critical node check")
	}

	// Enter maintenance mode without passing a reason
	a.EnableNodeMaintenance("", "")

	// Make sure the check was registered with the default note
	check, ok = a.state.Checks()[structs.NodeMaint]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}
	if check.Notes != defaultNodeMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_checkStateSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      api.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := a.AddCheck(check1, nil, true, ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Snapshot the state
	snap := a.snapshotCheckState()

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Reload the checks
	if err := a.loadChecks(a.Config); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Restore the state
	a.restoreCheckState(snap)

	// Search for the check
	out, ok := a.state.Checks()[check1.CheckID]
	if !ok {
		t.Fatalf("check should have been registered")
	}

	// Make sure state was restored
	if out.Status != api.HealthPassing {
		t.Fatalf("should have restored check state")
	}
}

func TestAgent_loadChecks_checkFails(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Persist a health check with an invalid service ID
	check := &structs.HealthCheck{
		Node:      a.Config.NodeName,
		CheckID:   "service:redis",
		Name:      "redischeck",
		Status:    api.HealthPassing,
		ServiceID: "nope",
	}
	if err := a.persistCheck(check, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check to make sure the check was persisted
	checkHash := checkIDHash(check.CheckID)
	checkPath := filepath.Join(a.Config.DataDir, checksDir, checkHash)
	if _, err := os.Stat(checkPath); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try loading the checks from the persisted files
	if err := a.loadChecks(a.Config); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the erroneous check was purged
	if _, err := os.Stat(checkPath); err == nil {
		t.Fatalf("should have purged check")
	}
}

func TestAgent_persistCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Create the TTL check to persist
	check := &CheckTTL{
		CheckID: "check1",
		TTL:     10 * time.Minute,
	}

	// Persist some check state for the check
	err := a.persistCheckState(check, api.HealthCritical, "nope")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the persisted file exists and has the content
	file := filepath.Join(a.Config.DataDir, checkStateDir, stringHash("check1"))
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Decode the state
	var p persistedCheckState
	if err := json.Unmarshal(buf, &p); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check the fields
	if p.CheckID != "check1" {
		t.Fatalf("bad: %#v", p)
	}
	if p.Output != "nope" {
		t.Fatalf("bad: %#v", p)
	}
	if p.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", p)
	}

	// Check the expiration time was set
	if p.Expires < time.Now().Unix() {
		t.Fatalf("bad: %#v", p)
	}
}

func TestAgent_loadCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Create a check whose state will expire immediately
	check := &CheckTTL{
		CheckID: "check1",
		TTL:     0,
	}

	// Persist the check state
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load the state
	health := &structs.HealthCheck{
		CheckID: "check1",
		Status:  api.HealthCritical,
	}
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should not have restored the status due to expiration
	if health.Status != api.HealthCritical {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "" {
		t.Fatalf("bad: %#v", health)
	}

	// Should have purged the state
	file := filepath.Join(a.Config.DataDir, checksDir, stringHash("check1"))
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have purged state")
	}

	// Set a TTL which will not expire before we check it
	check.TTL = time.Minute
	err = a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Try to load
	if err := a.loadCheckState(health); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should have restored
	if health.Status != api.HealthPassing {
		t.Fatalf("bad: %#v", health)
	}
	if health.Output != "yup" {
		t.Fatalf("bad: %#v", health)
	}
}

func TestAgent_purgeCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// No error if the state does not exist
	if err := a.purgeCheckState("check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Persist some state to the data dir
	check := &CheckTTL{
		CheckID: "check1",
		TTL:     time.Minute,
	}
	err := a.persistCheckState(check, api.HealthPassing, "yup")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Purge the check state
	if err := a.purgeCheckState("check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed the file
	file := filepath.Join(a.Config.DataDir, checkStateDir, stringHash("check1"))
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("should have removed file")
	}
}

func TestAgent_GetCoordinate(t *testing.T) {
	t.Parallel()
	check := func(server bool) {
		cfg := TestConfig()
		cfg.Server = server
		a := NewTestAgent(t.Name(), cfg)
		defer a.Shutdown()

		// This doesn't verify the returned coordinate, but it makes
		// sure that the agent chooses the correct Serf instance,
		// depending on how it's configured as a client or a server.
		// If it chooses the wrong one, this will crash.
		if _, err := a.GetLANCoordinate(); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	check(true)
	check(false)
}
