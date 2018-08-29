package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	for i := 0; i < 10; i++ {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := NewTestAgent(t.Name(), "")
			time.Sleep(250 * time.Millisecond)
			a.Shutdown()
		})
	}
}

func TestAgent_ConnectClusterIDConfig(t *testing.T) {
	tests := []struct {
		name          string
		hcl           string
		wantClusterID string
		wantPanic     bool
	}{
		{
			name:          "default TestAgent has fixed cluster id",
			hcl:           "",
			wantClusterID: connect.TestClusterID,
		},
		{
			name:          "no cluster ID specified sets to test ID",
			hcl:           "connect { enabled = true }",
			wantClusterID: connect.TestClusterID,
		},
		{
			name: "non-UUID cluster_id is fatal",
			hcl: `connect {
	   enabled = true
	   ca_config {
	     cluster_id = "fake-id"
	   }
	 }`,
			wantClusterID: "",
			wantPanic:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Indirection to support panic recovery cleanly
			testFn := func() {
				a := &TestAgent{Name: "test", HCL: tt.hcl}
				a.ExpectConfigError = tt.wantPanic
				a.Start()
				defer a.Shutdown()

				cfg := a.consulConfig()
				assert.Equal(t, tt.wantClusterID, cfg.CAConfig.ClusterID)
			}

			if tt.wantPanic {
				require.Panics(t, testFn)
			} else {
				testFn()
			}
		})
	}
}

func TestAgent_StartStop(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	var out struct{}
	if err := a.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_TokenStore(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), `
		acl_token = "user"
		acl_agent_token = "agent"
		acl_agent_master_token = "master"`,
	)
	defer a.Shutdown()

	if got, want := a.tokens.UserToken(), "user"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := a.tokens.AgentToken(), "agent"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := a.tokens.IsAgentMasterToken("master"), true; got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAgent_ReconnectConfigSettings(t *testing.T) {
	t.Parallel()
	func() {
		a := NewTestAgent(t.Name(), "")
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
		a := NewTestAgent(t.Name(), `
			reconnect_timeout = "24h"
			reconnect_timeout_wan = "36h"
		`)
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

func TestAgent_ReconnectConfigWanDisabled(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), `
		ports { serf_wan = -1 }
		reconnect_timeout_wan = "36h"
	`)
	defer a.Shutdown()

	// This is also testing that we dont panic like before #4515
	require.Nil(t, a.consulConfig().SerfWANConfig)
}

