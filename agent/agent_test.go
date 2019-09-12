package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"

	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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
			a := NewTestAgent(t, t.Name(), "")
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
		wantErr       bool
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
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a rare case where using a constructor for TestAgent
			// (NewTestAgent and the likes) won't work, since we expect an error
			// in one test case, and the constructors have built-in retry logic
			// that runs automatically upon error.
			a := &TestAgent{Name: tt.name, HCL: tt.hcl, LogOutput: testutil.TestWriter(t)}
			err := a.Start()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return // don't run the rest of the test
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
			defer a.Shutdown()

			cfg := a.consulConfig()
			assert.Equal(t, tt.wantClusterID, cfg.CAConfig.ClusterID)
		})
	}
}

func TestAgent_StartStop(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	var out struct{}
	if err := a.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_TokenStore(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), `
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
		a := NewTestAgent(t, t.Name(), "")
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
		a := NewTestAgent(t, t.Name(), `
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

	a := NewTestAgent(t, t.Name(), `
		ports { serf_wan = -1 }
		reconnect_timeout_wan = "36h"
	`)
	defer a.Shutdown()

	// This is also testing that we dont panic like before #4515
	require.Nil(t, a.consulConfig().SerfWANConfig)
}

func TestAgent_setupNodeID(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
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
	a := NewTestAgent(t, t.Name(), `
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
	a := NewTestAgent(t, t.Name(), `
		node_name = "node1"
	`)
	defer a.Shutdown()

	tests := []struct {
		desc       string
		srv        *structs.NodeService
		wantSrv    func(ns *structs.NodeService)
		chkTypes   []*structs.CheckType
		healthChks map[string]*structs.HealthCheck
	}{
		{
			"one check",
			&structs.NodeService{
				ID:      "svcid1",
				Service: "svcname1",
				Tags:    []string{"tag1"},
				Weights: nil, // nil weights...
				Port:    8100,
			},
			// ... should be populated to avoid "IsSame" returning true during AE.
			func(ns *structs.NodeService) {
				ns.Weights = &structs.Weights{
					Passing: 1,
					Warning: 1,
				}
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
				Weights: &structs.Weights{
					Passing: 2,
					Warning: 1,
				},
				Tags: []string{"tag2"},
				Port: 8200,
			},
			nil, // No change expected
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
				err := a.AddService(tt.srv, tt.chkTypes, false, "", ConfigSourceLocal)
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				got := a.State.Services()[tt.srv.ID]
				// Make a copy since the tt.srv points to the one in memory in the local
				// state still so changing it is a tautology!
				want := *tt.srv
				if tt.wantSrv != nil {
					tt.wantSrv(&want)
				}
				require.Equal(t, &want, got)
				require.True(t, got.IsSame(&want))
			})

			// check the health checks
			for k, v := range tt.healthChks {
				t.Run(k, func(t *testing.T) {
					got := a.State.Checks()[types.CheckID(k)]
					require.Equal(t, v, got)
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

func TestAgent_AddServices_AliasUpdateCheckNotReverted(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		node_name = "node1"
	`)
	defer a.Shutdown()

	// It's tricky to get an UpdateCheck call to be timed properly so it lands
	// right in the middle of an addServiceInternal call so we cheat a bit and
	// rely upon alias checks to do that work for us.  We add enough services
	// that probabilistically one of them is going to end up properly in the
	// critical section.
	//
	// The first number I picked here (10) surprisingly failed every time prior
	// to PR #6144 solving the underlying problem.
	const numServices = 10

	services := make([]*structs.ServiceDefinition, numServices)
	checkIDs := make([]types.CheckID, numServices)
	for i := 0; i < numServices; i++ {
		name := fmt.Sprintf("web-%d", i)

		services[i] = &structs.ServiceDefinition{
			ID:   name,
			Name: name,
			Port: 8080 + i,
			Checks: []*structs.CheckType{
				&structs.CheckType{
					Name:         "alias-for-fake-service",
					AliasService: "fake",
				},
			},
		}

		checkIDs[i] = types.CheckID("service:" + name)
	}

	// Add all of the services quickly as you might do from config file snippets.
	for _, service := range services {
		ns := service.NodeService()

		chkTypes, err := service.CheckTypes()
		require.NoError(t, err)

		require.NoError(t, a.AddService(ns, chkTypes, false, service.Token, ConfigSourceLocal))
	}

	retry.Run(t, func(r *retry.R) {
		gotChecks := a.State.Checks()
		for id, check := range gotChecks {
			require.Equal(r, "passing", check.Status, "check %q is wrong", id)
			require.Equal(r, "No checks found.", check.Output, "check %q is wrong", id)
		}
	})
}

