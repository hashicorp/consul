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

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/pascaldekloe/goe/verify"
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

func TestAgent_StartStop(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
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
	if err := a.RPC("Health.ServiceNodes", args, &before); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := len(before.Nodes), 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got, want := len(before.Nodes[0].Checks), 3; /* incl. serfHealth */ got != want {
		t.Fatalf("got %d want %d", got, want)
	}

	for i := 0; i < 10; i++ {
		if err := a.sync.State.SyncFull(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

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
		Script:   "exit 0",
		Interval: 15 * time.Second,
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
		Script:   "exit 0",
		Interval: 15 * time.Second,
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
		Script:   "exit 0",
		Interval: time.Microsecond,
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
		Script:   "exit 0",
		Interval: 15 * time.Second,
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
			script = "/bin/check-redis.py"
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