func TestAgent_setupNodeID(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		node_id = ""
	`)
	defer a.Shutdown()

	cfg := a.config

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
	a := NewTestAgent(t.Name(), `
		node_id = ""
	`)
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
	a.Config.DisableHostNodeID = false
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
	a := NewTestAgent(t.Name(), `
		node_name = "node1"
	`)
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
					ServiceTags: []string{"tag1"},
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
					ServiceTags: []string{"tag2"},
				},
				"check-noname": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "check-noname",
					Name:        "Service 'svcname2' check",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
					ServiceTags: []string{"tag2"},
				},
				"service:svcid2:3": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "service:svcid2:3",
					Name:        "check-noid",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
					ServiceTags: []string{"tag2"},
				},
				"service:svcid2:4": &structs.HealthCheck{
					Node:        "node1",
					CheckID:     "service:svcid2:4",
					Name:        "Service 'svcname2' check",
					Status:      "critical",
					ServiceID:   "svcid2",
					ServiceName: "svcname2",
					ServiceTags: []string{"tag2"},
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

				got, want := a.State.Services()[tt.srv.ID], tt.srv
				verify.Values(t, "", got, want)
			})

			// check the health checks
			for k, v := range tt.healthChks {
				t.Run(k, func(t *testing.T) {
					got, want := a.State.Checks()[types.CheckID(k)], v
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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Remove a service that doesn't exist
	if err := a.RemoveService("redis", false); err != nil {
		t.Fatalf("err: %v", err)
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
		if _, ok := a.State.Checks()["service:memcache"]; ok {
			t.Fatalf("have memcache check")
		}
		if _, ok := a.State.Checks()["check2"]; ok {
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
		if _, ok := a.State.Services()["redis"]; ok {
			t.Fatalf("have redis service")
		}

		// Ensure checks were removed
		if _, ok := a.State.Checks()["service:redis:1"]; ok {
			t.Fatalf("check redis:1 should be removed")
		}
		if _, ok := a.State.Checks()["service:redis:2"]; ok {
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
	a := NewTestAgent(t.Name(), `
		node_name = "node1"
	`)
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
	if a.State.Checks()["chk1"] == nil {
		t.Fatal("Could not find health check chk1")
	}

	// update the service with chk2
	if err := a.AddService(svc, []*structs.CheckType{chk2}, false, ""); err != nil {
		t.Fatal("Failed to update service", err)
	}

	// check that both checks are there
	if got, want := a.State.Checks()["chk1"], hchk1; !verify.Values(t, "", got, want) {
		t.FailNow()
	}
	if got, want := a.State.Checks()["chk2"], hchk2; !verify.Values(t, "", got, want) {
		t.FailNow()
	}

	// Remove service
	if err := a.RemoveService("redis", false); err != nil {
		t.Fatal("Failed to remove service", err)
	}

	// Check that both checks are gone
	if a.State.Checks()["chk1"] != nil {
		t.Fatal("Found health check chk1 want nil")
	}
	if a.State.Checks()["chk2"] != nil {
		t.Fatal("Found health check chk2 want nil")
	}
}

// TestAgent_IndexChurn is designed to detect a class of issues where
// we would have unnecessary catalog churn from anti-entropy. See issues
// #3259, #3642, #3845, and #3866.
func TestAgent_IndexChurn(t *testing.T) {
	t.Parallel()

	t.Run("no tags", func(t *testing.T) {
		verifyIndexChurn(t, nil)
	})

	t.Run("with tags", func(t *testing.T) {
		verifyIndexChurn(t, []string{"foo", "bar"})
	})
}

// verifyIndexChurn registers some things and runs anti-entropy a bunch of times
// in a row to make sure there are no index bumps.
func verifyIndexChurn(t *testing.T, tags []string) {
	t.Helper()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Ensure we have a leader before we start adding the services
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Tags:    tags,
	}
	if err := a.AddService(svc, nil, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk := &structs.HealthCheck{
		CheckID:   "redis-check",
		Name:      "Service-level check",
		ServiceID: "redis",
		Status:    api.HealthCritical,
	}
	chkt := &structs.CheckType{
		TTL: time.Hour,
	}
	if err := a.AddCheck(chk, chkt, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	chk = &structs.HealthCheck{
		CheckID: "node-check",
		Name:    "Node-level check",
		Status:  api.HealthCritical,
	}
	chkt = &structs.CheckType{
		TTL: time.Hour,
	}
	if err := a.AddCheck(chk, chkt, true, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.sync.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	args := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "redis",
	}
	var before structs.IndexedCheckServiceNodes

	// This sleep is so that the serfHealth check is added to the agent
	// A value of 375ms is sufficient enough time to ensure the serfHealth
	// check is added to an agent. 500ms so that we don't see flakiness ever.
	time.Sleep(500 * time.Millisecond)

	if err := a.RPC("Health.ServiceNodes", args, &before); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, name := range before.Nodes[0].Checks {
		a.logger.Println("[DEBUG] Checks Registered: ", name.Name)
	}
	if got, want := len(before.Nodes), 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got, want := len(before.Nodes[0].Checks), 3; /* incl. serfHealth */ got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	for i := 0; i < 10; i++ {
		a.logger.Println("[INFO] # ", i+1, "Sync in progress ")
		if err := a.sync.State.SyncFull(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	// If this test fails here this means that the Consul-X-Index
	// has changed for the RPC, which means that idempotent ops
	// are not working as intended.
	var after structs.IndexedCheckServiceNodes
	if err := a.RPC("Health.ServiceNodes", args, &after); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", after, before)
}

func TestAgent_AddCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk, ok := a.State.Checks()["mem"]
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
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthPassing,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk, ok := a.State.Checks()["mem"]
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
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	if _, ok := a.State.Checks()["mem"]; !ok {
		t.Fatalf("missing mem check")
	}

	// Ensure a TTL is setup
	if mon, ok := a.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	} else if mon.Interval != checks.MinInterval {
		t.Fatalf("bad mem monitor interval")
	}
}

func TestAgent_AddCheck_MissingService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "baz",
		Name:      "baz check 1",
		ServiceID: "baz",
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   time.Microsecond,
	}
	err := a.AddCheck(health, chk, false, "")
	if err == nil || err.Error() != `ServiceID "baz" does not exist` {
		t.Fatalf("expected service id error, got: %v", err)
	}
}

func TestAgent_AddCheck_RestoreState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create some state and persist it
	ttl := &checks.CheckTTL{
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
	checks := a.State.Checks()
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

func TestAgent_AddCheck_ExecDisable(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	if memChk := a.State.Checks()["mem"]; memChk != nil {
		t.Fatalf("should be missing mem check")
	}
}

func TestAgent_AddCheck_GRPC(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "grpchealth",
		Name:    "grpc health checking protocol",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		GRPC:     "localhost:12345/package.Service",
		Interval: 15 * time.Second,
	}
	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	sChk, ok := a.State.Checks()["grpchealth"]
	if !ok {
		t.Fatalf("missing grpchealth check")
	}

	// Ensure our check is in the right state
	if sChk.Status != api.HealthCritical {
		t.Fatalf("check not critical")
	}

	// Ensure a check is setup
	if _, ok := a.checkGRPCs["grpchealth"]; !ok {
		t.Fatalf("missing grpchealth check")
	}
}

func TestAgent_AddCheck_Alias(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "")
	require.NoError(err)

	// Ensure we have a check mapping
	sChk, ok := a.State.Checks()["aliashealth"]
	require.True(ok, "missing aliashealth check")
	require.NotNil(sChk)
	require.Equal(api.HealthCritical, sChk.Status)

	chkImpl, ok := a.checkAliases["aliashealth"]
	require.True(ok, "missing aliashealth check")
	require.Equal("", chkImpl.RPCReq.Token)

	cs := a.State.CheckState("aliashealth")
	require.NotNil(cs)
	require.Equal("", cs.Token)
}

func TestAgent_AddCheck_Alias_setToken(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "foo")
	require.NoError(err)

	cs := a.State.CheckState("aliashealth")
	require.NotNil(cs)
	require.Equal("foo", cs.Token)

	chkImpl, ok := a.checkAliases["aliashealth"]
	require.True(ok, "missing aliashealth check")
	require.Equal("foo", chkImpl.RPCReq.Token)
}

func TestAgent_AddCheck_Alias_userToken(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), `
acl_token = "hello"
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "")
	require.NoError(err)

	cs := a.State.CheckState("aliashealth")
	require.NotNil(cs)
	require.Equal("", cs.Token) // State token should still be empty

	chkImpl, ok := a.checkAliases["aliashealth"]
	require.True(ok, "missing aliashealth check")
	require.Equal("hello", chkImpl.RPCReq.Token) // Check should use the token
}