func TestAgent_AddServiceNoExec(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		node_name = "node1"
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		ID:      "svcid1",
		Service: "svcname1",
		Tags:    []string{"tag1"},
		Port:    8100,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}

	err := a.AddService(srv, []*structs.CheckType{chk}, false, "", ConfigSourceLocal)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	err = a.AddService(srv, []*structs.CheckType{chk}, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_AddServiceNoRemoteExec(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		node_name = "node1"
		enable_local_script_checks = true
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	srv := &structs.NodeService{
		ID:      "svcid1",
		Service: "svcname1",
		Tags:    []string{"tag1"},
		Port:    8100,
	}
	chk := &structs.CheckType{
		ScriptArgs: []string{"exit", "0"},
		Interval:   15 * time.Second,
	}

	err := a.AddService(srv, []*structs.CheckType{chk}, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_RemoveService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
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

		if err := a.AddService(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
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
		if err := a.AddCheck(hc, check.CheckType(), false, "", ConfigSourceLocal); err != nil {
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
		// add a service to remove
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Port:    8000,
		}
		chkTypes := []*structs.CheckType{
			&structs.CheckType{TTL: time.Minute},
			&structs.CheckType{TTL: 30 * time.Second},
		}
		if err := a.AddService(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
			t.Fatalf("err: %v", err)
		}

		// add another service that wont be affected
		srv = &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Port:    3306,
		}
		chkTypes = []*structs.CheckType{
			&structs.CheckType{TTL: time.Minute},
			&structs.CheckType{TTL: 30 * time.Second},
		}
		if err := a.AddService(srv, chkTypes, false, "", ConfigSourceLocal); err != nil {
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

		// Ensure the redis checks are removed
		if _, ok := a.checkTTLs["service:redis:1"]; ok {
			t.Fatalf("check ttl for redis:1 should be removed")
		}
		if check := a.State.Check(types.CheckID("service:redis:1")); check != nil {
			t.Fatalf("check ttl for redis:1 should be removed")
		}
		if _, ok := a.checkTTLs["service:redis:2"]; ok {
			t.Fatalf("check ttl for redis:2 should be removed")
		}
		if check := a.State.Check(types.CheckID("service:redis:2")); check != nil {
			t.Fatalf("check ttl for redis:2 should be removed")
		}

		// check the mysql service is unnafected
		if _, ok := a.checkTTLs["service:mysql:1"]; !ok {
			t.Fatalf("check ttl for mysql:1 should not be removed")
		}
		if check := a.State.Check(types.CheckID("service:mysql:1")); check == nil {
			t.Fatalf("check ttl for mysql:1 should not be removed")
		}
		if _, ok := a.checkTTLs["service:mysql:2"]; !ok {
			t.Fatalf("check ttl for mysql:2 should not be removed")
		}
		if check := a.State.Check(types.CheckID("service:mysql:2")); check == nil {
			t.Fatalf("check ttl for mysql:2 should not be removed")
		}
	}
}

func TestAgent_RemoveServiceRemovesAllChecks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		node_name = "node1"
	`)
	defer a.Shutdown()

	svc := &structs.NodeService{ID: "redis", Service: "redis", Port: 8000}
	chk1 := &structs.CheckType{CheckID: "chk1", Name: "chk1", TTL: time.Minute}
	chk2 := &structs.CheckType{CheckID: "chk2", Name: "chk2", TTL: 2 * time.Minute}
	hchk1 := &structs.HealthCheck{Node: "node1", CheckID: "chk1", Name: "chk1", Status: "critical", ServiceID: "redis", ServiceName: "redis"}
	hchk2 := &structs.HealthCheck{Node: "node1", CheckID: "chk2", Name: "chk2", Status: "critical", ServiceID: "redis", ServiceName: "redis"}

	// register service with chk1
	if err := a.AddService(svc, []*structs.CheckType{chk1}, false, "", ConfigSourceLocal); err != nil {
		t.Fatal("Failed to register service", err)
	}

	// verify chk1 exists
	if a.State.Checks()["chk1"] == nil {
		t.Fatal("Could not find health check chk1")
	}

	// update the service with chk2
	if err := a.AddService(svc, []*structs.CheckType{chk2}, false, "", ConfigSourceLocal); err != nil {
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

	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	weights := &structs.Weights{
		Passing: 1,
		Warning: 1,
	}
	// Ensure we have a leader before we start adding the services
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
		Tags:    tags,
		Weights: weights,
	}
	if err := a.AddService(svc, nil, true, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(chk, chkt, true, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(chk, chkt, true, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err == nil || err.Error() != `ServiceID "baz" does not exist` {
		t.Fatalf("expected service id error, got: %v", err)
	}
}

func TestAgent_AddCheck_RestoreState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
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
	err = a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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

	a := NewTestAgent(t, t.Name(), "")
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	if memChk := a.State.Checks()["mem"]; memChk != nil {
		t.Fatalf("should be missing mem check")
	}

	err = a.AddCheck(health, chk, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	if memChk := a.State.Checks()["mem"]; memChk != nil {
		t.Fatalf("should be missing mem check")
	}
}

func TestAgent_AddCheck_ExecRemoteDisable(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), `
		enable_local_script_checks = true
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	err := a.AddCheck(health, chk, false, "", ConfigSourceRemote)
	if err == nil || !strings.Contains(err.Error(), "Scripts are disabled on this agent from remote calls") {
		t.Fatalf("err: %v", err)
	}

	// Ensure we don't have a check mapping
	if memChk := a.State.Checks()["mem"]; memChk != nil {
		t.Fatalf("should be missing mem check")
	}
}

func TestAgent_AddCheck_GRPC(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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

func TestAgent_RestoreServiceWithAliasCheck(t *testing.T) {
	// t.Parallel() don't even think about making this parallel

	// This test is very contrived and tests for the absence of race conditions
	// related to the implementation of alias checks. As such it is slow,
	// serial, full of sleeps and retries, and not generally a great test to
	// run all of the time.
	//
	// That said it made it incredibly easy to root out various race conditions
	// quite successfully.
	//
	// The original set of races was between:
	//
	//   - agent startup reloading Services and Checks from disk
	//   - API requests to also re-register those same Services and Checks
	//   - the goroutines for the as-yet-to-be-stopped CheckAlias goroutines

	if os.Getenv("SLOWTEST") != "1" {
		t.Skip("skipping slow test; set SLOWTEST=1 to run")
		return
	}

	// We do this so that the agent logs and the informational messages from
	// the test itself are interwoven properly.
	logf := func(t *testing.T, a *TestAgent, format string, args ...interface{}) {
		a.logger.Printf("[INFO] testharness: "+format, args...)
	}

	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	cfg := `
		server = false
		bootstrap = false
	    enable_central_service_config = false
		data_dir = "` + dataDir + `"
	`
	a := NewTestAgentWithFields(t, true, TestAgent{HCL: cfg, DataDir: dataDir})
	defer os.RemoveAll(dataDir)
	defer a.Shutdown()

	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	testHTTPServer := launchHTTPCheckServer(t, testCtx)
	defer testHTTPServer.Close()

	registerServicesAndChecks := func(t *testing.T, a *TestAgent) {
		// add one persistent service with a simple check
		require.NoError(t, a.AddService(
			&structs.NodeService{
				ID:      "ping",
				Service: "ping",
				Port:    8000,
			},
			[]*structs.CheckType{
				&structs.CheckType{
					HTTP:     testHTTPServer.URL,
					Method:   "GET",
					Interval: 5 * time.Second,
					Timeout:  1 * time.Second,
				},
			},
			true, "", ConfigSourceLocal,
		))

		// add one persistent sidecar service with an alias check in the manner
		// of how sidecar_service would add it
		require.NoError(t, a.AddService(
			&structs.NodeService{
				ID:      "ping-sidecar-proxy",
				Service: "ping-sidecar-proxy",
				Port:    9000,
			},
			[]*structs.CheckType{
				&structs.CheckType{
					Name:         "Connect Sidecar Aliasing ping",
					AliasService: "ping",
				},
			},
			true, "", ConfigSourceLocal,
		))
	}

	retryUntilCheckState := func(t *testing.T, a *TestAgent, checkID string, expectedStatus string) {
		t.Helper()
		retry.Run(t, func(r *retry.R) {
			chk := a.State.CheckState(types.CheckID(checkID))
			if chk == nil {
				r.Fatalf("check=%q is completely missing", checkID)
			}
			if chk.Check.Status != expectedStatus {
				logf(t, a, "check=%q expected status %q but got %q", checkID, expectedStatus, chk.Check.Status)
				r.Fatalf("check=%q expected status %q but got %q", checkID, expectedStatus, chk.Check.Status)
			}
			logf(t, a, "check %q has reached desired status %q", checkID, expectedStatus)
		})
	}

	registerServicesAndChecks(t, a)

	time.Sleep(1 * time.Second)

	retryUntilCheckState(t, a, "service:ping", api.HealthPassing)
	retryUntilCheckState(t, a, "service:ping-sidecar-proxy", api.HealthPassing)

	logf(t, a, "==== POWERING DOWN ORIGINAL ====")

	require.NoError(t, a.Shutdown())

	time.Sleep(1 * time.Second)

	futureHCL := cfg + `
node_id = "` + string(a.Config.NodeID) + `"
node_name = "` + a.Config.NodeName + `"
	`

	restartOnce := func(idx int, t *testing.T) {
		t.Helper()

		// Reload and retain former NodeID and data directory.
		a2 := NewTestAgentWithFields(t, true, TestAgent{HCL: futureHCL, DataDir: dataDir})
		defer a2.Shutdown()
		a = nil

		// reregister during standup; we use an adjustable timing to try and force a race
		sleepDur := time.Duration(idx+1) * 500 * time.Millisecond
		time.Sleep(sleepDur)
		logf(t, a2, "re-registering checks and services after a delay of %v", sleepDur)
		for i := 0; i < 20; i++ { // RACE RACE RACE!
			registerServicesAndChecks(t, a2)
			time.Sleep(50 * time.Millisecond)
		}

		time.Sleep(1 * time.Second)

		retryUntilCheckState(t, a2, "service:ping", api.HealthPassing)

		logf(t, a2, "giving the alias check a chance to notice...")
		time.Sleep(5 * time.Second)

		retryUntilCheckState(t, a2, "service:ping-sidecar-proxy", api.HealthPassing)
	}

	for i := 0; i < 20; i++ {
		name := "restart-" + strconv.Itoa(i)
		ok := t.Run(name, func(t *testing.T) {
			restartOnce(i, t)
		})
		require.True(t, ok, name+" failed")
	}
}

func launchHTTPCheckServer(t *testing.T, ctx context.Context) *httptest.Server {
	ports := freeport.GetT(t, 1)
	port := ports[0]

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", addr)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})

	srv := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	srv.Start()
	return srv
}

func TestAgent_AddCheck_Alias(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, t.Name(), "")
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), "")
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
	err := a.AddCheck(health, chk, false, "foo", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "goodbye", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), `
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
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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

	a := NewTestAgent(t, t.Name(), "")
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

	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
		a := NewTestAgentWithFields(t, true, TestAgent{
			Name:   t.Name(),
			UseTLS: true,
			HCL: `
				enable_agent_tls_for_checks = true

				verify_incoming = true
				server_name = "consul.test"
				key_file = "../test/client_certs/server.key"
				cert_file = "../test/client_certs/server.crt"
			` + ca,
		})
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

		err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	checkBufSize := 100
	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  api.HealthCritical,
	}
	chk := &structs.CheckType{
		TTL:           15 * time.Second,
		OutputMaxSize: checkBufSize,
	}

	// Add check and update it.
	err := a.AddCheck(health, chk, false, "", ConfigSourceLocal)
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

	if err := a.updateTTLCheck("mem", api.HealthCritical, strings.Repeat("--bad-- ", 5*checkBufSize)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping.
	status = a.State.Checks()["mem"]
	if status.Status != api.HealthCritical {
		t.Fatalf("bad: %v", status)
	}
	if len(status.Output) > checkBufSize*2 {
		t.Fatalf("bad: %v", len(status.Output))
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
	a := NewTestAgentWithFields(t, true, TestAgent{HCL: cfg, DataDir: dataDir})
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
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Persists to file if requested
	if err := a.AddService(svc, nil, true, "mytoken", ConfigSourceLocal); err != nil {
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
	if err := a.AddService(svc, nil, true, "mytoken", ConfigSourceLocal); err != nil {
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
	a2 := NewTestAgentWithFields(t, true, TestAgent{HCL: cfg, DataDir: dataDir})
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
		Weights: &structs.Weights{Passing: 1, Warning: 1},
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
	require.Equal(t, svc, result)
}

func TestAgent_PurgeService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(a.Config.DataDir, servicesDir, stringHash(svc.ID))
	if err := a.AddService(svc, nil, true, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddService(svc, nil, true, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgentWithFields(t, true, TestAgent{HCL: cfg, DataDir: dataDir})
	defer a.Shutdown()
	defer os.RemoveAll(dataDir)

	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// First persist the service
	if err := a.AddService(svc1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	a.Shutdown()

	// Try bringing the agent back up with the service already
	// existing in the config
	a2 := NewTestAgentWithFields(t, true, TestAgent{Name: t.Name() + "-a2", HCL: cfg + `
		service = {
			id = "redis"
			name = "redis"
			tags = ["bar"]
			port = 9000
		}
	`, DataDir: dataDir})
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
	a := NewTestAgentWithFields(t, true, TestAgent{HCL: cfg, DataDir: dataDir})
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
	if err := a.AddCheck(check, chkType, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Should persist if requested
	if err := a.AddCheck(check, chkType, true, "mytoken", ConfigSourceLocal); err != nil {
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
		t.Fatalf("bad: %s != %s", string(content), expected)
	}

	// Updates the check definition on disk
	check.Name = "mem1"
	if err := a.AddCheck(check, chkType, true, "mytoken", ConfigSourceLocal); err != nil {
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
	a2 := NewTestAgentWithFields(t, true, TestAgent{Name: t.Name() + "-a2", HCL: cfg, DataDir: dataDir})
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	check := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  api.HealthPassing,
	}

	file := filepath.Join(a.Config.DataDir, checksDir, checkIDHash(check.CheckID))
	if err := a.AddCheck(check, nil, true, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), `
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
	if err := a.AddCheck(check1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %v", err)
	}
	a.Shutdown()

	// Start again with the check registered in config
	a2 := NewTestAgent(t, t.Name()+"-a2", `
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
	a := NewTestAgent(t, t.Name(), `
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(check1, nil, false, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), `
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

func TestAgent_loadServices_sidecar(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect = {
				sidecar_service {}
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
	if _, ok := services["rabbitmq-sidecar-proxy"]; !ok {
		t.Fatalf("missing service")
	}
	if token := a.State.ServiceToken("rabbitmq-sidecar-proxy"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Sanity check rabbitmq service should NOT have sidecar info in state since
	// it's done it's job and should be a registration syntax sugar only.
	assert.Nil(t, services["rabbitmq"].Connect.SidecarService)
}

func TestAgent_loadServices_sidecarSeparateToken(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			token = "abc123"
			connect = {
				sidecar_service {
					token = "789xyz"
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
	if _, ok := services["rabbitmq-sidecar-proxy"]; !ok {
		t.Fatalf("missing service")
	}
	if token := a.State.ServiceToken("rabbitmq-sidecar-proxy"); token != "789xyz" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_loadServices_sidecarInheritMeta(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			tags = ["a", "b"],
			meta = {
				environment = "prod"
			}
			connect = {
				sidecar_service {

				}
			}
		}
	`)
	defer a.Shutdown()

	services := a.State.Services()

	svc, ok := services["rabbitmq"]
	require.True(t, ok, "missing service")
	require.Len(t, svc.Tags, 2)
	require.Len(t, svc.Meta, 1)

	sidecar, ok := services["rabbitmq-sidecar-proxy"]
	require.True(t, ok, "missing sidecar service")
	require.ElementsMatch(t, svc.Tags, sidecar.Tags)
	require.Len(t, sidecar.Meta, 1)
	meta, ok := sidecar.Meta["environment"]
	require.True(t, ok, "missing sidecar service meta")
	require.Equal(t, "prod", meta)
}

func TestAgent_loadServices_sidecarOverrideMeta(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), `
		service = {
			id = "rabbitmq"
			name = "rabbitmq"
			port = 5672
			tags = ["a", "b"],
			meta = {
				environment = "prod"
			}
			connect = {
				sidecar_service {
					tags = ["foo"],
					meta = {
						environment = "qa"
					}
				}
			}
		}
	`)
	defer a.Shutdown()

	services := a.State.Services()

	svc, ok := services["rabbitmq"]
	require.True(t, ok, "missing service")
	require.Len(t, svc.Tags, 2)
	require.Len(t, svc.Meta, 1)

	sidecar, ok := services["rabbitmq-sidecar-proxy"]
	require.True(t, ok, "missing sidecar service")
	require.Len(t, sidecar.Tags, 1)
	require.Equal(t, "foo", sidecar.Tags[0])
	require.Len(t, sidecar.Meta, 1)
	meta, ok := sidecar.Meta["environment"]
	require.True(t, ok, "missing sidecar service meta")
	require.Equal(t, "qa", meta)
}

func TestAgent_unloadServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), `
		check_reap_interval = "50ms"
		check_deregister_interval_min = "0s"
	`)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chkTypes := []*structs.CheckType{
		&structs.CheckType{
			Status:                         api.HealthPassing,
			TTL:                            25 * time.Millisecond,
			DeregisterCriticalServiceAfter: 200 * time.Millisecond,
		},
	}

	// Register the service.
	if err := a.AddService(svc, chkTypes, false, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), `
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
	if err := a.AddService(svc, chkTypes, false, "", ConfigSourceLocal); err != nil {
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

func TestAgent_AddService_restoresSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(check1, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-registering the service preserves the state of the check
	chkTypes := []*structs.CheckType{&structs.CheckType{TTL: 30 * time.Second}}
	if err := a.AddService(svc, chkTypes, false, "", ConfigSourceLocal); err != nil {
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

func TestAgent_AddCheck_restoresSnapshot(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(check1, nil, false, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Re-registering the check preserves its state
	check1.Status = ""
	if err := a.AddCheck(check1, &structs.CheckType{TTL: 30 * time.Second}, false, "", ConfigSourceLocal); err != nil {
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
	a := NewTestAgent(t, t.Name(), "")
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
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := a.AddService(svc, nil, false, "", ConfigSourceLocal); err != nil {
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
	if err := a.AddCheck(check1, nil, true, "", ConfigSourceLocal); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Snapshot the state
	snap := a.snapshotCheckState()

	// Unload all of the checks
	if err := a.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Reload the checks and restore the snapshot.
	if err := a.loadChecks(a.Config, snap); err != nil {
		t.Fatalf("err: %s", err)
	}

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
	a := NewTestAgent(t, t.Name(), "")
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
	if err := a.loadChecks(a.Config, nil); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the erroneous check was purged
	if _, err := os.Stat(checkPath); err == nil {
		t.Fatalf("should have purged check")
	}
}

func TestAgent_persistCheckState(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), "")
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
	a := NewTestAgent(t, t.Name(), "")
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
	a := NewTestAgent(t, t.Name(), "")
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
		a := NewTestAgent(t, t.Name(), `
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
	a := NewTestAgent(t, t.Name(), "")
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
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
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