func TestAgent_AddCheck_Alias_userAndSetToken(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), `
acl_token = "hello"
	`)
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "aliashealth",
		Name:    "Alias health check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		AliasService: "foo",
	}
	err := a.AddCheck(health, chk, false, "goodbye")
	require.NoError(err)

	cs := a.State.CheckState("aliashealth")
	require.NotNil(cs)
	require.Equal("goodbye", cs.Token)

	chkImpl, ok := a.checkAliases["aliashealth"]
	require.True(ok, "missing aliashealth check")
	require.Equal("goodbye", chkImpl.RPCReq.Token)
}

func TestAgent_RemoveCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
	`)
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
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
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
	if _, ok := a.State.Checks()["mem"]; ok {
		t.Fatalf("have mem check")
	}

	// Ensure a TTL is setup
	if _, ok := a.checkMonitors["mem"]; ok {
		t.Fatalf("have mem monitor")
	}
}

func TestAgent_HTTPCheck_TLSSkipVerify(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "GOOD")
	})
	server := httptest.NewTLSServer(handler)
	defer server.Close()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "tls",
		Name:    "tls check",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		HTTP:          server.URL,
		Interval:      20 * time.Millisecond,
		TLSSkipVerify: true,
	}

	err := a.AddCheck(health, chk, false, "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		status := a.State.Checks()["tls"]
		if status.Status != api.HealthPassing {
			r.Fatalf("bad: %v", status.Status)
		}
		if !strings.Contains(status.Output, "GOOD") {
			r.Fatalf("bad: %v", status.Output)
		}
	})

}

func TestAgent_HTTPCheck_EnableAgentTLSForChecks(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, ca string) {
		a := &TestAgent{
			Name:   t.Name(),
			UseTLS: true,
			HCL: `
				enable_agent_tls_for_checks = true

				verify_incoming = true
				server_name = "consul.test"
				key_file = "../test/client_certs/server.key"
				cert_file = "../test/client_certs/server.crt"
			` + ca,
		}
		a.Start()
		defer a.Shutdown()

		health := &structs.HealthCheck{
			Node:    "foo",
			CheckID: "tls",
			Name:    "tls check",
			Status:  api.HealthCritical,
		}

		url := fmt.Sprintf("https://%s/v1/agent/self", a.srv.ln.Addr().String())
		chk := &structs.CheckType{
			HTTP:     url,
			Interval: 20 * time.Millisecond,
		}

		err := a.AddCheck(health, chk, false, "")
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Run(t, func(r *retry.R) {
			status := a.State.Checks()["tls"]
			if status.Status != api.HealthPassing {
				r.Fatalf("bad: %v", status.Status)
			}
			if !strings.Contains(status.Output, "200 OK") {
				r.Fatalf("bad: %v", status.Output)
			}
		})
	}

	// We need to test both methods of passing the CA info to ensure that
	// we propagate all the fields correctly. All the other fields are
	// covered by the HCL in the test run function.
	tests := []struct {
		desc   string
		config string
	}{
		{"ca_file", `ca_file = "../test/client_certs/rootca.crt"`},
		{"ca_path", `ca_path = "../test/client_certs/path"`},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			run(t, tt.config)
		})
	}
}

func TestAgent_updateTTLCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	status := a.State.Checks()["mem"]
	if status.Status != api.HealthPassing {
		t.Fatalf("bad: %v", status)
	}
	if status.Output != "foo" {
		t.Fatalf("bad: %v", status)
	}
}

func TestAgent_PersistService(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		server = false
		bootstrap = false
		data_dir = "` + dataDir + `"
	`
	a := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a.Start()
	defer os.RemoveAll(dataDir)
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
	a2 := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a2.Start()
	defer a2.Shutdown()

	restored := a2.State.ServiceState(svc.ID)
	if restored == nil {
		t.Fatalf("service %q missing", svc.ID)
	}
	if got, want := restored.Token, "mytoken"; got != want {
		t.Fatalf("got token %q want %q", got, want)
	}
	if got, want := restored.Service.Port, 8001; got != want {
		t.Fatalf("got port %d want %d", got, want)
	}
}