func TestAgent_loadTokens(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t, t.Name(), `
		acl = {
			enabled = true
			tokens = {
				agent = "alfa"
				agent_master = "bravo",
				default = "charlie"
				replication = "delta"
			}
		}

	`)
	defer a.Shutdown()
	require := require.New(t)

	tokensFullPath := filepath.Join(a.config.DataDir, tokensPath)

	t.Run("original-configuration", func(t *testing.T) {
		require.Equal("alfa", a.tokens.AgentToken())
		require.Equal("bravo", a.tokens.AgentMasterToken())
		require.Equal("charlie", a.tokens.UserToken())
		require.Equal("delta", a.tokens.ReplicationToken())
	})

	t.Run("updated-configuration", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentMasterToken: "golf",
			ACLReplicationToken: "hotel",
		}
		// ensures no error for missing persisted tokens file
		require.NoError(a.loadTokens(cfg))
		require.Equal("echo", a.tokens.UserToken())
		require.Equal("foxtrot", a.tokens.AgentToken())
		require.Equal("golf", a.tokens.AgentMasterToken())
		require.Equal("hotel", a.tokens.ReplicationToken())
	})

	t.Run("persisted-tokens", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "echo",
			ACLAgentToken:       "foxtrot",
			ACLAgentMasterToken: "golf",
			ACLReplicationToken: "hotel",
		}

		tokens := `{
			"agent" : "india",
			"agent_master" : "juliett",
			"default": "kilo",
			"replication" : "lima"
		}`

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		// no updates since token persistence is not enabled
		require.Equal("echo", a.tokens.UserToken())
		require.Equal("foxtrot", a.tokens.AgentToken())
		require.Equal("golf", a.tokens.AgentMasterToken())
		require.Equal("hotel", a.tokens.ReplicationToken())

		a.config.ACLEnableTokenPersistence = true
		require.NoError(a.loadTokens(cfg))

		require.Equal("india", a.tokens.AgentToken())
		require.Equal("juliett", a.tokens.AgentMasterToken())
		require.Equal("kilo", a.tokens.UserToken())
		require.Equal("lima", a.tokens.ReplicationToken())
	})

	t.Run("persisted-tokens-override", func(t *testing.T) {
		tokens := `{
			"agent" : "mike",
			"agent_master" : "november",
			"default": "oscar",
			"replication" : "papa"
		}`

		cfg := &config.RuntimeConfig{
			ACLToken:            "quebec",
			ACLAgentToken:       "romeo",
			ACLAgentMasterToken: "sierra",
			ACLReplicationToken: "tango",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		require.Equal("mike", a.tokens.AgentToken())
		require.Equal("november", a.tokens.AgentMasterToken())
		require.Equal("oscar", a.tokens.UserToken())
		require.Equal("papa", a.tokens.ReplicationToken())
	})

	t.Run("partial-persisted", func(t *testing.T) {
		tokens := `{
			"agent" : "uniform",
			"agent_master" : "victor"
		}`

		cfg := &config.RuntimeConfig{
			ACLToken:            "whiskey",
			ACLAgentToken:       "xray",
			ACLAgentMasterToken: "yankee",
			ACLReplicationToken: "zulu",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte(tokens), 0600))
		require.NoError(a.loadTokens(cfg))

		require.Equal("uniform", a.tokens.AgentToken())
		require.Equal("victor", a.tokens.AgentMasterToken())
		require.Equal("whiskey", a.tokens.UserToken())
		require.Equal("zulu", a.tokens.ReplicationToken())
	})

	t.Run("persistence-error-not-json", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "one",
			ACLAgentToken:       "two",
			ACLAgentMasterToken: "three",
			ACLReplicationToken: "four",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, 0600))
		err := a.loadTokens(cfg)
		require.Error(err)

		require.Equal("one", a.tokens.UserToken())
		require.Equal("two", a.tokens.AgentToken())
		require.Equal("three", a.tokens.AgentMasterToken())
		require.Equal("four", a.tokens.ReplicationToken())
	})

	t.Run("persistence-error-wrong-top-level", func(t *testing.T) {
		cfg := &config.RuntimeConfig{
			ACLToken:            "alfa",
			ACLAgentToken:       "bravo",
			ACLAgentMasterToken: "charlie",
			ACLReplicationToken: "foxtrot",
		}

		require.NoError(ioutil.WriteFile(tokensFullPath, []byte("[1,2,3]"), 0600))
		err := a.loadTokens(cfg)
		require.Error(err)

		require.Equal("alfa", a.tokens.UserToken())
		require.Equal("bravo", a.tokens.AgentToken())
		require.Equal("charlie", a.tokens.AgentMasterToken())
		require.Equal("foxtrot", a.tokens.ReplicationToken())
	})
}