func TestAgent_persistedService_compat(t *testing.T) {
	t.Parallel()
	// Tests backwards compatibility of persisted services from pre-0.5.1
	a := NewTestAgent(t.Name(), "")
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
	services := a.State.Services()
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
	a := NewTestAgent(t.Name(), "")
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
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		data_dir = "` + dataDir + `"
		server = false
		bootstrap = false
	`
	a := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a.Start()
	defer a.Shutdown()
	defer os.RemoveAll(dataDir)

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
	a2 := &TestAgent{Name: t.Name() + "-a2", HCL: cfg + `
		service = {
			id = "redis"
			name = "redis"
			tags = ["bar"]
			port = 9000
		}
	`, DataDir: dataDir}
	a2.Start()
	defer a2.Shutdown()

	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc1.ID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted service")
	}
	result := a2.State.Service("redis")
	if result == nil {
		t.Fatalf("missing service registration")
	}
	if !reflect.DeepEqual(result.Tags, []string{"bar"}) || result.Port != 9000 {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_PersistProxy(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		server = false
		bootstrap = false
		data_dir = "` + dataDir + `"
	`
	a := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a.Start()
	defer os.RemoveAll(dataDir)
	defer a.Shutdown()

	require := require.New(t)
	assert := assert.New(t)

	// Add a service to proxy (precondition for AddProxy)
	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	require.NoError(a.AddService(svc1, nil, true, ""))

	// Add a proxy for it
	proxy := &structs.ConnectManagedProxy{
		TargetServiceID: svc1.ID,
		Command:         []string{"/bin/sleep", "3600"},
	}

	file := filepath.Join(a.Config.DataDir, proxyDir, stringHash("redis-proxy"))

	// Proxy is not persisted unless requested
	require.NoError(a.AddProxy(proxy, false, false, ""))
	_, err := os.Stat(file)
	require.Error(err, "proxy should not be persisted")

	// Proxy is  persisted if requested
	require.NoError(a.AddProxy(proxy, true, false, ""))
	_, err = os.Stat(file)
	require.NoError(err, "proxy should be persisted")

	content, err := ioutil.ReadFile(file)
	require.NoError(err)

	var gotProxy persistedProxy
	require.NoError(json.Unmarshal(content, &gotProxy))
	assert.Equal(proxy.Command, gotProxy.Proxy.Command)
	assert.Len(gotProxy.ProxyToken, 36) // sanity check for UUID

	// Updates service definition on disk
	proxy.Config = map[string]interface{}{
		"foo": "bar",
	}
	require.NoError(a.AddProxy(proxy, true, false, ""))

	content, err = ioutil.ReadFile(file)
	require.NoError(err)

	require.NoError(json.Unmarshal(content, &gotProxy))
	assert.Equal(gotProxy.Proxy.Command, proxy.Command)
	assert.Equal(gotProxy.Proxy.Config, proxy.Config)
	assert.Len(gotProxy.ProxyToken, 36) // sanity check for UUID

	a.Shutdown()

	// Should load it back during later start
	a2 := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a2.Start()
	defer a2.Shutdown()

	restored := a2.State.Proxy("redis-proxy")
	require.NotNil(restored)
	assert.Equal(gotProxy.ProxyToken, restored.ProxyToken)
	// Ensure the port that was auto picked at random is the same again
	assert.Equal(gotProxy.Proxy.ProxyService.Port, restored.Proxy.ProxyService.Port)
	assert.Equal(gotProxy.Proxy.Command, restored.Proxy.Command)
}

func TestAgent_PurgeProxy(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	require := require.New(t)

	// Add a service to proxy (precondition for AddProxy)
	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	require.NoError(a.AddService(svc1, nil, true, ""))

	// Add a proxy for it
	proxy := &structs.ConnectManagedProxy{
		TargetServiceID: svc1.ID,
		Command:         []string{"/bin/sleep", "3600"},
	}
	proxyID := "redis-proxy"
	require.NoError(a.AddProxy(proxy, true, false, ""))

	file := filepath.Join(a.Config.DataDir, proxyDir, stringHash("redis-proxy"))

	// Not removed
	require.NoError(a.RemoveProxy(proxyID, false))
	_, err := os.Stat(file)
	require.NoError(err, "should not be removed")

	// Re-add the proxy
	require.NoError(a.AddProxy(proxy, true, false, ""))

	// Removed
	require.NoError(a.RemoveProxy(proxyID, true))
	_, err = os.Stat(file)
	require.Error(err, "should be removed")
}