func TestAgent_ReloadConfigOutgoingRPCConfig(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	defer os.RemoveAll(dataDir)
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, t.Name(), hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.OutgoingRPCConfig()
	require.True(t, tlsConf.InsecureSkipVerify)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)

	hcl = `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_path = "../test/ca_path"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = true
	`
	c := TestConfig(config.Source{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.ReloadConfig(c))
	tlsConf = a.tlsConfigurator.OutgoingRPCConfig()
	require.False(t, tlsConf.InsecureSkipVerify)
	require.Len(t, tlsConf.RootCAs.Subjects(), 2)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
}

func TestAgent_ReloadConfigIncomingRPCConfig(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	defer os.RemoveAll(dataDir)
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, t.Name(), hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.IncomingRPCConfig()
	require.NotNil(t, tlsConf.GetConfigForClient)
	tlsConf, err := tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.NotNil(t, tlsConf)
	require.True(t, tlsConf.InsecureSkipVerify)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)

	hcl = `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_path = "../test/ca_path"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = true
	`
	c := TestConfig(config.Source{Name: t.Name(), Format: "hcl", Data: hcl})
	require.NoError(t, a.ReloadConfig(c))
	tlsConf, err = tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.False(t, tlsConf.InsecureSkipVerify)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 2)
	require.Len(t, tlsConf.RootCAs.Subjects(), 2)
}