func TestAgent_PurgeProxyOnDuplicate(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		data_dir = "` + dataDir + `"
		server = false
		bootstrap = false
	`
	a := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a.Start()
	defer a.Shutdown()
	defer os.RemoveAll(dataDir)

	require := require.New(t)

	// Add a service to proxy (precondition for AddProxy)
	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	require.NoError(a.AddService(svc1, nil, true, ""))

	// Add a proxy for it
	proxy := &structs.ConnectManagedProxy{
		TargetServiceID: svc1.ID,
		Command:         []string{"/bin/sleep", "3600"},
	}
	proxyID := "redis-proxy"
	require.NoError(a.AddProxy(proxy, true, false, ""))

	a.Shutdown()

	// Try bringing the agent back up with the service already
	// existing in the config
	a2 := &TestAgent{Name: t.Name() + "-a2", HCL: cfg + `
		service = {
			id = "redis"
			name = "redis"
			tags = ["bar"]
			port = 9000
			connect {
				proxy {
					command = ["/bin/sleep", "3600"]
				}
			}
		}
	`, DataDir: dataDir}
	a2.Start()
	defer a2.Shutdown()

	file := filepath.Join(a.Config.DataDir, proxyDir, stringHash(proxyID))
	_, err := os.Stat(file)
	require.NoError(err, "Config File based proxies should be persisted too")

	result := a2.State.Proxy(proxyID)
	require.NotNil(result)
	require.Equal(proxy.Command, result.Proxy.Command)
}

func TestAgent_PersistCheck(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		data_dir = "` + dataDir + `"
		server = false
		bootstrap = false
		enable_script_checks = true
	`
	a := &TestAgent{Name: t.Name(), HCL: cfg, DataDir: dataDir}
	a.Start()
	defer os.RemoveAll(dataDir)
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    a.config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}
	chkType := &structs.CheckType{
		ScriptArgs: []string{"/bin/true"},
		Interval:   10 * time.Second,
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
	a2 := &TestAgent{Name: t.Name() + "-a2", HCL: cfg, DataDir: dataDir}
	a2.Start()
	defer a2.Shutdown()

	result := a2.State.Check(check.CheckID)
	if result == nil {
		t.Fatalf("bad: %#v", a2.State.Checks())
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
	if a2.State.CheckState(check.CheckID).Token != "mytoken" {
		t.Fatalf("bad: %s", a2.State.CheckState(check.CheckID).Token)
	}
}

func TestAgent_PurgeCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	nodeID := NodeID()
	dataDir := testutil.TempDir(t, "agent")
	a := NewTestAgent(t.Name(), `
	    node_id = "`+nodeID+`"
	    node_name = "Node `+nodeID+`"
		data_dir = "`+dataDir+`"
		server = false
		bootstrap = false
		enable_script_checks = true
	`)
	defer os.RemoveAll(dataDir)
	defer a.Shutdown()

	check1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
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
	a2 := NewTestAgent(t.Name()+"-a2", `
	    node_id = "`+nodeID+`"
	    node_name = "Node `+nodeID+`"
		data_dir = "`+dataDir+`"
		server = false
		bootstrap = false
		enable_script_checks = true
		check = {
			id = "mem"
			name = "memory check"
			notes = "my cool notes"
			args = ["/bin/check-redis.py"]
			interval = "30s"
		}
	`)
	defer a2.Shutdown()

	file := filepath.Join(dataDir, checksDir, checkIDHash(check1.CheckID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted check")
	}
	result := a2.State.Check("mem")
	if result == nil {
		t.Fatalf("missing check registration")
	}
	expected := &structs.HealthCheck{
		Node:    a2.Config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthCritical,
		Notes:   "my cool notes",
	}
	if got, want := result, expected; !verify.Values(t, "", got, want) {
		t.FailNow()
	}
}

func TestAgent_loadChecks_token(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		check = {
			id = "rabbitmq"
			name = "rabbitmq"
			token = "abc123"
			ttl = "10s"
		}
	`)
	defer a.Shutdown()

	checks := a.State.Checks()
	if _, ok := checks["rabbitmq"]; !ok {
		t.Fatalf("missing check")
	}
	if token := a.State.CheckToken("rabbitmq"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_unloadChecks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	for check := range a.State.Checks() {
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
	for check := range a.State.Checks() {
		if check == check1.CheckID {
			t.Fatalf("should have unloaded checks")
		}
	}
}

func TestAgent_loadServices_token(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
		}
	`)
	defer a.Shutdown()

	services := a.State.Services()
	if _, ok := services["rabbitmq"]; !ok {
		t.Fatalf("missing service")
	}
	if token := a.State.ServiceToken("rabbitmq"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_unloadServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	for id := range a.State.Services() {
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
	if len(a.State.Services()) != 0 {
		t.Fatalf("should have unloaded services")
	}
}

func TestAgent_loadProxies(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect {
				proxy {
					config {
						bind_port = 1234
					}
				}
			}
		}
	`)
	defer a.Shutdown()

	services := a.State.Services()
	if _, ok := services["rabbitmq"]; !ok {
		t.Fatalf("missing service")
	}
	if token := a.State.ServiceToken("rabbitmq"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
	if _, ok := services["rabbitmq-proxy"]; !ok {
		t.Fatalf("missing proxy service")
	}
	if token := a.State.ServiceToken("rabbitmq-proxy"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
	proxies := a.State.Proxies()
	if _, ok := proxies["rabbitmq-proxy"]; !ok {
		t.Fatalf("missing proxy")
	}
}

func TestAgent_loadProxies_nilProxy(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect {
			}
		}
	`)
	defer a.Shutdown()

	services := a.State.Services()
	require.Contains(t, services, "rabbitmq")
	require.Equal(t, "abc123", a.State.ServiceToken("rabbitmq"))
	require.NotContains(t, services, "rabbitme-proxy")
	require.Empty(t, a.State.Proxies())
}

func TestAgent_unloadProxies(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect {
				proxy {
					config {
						bind_port = 1234
					}
				}
			}
		}
	`)
	defer a.Shutdown()

	// Sanity check it's there
	require.NotNil(t, a.State.Proxy("rabbitmq-proxy"))

	// Unload all proxies
	if err := a.unloadProxies(); err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(a.State.Proxies()) != 0 {
		t.Fatalf("should have unloaded proxies")
	}
}

func TestAgent_Service_MaintenanceMode(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	check, ok := a.State.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical maintenance check")
	}

	// Check that the token was used to register the check
	if token := a.State.CheckToken(checkID); token != "mytoken" {
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
	if _, ok := a.State.Checks()[checkID]; ok {
		t.Fatalf("should have deregistered maintenance check")
	}

	// Enter service maintenance mode without providing a reason
	if err := a.EnableServiceMaintenance("redis", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was registered with the default notes
	check, ok = a.State.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical check")
	}
	if check.Notes != defaultServiceMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_Service_Reap(t *testing.T) {
	// t.Parallel() // timing test. no parallel
	a := NewTestAgent(t.Name(), `
		check_reap_interval = "50ms"
		check_deregister_interval_min = "0s"
	`)
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
			TTL:    25 * time.Millisecond,
			DeregisterCriticalServiceAfter: 200 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.AddService(svc, chkTypes, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail but before the check is reaped.
	time.Sleep(100 * time.Millisecond)
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Pass the TTL.
	if err := a.updateTTLCheck("service:redis", api.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail again.
	time.Sleep(100 * time.Millisecond)
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Wait for the reap.
	time.Sleep(400 * time.Millisecond)
	if _, ok := a.State.Services()["redis"]; ok {
		t.Fatalf("redis service should have been reaped")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}
}

func TestAgent_Service_NoReap(t *testing.T) {
	// t.Parallel() // timing test. no parallel
	a := NewTestAgent(t.Name(), `
		check_reap_interval = "50ms"
		check_deregister_interval_min = "0s"
	`)
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
			TTL:    25 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.AddService(svc, chkTypes, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's there and there's no critical check yet.
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have critical checks")
	}

	// Wait for the check TTL to fail.
	time.Sleep(200 * time.Millisecond)
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}

	// Wait a while and make sure it doesn't reap.
	time.Sleep(200 * time.Millisecond)
	if _, ok := a.State.Services()["redis"]; !ok {
		t.Fatalf("should have redis service")
	}
	if checks := a.State.CriticalCheckStates(); len(checks) != 1 {
		t.Fatalf("should have a critical check")
	}
}

func TestAgent_addCheck_restoresSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	check, ok := a.State.Checks()["service:redis"]
	if !ok {
		t.Fatalf("missing check")
	}
	if check.Status != api.HealthPassing {
		t.Fatalf("bad: %s", check.Status)
	}
}

func TestAgent_NodeMaintenanceMode(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Enter maintenance mode for the node
	a.EnableNodeMaintenance("broken", "mytoken")

	// Make sure the critical health check was added
	check, ok := a.State.Checks()[structs.NodeMaint]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}

	// Check that the token was used to register the check
	if token := a.State.CheckToken(structs.NodeMaint); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got: '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	a.DisableNodeMaintenance()

	// Ensure the check was deregistered
	if _, ok := a.State.Checks()[structs.NodeMaint]; ok {
		t.Fatalf("should have deregistered critical node check")
	}

	// Enter maintenance mode without passing a reason
	a.EnableNodeMaintenance("", "")

	// Make sure the check was registered with the default note
	check, ok = a.State.Checks()[structs.NodeMaint]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}
	if check.Notes != defaultNodeMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_checkStateSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	out, ok := a.State.Checks()[check1.CheckID]
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
	a := NewTestAgent(t.Name(), "")
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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create the TTL check to persist
	check := &checks.CheckTTL{
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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create a check whose state will expire immediately
	check := &checks.CheckTTL{
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
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// No error if the state does not exist
	if err := a.purgeCheckState("check1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Persist some state to the data dir
	check := &checks.CheckTTL{
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
		a := NewTestAgent(t.Name(), `
			server = true
		`)
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

func TestAgent_reloadWatches(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Normal watch with http addr set, should succeed
	newConf := *a.config
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Should fail to reload with connect watches
	newConf.Watches = []map[string]interface{}{
		{
			"type": "connect_roots",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err == nil || !strings.Contains(err.Error(), "not allowed in agent config") {
		t.Fatalf("bad: %s", err)
	}

	// Should still succeed with only HTTPS addresses
	newConf.HTTPSAddrs = newConf.HTTPAddrs
	newConf.HTTPAddrs = make([]net.Addr, 0)
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Should fail to reload with no http or https addrs
	newConf.HTTPSAddrs = make([]net.Addr, 0)
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err == nil || !strings.Contains(err.Error(), "watch plans require an HTTP or HTTPS endpoint") {
		t.Fatalf("bad: %s", err)
	}
}

func TestAgent_reloadWatchesHTTPS(t *testing.T) {
	t.Parallel()
	a := TestAgent{Name: t.Name(), UseTLS: true}
	a.Start()
	defer a.Shutdown()

	// Normal watch with http addr set, should succeed
	newConf := *a.config
	newConf.Watches = []map[string]interface{}{
		{
			"type": "key",
			"key":  "asdf",
			"args": []interface{}{"ls"},
		},
	}
	if err := a.reloadWatches(&newConf); err != nil {
		t.Fatalf("bad: %s", err)
	}
}

func TestAgent_AddProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc             string
		proxy, wantProxy *structs.ConnectManagedProxy
		wantTCPCheck     string
		wantErr          bool
	}{
		{
			desc: "basic proxy adding, unregistered service",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo": "bar",
				},
				TargetServiceID: "db", // non-existent service.
			},
			// Target service must be registered.
			wantErr: true,
		},
		{
			desc: "basic proxy adding, registered service",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo": "bar",
				},
				TargetServiceID: "web",
			},
			// Proxy will inherit agent's 0.0.0.0 bind address but we can't check that
			// so we should default to localhost in that case.
			wantTCPCheck: "127.0.0.1:20000",
			wantErr:      false,
		},
		{
			desc: "default global exec mode",
			proxy: &structs.ConnectManagedProxy{
				Command:         []string{"consul", "connect", "proxy"},
				TargetServiceID: "web",
			},
			wantProxy: &structs.ConnectManagedProxy{
				ExecMode:        structs.ProxyExecModeScript,
				Command:         []string{"consul", "connect", "proxy"},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.0.0.1:20000",
			wantErr:      false,
		},
		{
			desc: "default daemon command",
			proxy: &structs.ConnectManagedProxy{
				ExecMode:        structs.ProxyExecModeDaemon,
				TargetServiceID: "web",
			},
			wantProxy: &structs.ConnectManagedProxy{
				ExecMode:        structs.ProxyExecModeDaemon,
				Command:         []string{"foo", "bar"},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.0.0.1:20000",
			wantErr:      false,
		},
		{
			desc: "default script command",
			proxy: &structs.ConnectManagedProxy{
				ExecMode:        structs.ProxyExecModeScript,
				TargetServiceID: "web",
			},
			wantProxy: &structs.ConnectManagedProxy{
				ExecMode:        structs.ProxyExecModeScript,
				Command:         []string{"bar", "foo"},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.0.0.1:20000",
			wantErr:      false,
		},
		{
			desc: "managed proxy with custom bind port",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo":          "bar",
					"bind_address": "127.10.10.10",
					"bind_port":    1234,
				},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.10.10.10:1234",
			wantErr:      false,
		},
		{
			// This test is necessary since JSON and HCL both will parse
			// numbers as a float64.
			desc: "managed proxy with custom bind port (float64)",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo":          "bar",
					"bind_address": "127.10.10.10",
					"bind_port":    float64(1234),
				},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.10.10.10:1234",
			wantErr:      false,
		},
		{
			desc: "managed proxy with overridden but unspecified ipv6 bind address",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo":          "bar",
					"bind_address": "[::]",
				},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.0.0.1:20000",
			wantErr:      false,
		},
		{
			desc: "managed proxy with overridden check address",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo":               "bar",
					"tcp_check_address": "127.20.20.20",
				},
				TargetServiceID: "web",
			},
			wantTCPCheck: "127.20.20.20:20000",
			wantErr:      false,
		},
		{
			desc: "managed proxy with disabled check",
			proxy: &structs.ConnectManagedProxy{
				ExecMode: structs.ProxyExecModeDaemon,
				Command:  []string{"consul", "connect", "proxy"},
				Config: map[string]interface{}{
					"foo":               "bar",
					"disable_tcp_check": true,
				},
				TargetServiceID: "web",
			},
			wantTCPCheck: "",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require := require.New(t)

			a := NewTestAgent(t.Name(), `
				node_name = "node1"

				# Explicit test because proxies inheriting this value must have a health
				# check on a different IP.
				bind_addr = "0.0.0.0"

				connect {
					proxy_defaults {
						exec_mode = "script"
						daemon_command = ["foo", "bar"]
						script_command = ["bar", "foo"]
					}
				}

				ports {
					proxy_min_port = 20000
					proxy_max_port = 20000
				}
			`)
			defer a.Shutdown()

			// Register a target service we can use
			reg := &structs.NodeService{
				Service: "web",
				Port:    8080,
			}
			require.NoError(a.AddService(reg, nil, false, ""))

			err := a.AddProxy(tt.proxy, false, false, "")
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)

			// Test the ID was created as we expect.
			got := a.State.Proxy("web-proxy")
			wantProxy := tt.wantProxy
			if wantProxy == nil {
				wantProxy = tt.proxy
			}
			wantProxy.ProxyService = got.Proxy.ProxyService
			require.Equal(wantProxy, got.Proxy)

			// Ensure a TCP check was created for the service.
			gotCheck := a.State.Check("service:web-proxy")
			if tt.wantTCPCheck == "" {
				require.Nil(gotCheck)
			} else {
				require.NotNil(gotCheck)
				require.Equal("Connect Proxy Listening", gotCheck.Name)

				// Confusingly, a.State.Check("service:web-proxy") will return the state
				// but it's Definition field will be empty. This appears to be expected
				// when adding Checks as part of `AddService`. Notice how `AddService`
				// tests in this file don't assert on that state but instead look at the
				// agent's check state directly to ensure the right thing was registered.
				// We'll do the same for now.
				gotTCP, ok := a.checkTCPs["service:web-proxy"]
				require.True(ok)
				require.Equal(tt.wantTCPCheck, gotTCP.TCP)
				require.Equal(10*time.Second, gotTCP.Interval)
			}
		})
	}
}

func TestAgent_RemoveProxy(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		node_name = "node1"
	`)
	defer a.Shutdown()
	require := require.New(t)

	// Register a target service we can use
	reg := &structs.NodeService{
		Service: "web",
		Port:    8080,
	}
	require.NoError(a.AddService(reg, nil, false, ""))

	// Add a proxy for web
	pReg := &structs.ConnectManagedProxy{
		TargetServiceID: "web",
		ExecMode:        structs.ProxyExecModeDaemon,
		Command:         []string{"foo"},
	}
	require.NoError(a.AddProxy(pReg, false, false, ""))

	// Test the ID was created as we expect.
	gotProxy := a.State.Proxy("web-proxy")
	require.NotNil(gotProxy)

	err := a.RemoveProxy("web-proxy", false)
	require.NoError(err)

	gotProxy = a.State.Proxy("web-proxy")
	require.Nil(gotProxy)
	require.Nil(a.State.Service("web-proxy"), "web-proxy service")

	// Removing invalid proxy should be an error
	err = a.RemoveProxy("foobar", false)
	require.Error(err)
}