func TestAgent_ReloadConfigTLSConfigFailure(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	defer os.RemoveAll(dataDir)
	hcl := `
		data_dir = "` + dataDir + `"
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		verify_server_hostname = false
	`
	a := NewTestAgent(t, t.Name(), hcl)
	defer a.Shutdown()
	tlsConf := a.tlsConfigurator.IncomingRPCConfig()

	hcl = `
		data_dir = "` + dataDir + `"
		verify_incoming = true
	`
	c := TestConfig(config.Source{Name: t.Name(), Format: "hcl", Data: hcl})
	require.Error(t, a.ReloadConfig(c))
	tlsConf, err := tlsConf.GetConfigForClient(nil)
	require.NoError(t, err)
	require.Equal(t, tls.NoClientCert, tlsConf.ClientAuth)
	require.Len(t, tlsConf.ClientCAs.Subjects(), 1)
	require.Len(t, tlsConf.RootCAs.Subjects(), 1)
}

func TestAgent_consulConfig_AutoEncryptAllowTLS(t *testing.T) {
	t.Parallel()
	dataDir := testutil.TempDir(t, "agent") // we manage the data dir
	defer os.RemoveAll(dataDir)
	hcl := `
		data_dir = "` + dataDir + `"
		verify_incoming = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		auto_encrypt { allow_tls = true }
	`
	a := NewTestAgent(t, t.Name(), hcl)
	defer a.Shutdown()
	require.True(t, a.consulConfig().AutoEncryptAllowTLS)
}

func TestAgent_consulConfig_RaftTrailingLogs(t *testing.T) {
	t.Parallel()
	hcl := `
		raft_trailing_logs = 812345
	`
	a := NewTestAgent(t, t.Name(), hcl)
	defer a.Shutdown()
	require.Equal(t, uint64(812345), a.consulConfig().RaftConfig.TrailingLogs)
}