func TestAgent_ReLoadProxiesFromConfig(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(),
		`node_name = "node1"
	`)
	defer a.Shutdown()
	require := require.New(t)

	// Register a target service we can use
	reg := &structs.NodeService{
		Service: "web",
		Port:    8080,
	}
	require.NoError(a.AddService(reg, nil, false, ""))

	proxies := a.State.Proxies()
	require.Len(proxies, 0)

	config := config.RuntimeConfig{
		Services: []*structs.ServiceDefinition{
			&structs.ServiceDefinition{
				Name: "web",
				Connect: &structs.ServiceConnect{
					Native: false,
					Proxy:  &structs.ServiceDefinitionConnectProxy{},
				},
			},
		},
	}

	require.NoError(a.loadProxies(&config))

	// ensure we loaded the proxy
	proxies = a.State.Proxies()
	require.Len(proxies, 1)

	// store the auto-generated token
	ptok := ""
	pid := ""
	for id := range proxies {
		pid = id
		ptok = proxies[id].ProxyToken
		break
	}

	// reload the proxies and ensure the proxy token is the same
	require.NoError(a.unloadProxies())
	proxies = a.State.Proxies()
	require.Len(proxies, 0)
	require.NoError(a.loadProxies(&config))
	proxies = a.State.Proxies()
	require.Len(proxies, 1)
	require.Equal(ptok, proxies[pid].ProxyToken)

	// make sure when the config goes away so does the proxy
	require.NoError(a.unloadProxies())
	proxies = a.State.Proxies()
	require.Len(proxies, 0)

	// a.config contains no services or proxies
	require.NoError(a.loadProxies(a.config))
	proxies = a.State.Proxies()
	require.Len(proxies, 0)
}
