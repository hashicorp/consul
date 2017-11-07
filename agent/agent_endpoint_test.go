package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"
	"github.com/pascaldekloe/goe/verify"
)

func makeReadOnlyAgentACL(t *testing.T, srv *HTTPServer) string {
	args := map[string]interface{}{
		"Name":  "User Token",
		"Type":  "client",
		"Rules": `agent "" { policy = "read" }`,
	}
	req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := srv.ACLCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	aclResp := obj.(aclCreateResponse)
	return aclResp.ID
}

func TestAgent_Services(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
	}
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[string]*structs.NodeService)
	if len(val) != 1 {
		t.Fatalf("bad services: %v", obj)
	}
	if val["mysql"].Port != 5000 {
		t.Fatalf("bad service: %v", obj)
	}
}

func TestAgent_Services_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
	}
	a.State.AddService(srv1, "")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
		obj, err := a.srv.AgentServices(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[string]*structs.NodeService)
		if len(val) != 0 {
			t.Fatalf("bad: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/services?token=root", nil)
		obj, err := a.srv.AgentServices(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[string]*structs.NodeService)
		if len(val) != 1 {
			t.Fatalf("bad: %v", obj)
		}
	})
}

func TestAgent_Checks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
	obj, err := a.srv.AgentChecks(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[types.CheckID]*structs.HealthCheck)
	if len(val) != 1 {
		t.Fatalf("bad checks: %v", obj)
	}
	if val["mysql"].Status != api.HealthPassing {
		t.Fatalf("bad check: %v", obj)
	}
}

func TestAgent_Checks_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk1, "")

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks", nil)
		obj, err := a.srv.AgentChecks(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[types.CheckID]*structs.HealthCheck)
		if len(val) != 0 {
			t.Fatalf("bad checks: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/checks?token=root", nil)
		obj, err := a.srv.AgentChecks(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.(map[types.CheckID]*structs.HealthCheck)
		if len(val) != 1 {
			t.Fatalf("bad checks: %v", obj)
		}
	})
}

func TestAgent_Self(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		node_meta {
			somekey = "somevalue"
		}
	`)
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
	obj, err := a.srv.AgentSelf(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	val := obj.(Self)
	if int(val.Member.Port) != a.Config.SerfPortLAN {
		t.Fatalf("incorrect port: %v", obj)
	}

	if val.DebugConfig["SerfPortLAN"].(int) != a.Config.SerfPortLAN {
		t.Fatalf("incorrect port: %v", obj)
	}

	cs, err := a.GetLANCoordinate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if c := cs[a.config.SegmentName]; !reflect.DeepEqual(c, val.Coord) {
		t.Fatalf("coordinates are not equal: %v != %v", c, val.Coord)
	}
	delete(val.Meta, structs.MetaSegmentKey) // Added later, not in config.
	if !reflect.DeepEqual(a.config.NodeMeta, val.Meta) {
		t.Fatalf("meta fields are not equal: %v != %v", a.config.NodeMeta, val.Meta)
	}
}

func TestAgent_Self_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
		if _, err := a.srv.AgentSelf(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/self?token=towel", nil)
		if _, err := a.srv.AgentSelf(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/self?token=%s", ro), nil)
		if _, err := a.srv.AgentSelf(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_Metrics_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics", nil)
		if _, err := a.srv.AgentMetrics(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/metrics?token=towel", nil)
		if _, err := a.srv.AgentMetrics(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/agent/metrics?token=%s", ro), nil)
		if _, err := a.srv.AgentMetrics(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_Reload(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		acl_enforce_version_8 = false
		services = [
			{
				name = "redis"
			}
		]
		watches = [
			{
				datacenter = "dc1"
				type = "key"
				key = "test"
				handler = "true"
			}
		]
	`)
	defer a.Shutdown()

	if a.State.Service("redis") == nil {
		t.Fatal("missing redis service")
	}

	cfg2 := TestConfig(config.Source{
		Name:   "reload",
		Format: "hcl",
		Data: `
			data_dir = "` + a.Config.DataDir + `"
			node_id = "` + string(a.Config.NodeID) + `"
			node_name = "` + a.Config.NodeName + `"

			acl_enforce_version_8 = false
			services = [
				{
					name = "redis-reloaded"
				}
			]
		`,
	})

	if err := a.ReloadConfig(cfg2); err != nil {
		t.Fatalf("got error %v want nil", err)
	}
	if a.State.Service("redis-reloaded") == nil {
		t.Fatal("missing redis-reloaded service")
	}

	for _, wp := range a.watchPlans {
		if !wp.IsStopped() {
			t.Fatalf("Reloading configs should stop watch plans of the previous configuration")
		}
	}
}

func TestAgent_Reload_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/reload", nil)
		if _, err := a.srv.AgentReload(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/reload?token=%s", ro), nil)
		if _, err := a.srv.AgentReload(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	// This proves we call the ACL function, and we've got the other reload
	// test to prove we do the reload, which should be sufficient.
	// The reload logic is a little complex to set up so isn't worth
	// repeating again here.
}

func TestAgent_Members(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
	obj, err := a.srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != a.Config.SerfPortLAN {
		t.Fatalf("not lan: %v", obj)
	}
}

func TestAgent_Members_WAN(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/agent/members?wan=true", nil)
	obj, err := a.srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != a.Config.SerfPortWAN {
		t.Fatalf("not wan: %v", obj)
	}
}

func TestAgent_Members_ACLFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members", nil)
		obj, err := a.srv.AgentMembers(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.([]serf.Member)
		if len(val) != 0 {
			t.Fatalf("bad members: %v", obj)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/agent/members?token=root", nil)
		obj, err := a.srv.AgentMembers(nil, req)
		if err != nil {
			t.Fatalf("Err: %v", err)
		}
		val := obj.([]serf.Member)
		if len(val) != 1 {
			t.Fatalf("bad members: %v", obj)
		}
	})
}

func TestAgent_Join(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t.Name(), "")
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
	obj, err := a1.srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	if len(a1.LANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a2.LANMembers()), 2; got != want {
			r.Fatalf("got %d LAN members want %d", got, want)
		}
	})
}

func TestAgent_Join_WAN(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t.Name(), "")
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortWAN)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?wan=true", addr), nil)
	obj, err := a1.srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	if len(a1.WANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a2.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})
}

func TestAgent_Join_ACLDeny(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), TestACLConfig())
	defer a1.Shutdown()
	a2 := NewTestAgent(t.Name(), "")
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
		if _, err := a1.srv.AgentJoin(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?token=towel", addr), nil)
		_, err := a1.srv.AgentJoin(nil, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a1.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/join/%s?token=%s", addr, ro), nil)
		if _, err := a1.srv.AgentJoin(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})
}

type mockNotifier struct{ s string }

func (n *mockNotifier) Notify(state string) error {
	n.s = state
	return nil
}

func TestAgent_JoinLANNotify(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), "")
	defer a1.Shutdown()

	a2 := NewTestAgent(t.Name(), `
		server = false
		bootstrap = false
	`)
	defer a2.Shutdown()

	notif := &mockNotifier{}
	a1.joinLANNotifier = notif

	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if got, want := notif.s, "READY=1"; got != want {
		t.Fatalf("got joinLAN notification %q want %q", got, want)
	}
}

func TestAgent_Leave(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), "")
	defer a1.Shutdown()

	a2 := NewTestAgent(t.Name(), `
 		server = false
 		bootstrap = false
 	`)
	defer a2.Shutdown()

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Graceful leave now
	req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
	obj, err := a2.srv.AgentLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})
}

func TestAgent_Leave_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave", nil)
		if _, err := a.srv.AgentLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/leave?token=%s", ro), nil)
		if _, err := a.srv.AgentLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	// this sub-test will change the state so that there is no leader.
	// it must therefore be the last one in this list.
	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/leave?token=towel", nil)
		if _, err := a.srv.AgentLeave(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_ForceLeave(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), "")
	defer a1.Shutdown()
	a2 := NewTestAgent(t.Name(), "")

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.SerfPortLAN)
	_, err := a1.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// this test probably needs work
	a2.Shutdown()

	// Force leave now
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/%s", a2.Config.NodeName), nil)
	obj, err := a1.srv.AgentForceLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}
	retry.Run(t, func(r *retry.R) {
		m := a1.LANMembers()
		if got, want := m[1].Status, serf.StatusLeft; got != want {
			r.Fatalf("got status %q want %q", got, want)
		}
	})

}

func TestAgent_ForceLeave_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/force-leave/nope", nil)
		if _, err := a.srv.AgentForceLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("agent master token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/force-leave/nope?token=towel", nil)
		if _, err := a.srv.AgentForceLeave(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("read-only token", func(t *testing.T) {
		ro := makeReadOnlyAgentACL(t, a.srv)
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/v1/agent/force-leave/nope?token=%s", ro), nil)
		if _, err := a.srv.AgentForceLeave(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.CheckDefinition{
		Name: "test",
		TTL:  15 * time.Second,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token=abc123", jsonReader(args))
	obj, err := a.srv.AgentRegisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	checkID := types.CheckID("test")
	if _, ok := a.State.Checks()[checkID]; !ok {
		t.Fatalf("missing test check")
	}

	if _, ok := a.checkTTLs[checkID]; !ok {
		t.Fatalf("missing test check ttl")
	}

	// Ensure the token was configured
	if token := a.State.CheckToken(checkID); token == "" {
		t.Fatalf("missing token")
	}

	// By default, checks start in critical state.
	state := a.State.Checks()[checkID]
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

// This verifies all the forms of the new args-style check that we need to
// support as a result of https://github.com/hashicorp/consul/issues/3587.
func TestAgent_RegisterCheck_Scripts(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), `
		enable_script_checks = true
`)
	defer a.Shutdown()

	tests := []struct {
		name  string
		check map[string]interface{}
	}{
		{
			"< Consul 1.0.0",
			map[string]interface{}{
				"Name":     "test",
				"Interval": "2s",
				"Script":   "true",
			},
		},
		{
			"== Consul 1.0.0",
			map[string]interface{}{
				"Name":       "test",
				"Interval":   "2s",
				"ScriptArgs": []string{"true"},
			},
		},
		{
			"> Consul 1.0.0 (fixup)",
			map[string]interface{}{
				"Name":        "test",
				"Interval":    "2s",
				"script_args": []string{"true"},
			},
		},
		{
			"> Consul 1.0.0",
			map[string]interface{}{
				"Name":     "test",
				"Interval": "2s",
				"Args":     []string{"true"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+" as node check", func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(tt.check))
			resp := httptest.NewRecorder()
			if _, err := a.srv.AgentRegisterCheck(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})

		t.Run(tt.name+" as top-level service check", func(t *testing.T) {
			args := map[string]interface{}{
				"Name":  "a",
				"Port":  1234,
				"Check": tt.check,
			}

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			resp := httptest.NewRecorder()
			if _, err := a.srv.AgentRegisterService(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})

		t.Run(tt.name+" as slice-based service check", func(t *testing.T) {
			args := map[string]interface{}{
				"Name":   "a",
				"Port":   1234,
				"Checks": []map[string]interface{}{tt.check},
			}

			req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
			resp := httptest.NewRecorder()
			if _, err := a.srv.AgentRegisterService(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
			if resp.Code != http.StatusOK {
				t.Fatalf("bad: %d", resp.Code)
			}
		})
	}
}

func TestAgent_RegisterCheck_Passing(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.CheckDefinition{
		Name:   "test",
		TTL:    15 * time.Second,
		Status: api.HealthPassing,
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	obj, err := a.srv.AgentRegisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	checkID := types.CheckID("test")
	if _, ok := a.State.Checks()[checkID]; !ok {
		t.Fatalf("missing test check")
	}

	if _, ok := a.checkTTLs[checkID]; !ok {
		t.Fatalf("missing test check ttl")
	}

	state := a.State.Checks()[checkID]
	if state.Status != api.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_RegisterCheck_BadStatus(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.CheckDefinition{
		Name:   "test",
		TTL:    15 * time.Second,
		Status: "fluffy",
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentRegisterCheck(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("accepted bad status")
	}
}

func TestAgent_RegisterCheck_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	args := &structs.CheckDefinition{
		Name: "test",
		TTL:  15 * time.Second,
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(args))
		if _, err := a.srv.AgentRegisterCheck(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/register?token=root", jsonReader(args))
		if _, err := a.srv.AgentRegisterCheck(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_DeregisterCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := a.AddCheck(chk, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
	obj, err := a.srv.AgentDeregisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	if _, ok := a.State.Checks()["test"]; ok {
		t.Fatalf("have test check")
	}
}

func TestAgent_DeregisterCheckACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := a.AddCheck(chk, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test", nil)
		if _, err := a.srv.AgentDeregisterCheck(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/deregister/test?token=root", nil)
		if _, err := a.srv.AgentDeregisterCheck(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_PassCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test", nil)
	obj, err := a.srv.AgentCheckPass(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Checks()["test"]
	if state.Status != api.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_PassCheck_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test", nil)
		if _, err := a.srv.AgentCheckPass(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/pass/test?token=root", nil)
		if _, err := a.srv.AgentCheckPass(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_WarnCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test", nil)
	obj, err := a.srv.AgentCheckWarn(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Checks()["test"]
	if state.Status != api.HealthWarning {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_WarnCheck_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test", nil)
		if _, err := a.srv.AgentCheckWarn(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/warn/test?token=root", nil)
		if _, err := a.srv.AgentCheckWarn(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_FailCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test", nil)
	obj, err := a.srv.AgentCheckFail(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := a.State.Checks()["test"]
	if state.Status != api.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

func TestAgent_FailCheck_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test", nil)
		if _, err := a.srv.AgentCheckFail(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/check/fail/test?token=root", nil)
		if _, err := a.srv.AgentCheckFail(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_UpdateCheck(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	cases := []checkUpdate{
		checkUpdate{api.HealthPassing, "hello-passing"},
		checkUpdate{api.HealthCritical, "hello-critical"},
		checkUpdate{api.HealthWarning, "hello-warning"},
	}

	for _, c := range cases {
		t.Run(c.Status, func(t *testing.T) {
			req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(c))
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentCheckUpdate(resp, req)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if obj != nil {
				t.Fatalf("bad: %v", obj)
			}
			if resp.Code != 200 {
				t.Fatalf("expected 200, got %d", resp.Code)
			}

			state := a.State.Checks()["test"]
			if state.Status != c.Status || state.Output != c.Output {
				t.Fatalf("bad: %v", state)
			}
		})
	}

	t.Run("log output limit", func(t *testing.T) {
		args := checkUpdate{
			Status: api.HealthPassing,
			Output: strings.Repeat("-= bad -=", 5*checks.BufSize),
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.AgentCheckUpdate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if obj != nil {
			t.Fatalf("bad: %v", obj)
		}
		if resp.Code != 200 {
			t.Fatalf("expected 200, got %d", resp.Code)
		}

		// Since we append some notes about truncating, we just do a
		// rough check that the output buffer was cut down so this test
		// isn't super brittle.
		state := a.State.Checks()["test"]
		if state.Status != api.HealthPassing || len(state.Output) > 2*checks.BufSize {
			t.Fatalf("bad: %v", state)
		}
	})

	t.Run("bogus status", func(t *testing.T) {
		args := checkUpdate{Status: "itscomplicated"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.AgentCheckUpdate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if obj != nil {
			t.Fatalf("bad: %v", obj)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})
}

func TestAgent_UpdateCheck_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &structs.CheckType{TTL: 15 * time.Second}
	if err := a.AddCheck(chk, chkType, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		args := checkUpdate{api.HealthPassing, "hello-passing"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test", jsonReader(args))
		if _, err := a.srv.AgentCheckUpdate(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		args := checkUpdate{api.HealthPassing, "hello-passing"}
		req, _ := http.NewRequest("PUT", "/v1/agent/check/update/test?token=root", jsonReader(args))
		if _, err := a.srv.AgentCheckUpdate(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ServiceDefinition{
		Name: "test",
		Tags: []string{"master"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			&structs.CheckType{
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
				TTL: 30 * time.Second,
			},
		},
	}
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))

	obj, err := a.srv.AgentRegisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure the servie
	if _, ok := a.State.Services()["test"]; !ok {
		t.Fatalf("missing test service")
	}

	// Ensure we have a check mapping
	checks := a.State.Checks()
	if len(checks) != 3 {
		t.Fatalf("bad: %v", checks)
	}

	if len(a.checkTTLs) != 3 {
		t.Fatalf("missing test check ttls: %v", a.checkTTLs)
	}

	// Ensure the token was configured
	if token := a.State.ServiceToken("test"); token == "" {
		t.Fatalf("missing token")
	}
}

func TestAgent_RegisterService_TranslateKeys(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	json := `{"name":"test", "port":8000, "enable_tag_override": true}`
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", strings.NewReader(json))

	obj, err := a.srv.AgentRegisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	svc := &structs.NodeService{
		ID:                "test",
		Service:           "test",
		Port:              8000,
		EnableTagOverride: true,
	}

	if got, want := a.State.Service("test"), svc; !verify.Values(t, "", got, want) {
		t.Fail()
	}
}

func TestAgent_RegisterService_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	args := &structs.ServiceDefinition{
		Name: "test",
		Tags: []string{"master"},
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Checks: []*structs.CheckType{
			&structs.CheckType{
				TTL: 20 * time.Second,
			},
			&structs.CheckType{
				TTL: 30 * time.Second,
			},
		},
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		if _, err := a.srv.AgentRegisterService(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(args))
		if _, err := a.srv.AgentRegisterService(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterService_InvalidAddress(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	for _, addr := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run("addr "+addr, func(t *testing.T) {
			args := &structs.ServiceDefinition{
				Name:    "test",
				Address: addr,
				Port:    8000,
			}
			req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
			resp := httptest.NewRecorder()
			_, err := a.srv.AgentRegisterService(resp, req)
			if err != nil {
				t.Fatalf("got error %v want nil", err)
			}
			if got, want := resp.Code, 400; got != want {
				t.Fatalf("got code %d want %d", got, want)
			}
			if got, want := resp.Body.String(), "Invalid service address"; got != want {
				t.Fatalf("got body %q want %q", got, want)
			}
		})
	}
}

func TestAgent_DeregisterService(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
	obj, err := a.srv.AgentDeregisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	if _, ok := a.State.Services()["test"]; ok {
		t.Fatalf("have test service")
	}

	if _, ok := a.State.Checks()["test"]; ok {
		t.Fatalf("have test check")
	}
}

func TestAgent_DeregisterService_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test", nil)
		if _, err := a.srv.AgentDeregisterService(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test?token=root", nil)
		if _, err := a.srv.AgentDeregisterService(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_ServiceMaintenance_BadRequest(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	t.Run("not enabled", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("no service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/?enable=true", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 400 {
			t.Fatalf("expected 400, got %d", resp.Code)
		}
	})

	t.Run("bad service id", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/_nope_?enable=true", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}
		if resp.Code != 404 {
			t.Fatalf("expected 404, got %d", resp.Code)
		}
	})
}

func TestAgent_ServiceMaintenance_Enable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the service into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was registered
	checkID := serviceMaintCheckID("test")
	check, ok := a.State.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered maintenance check")
	}

	// Ensure the token was added
	if token := a.State.CheckToken(checkID); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_ServiceMaintenance_Disable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the service into maintenance mode
	if err := a.EnableServiceMaintenance("test", "", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=false", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentServiceMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was removed
	checkID := serviceMaintCheckID("test")
	if _, ok := a.State.Checks()[checkID]; ok {
		t.Fatalf("should have removed maintenance check")
	}
}

func TestAgent_ServiceMaintenance_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	// Register the service.
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a.AddService(service, nil, false, ""); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken", nil)
		if _, err := a.srv.AgentServiceMaintenance(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/service/maintenance/test?enable=true&reason=broken&token=root", nil)
		if _, err := a.srv.AgentServiceMaintenance(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_NodeMaintenance_BadRequest(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Fails when no enable flag provided
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 400 {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestAgent_NodeMaintenance_Enable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Force the node into maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken&token=mytoken", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was registered
	check, ok := a.State.Checks()[structs.NodeMaint]
	if !ok {
		t.Fatalf("should have registered maintenance check")
	}

	// Check that the token was used
	if token := a.State.CheckToken(structs.NodeMaint); token != "mytoken" {
		t.Fatalf("expected 'mytoken', got '%s'", token)
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_NodeMaintenance_Disable(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Force the node into maintenance mode
	a.EnableNodeMaintenance("", "")

	// Leave maintenance mode
	req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=false", nil)
	resp := httptest.NewRecorder()
	if _, err := a.srv.AgentNodeMaintenance(resp, req); err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	// Ensure the maintenance check was removed
	if _, ok := a.State.Checks()[structs.NodeMaint]; ok {
		t.Fatalf("should have removed maintenance check")
	}
}

func TestAgent_NodeMaintenance_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	t.Run("no token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken", nil)
		if _, err := a.srv.AgentNodeMaintenance(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
	})

	t.Run("root token", func(t *testing.T) {
		req, _ := http.NewRequest("PUT", "/v1/agent/self/maintenance?enable=true&reason=broken&token=root", nil)
		if _, err := a.srv.AgentNodeMaintenance(nil, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestAgent_RegisterCheck_Service(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ServiceDefinition{
		Name: "memcache",
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
	}

	// First register the service
	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	if _, err := a.srv.AgentRegisterService(nil, req); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now register an additional check
	checkArgs := &structs.CheckDefinition{
		Name:      "memcache_check2",
		ServiceID: "memcache",
		TTL:       15 * time.Second,
	}
	req, _ = http.NewRequest("PUT", "/v1/agent/check/register", jsonReader(checkArgs))
	if _, err := a.srv.AgentRegisterCheck(nil, req); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	result := a.State.Checks()
	if _, ok := result["service:memcache"]; !ok {
		t.Fatalf("missing memcached check")
	}
	if _, ok := result["memcache_check2"]; !ok {
		t.Fatalf("missing memcache_check2 check")
	}

	// Make sure the new check is associated with the service
	if result["memcache_check2"].ServiceID != "memcache" {
		t.Fatalf("bad: %#v", result["memcached_check2"])
	}
}

func TestAgent_Monitor(t *testing.T) {
	t.Parallel()
	logWriter := logger.NewLogWriter(512)
	a := &TestAgent{
		Name:      t.Name(),
		LogWriter: logWriter,
		LogOutput: io.MultiWriter(os.Stderr, logWriter),
	}
	a.Start()
	defer a.Shutdown()

	// Try passing an invalid log level
	req, _ := http.NewRequest("GET", "/v1/agent/monitor?loglevel=invalid", nil)
	resp := newClosableRecorder()
	if _, err := a.srv.AgentMonitor(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 400 {
		t.Fatalf("bad: %v", resp.Code)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Unknown log level") {
		t.Fatalf("bad: %s", body)
	}

	// Try to stream logs until we see the expected log line
	retry.Run(t, func(r *retry.R) {
		req, _ = http.NewRequest("GET", "/v1/agent/monitor?loglevel=debug", nil)
		resp = newClosableRecorder()
		done := make(chan struct{})
		go func() {
			if _, err := a.srv.AgentMonitor(resp, req); err != nil {
				t.Fatalf("err: %s", err)
			}
			close(done)
		}()

		resp.Close()
		<-done

		got := resp.Body.Bytes()
		want := []byte("raft: Initial configuration (index=1)")
		if !bytes.Contains(got, want) {
			r.Fatalf("got %q and did not find %q", got, want)
		}
	})
}

type closableRecorder struct {
	*httptest.ResponseRecorder
	closer chan bool
}

func newClosableRecorder() *closableRecorder {
	r := httptest.NewRecorder()
	closer := make(chan bool)
	return &closableRecorder{r, closer}
}

func (r *closableRecorder) Close() {
	close(r.closer)
}

func (r *closableRecorder) CloseNotify() <-chan bool {
	return r.closer
}

func TestAgent_Monitor_ACLDeny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	// Try without a token.
	req, _ := http.NewRequest("GET", "/v1/agent/monitor", nil)
	if _, err := a.srv.AgentMonitor(nil, req); !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// This proves we call the ACL function, and we've got the other monitor
	// test to prove monitor works, which should be sufficient. The monitor
	// logic is a little complex to set up so isn't worth repeating again
	// here.
}

func TestAgent_Token(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_token = ""
		acl_agent_token = ""
		acl_agent_master_token = ""
	`)
	defer a.Shutdown()

	type tokens struct {
		user, agent, master, repl string
	}

	resetTokens := func(got tokens) {
		a.tokens.UpdateUserToken(got.user)
		a.tokens.UpdateAgentToken(got.agent)
		a.tokens.UpdateAgentMasterToken(got.master)
		a.tokens.UpdateACLReplicationToken(got.repl)
	}

	body := func(token string) io.Reader {
		return jsonReader(&api.AgentToken{Token: token})
	}

	badJSON := func() io.Reader {
		return jsonReader(false)
	}

	tests := []struct {
		name        string
		method, url string
		body        io.Reader
		code        int
		got, want   tokens
	}{
		{
			name:   "bad token name",
			method: "PUT",
			url:    "nope?token=root",
			body:   body("X"),
			code:   http.StatusNotFound,
		},
		{
			name:   "bad JSON",
			method: "PUT",
			url:    "acl_token?token=root",
			body:   badJSON(),
			code:   http.StatusBadRequest,
		},
		{
			name:   "set user",
			method: "PUT",
			url:    "acl_token?token=root",
			body:   body("U"),
			code:   http.StatusOK,
			want:   tokens{user: "U", agent: "U"},
		},
		{
			name:   "set agent",
			method: "PUT",
			url:    "acl_agent_token?token=root",
			body:   body("A"),
			code:   http.StatusOK,
			got:    tokens{user: "U", agent: "U"},
			want:   tokens{user: "U", agent: "A"},
		},
		{
			name:   "set master",
			method: "PUT",
			url:    "acl_agent_master_token?token=root",
			body:   body("M"),
			code:   http.StatusOK,
			want:   tokens{master: "M"},
		},
		{
			name:   "set repl",
			method: "PUT",
			url:    "acl_replication_token?token=root",
			body:   body("R"),
			code:   http.StatusOK,
			want:   tokens{repl: "R"},
		},
		{
			name:   "clear user",
			method: "PUT",
			url:    "acl_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			got:    tokens{user: "U"},
		},
		{
			name:   "clear agent",
			method: "PUT",
			url:    "acl_agent_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			got:    tokens{agent: "A"},
		},
		{
			name:   "clear master",
			method: "PUT",
			url:    "acl_agent_master_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			got:    tokens{master: "M"},
		},
		{
			name:   "clear repl",
			method: "PUT",
			url:    "acl_replication_token?token=root",
			body:   body(""),
			code:   http.StatusOK,
			got:    tokens{repl: "R"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTokens(tt.got)
			url := fmt.Sprintf("/v1/agent/token/%s", tt.url)
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, url, tt.body)
			if _, err := a.srv.AgentToken(resp, req); err != nil {
				t.Fatalf("err: %v", err)
			}
			if got, want := resp.Code, tt.code; got != want {
				t.Fatalf("got %d want %d", got, want)
			}
			if got, want := a.tokens.UserToken(), tt.want.user; got != want {
				t.Fatalf("got %q want %q", got, want)
			}
			if got, want := a.tokens.AgentToken(), tt.want.agent; got != want {
				t.Fatalf("got %q want %q", got, want)
			}
			if tt.want.master != "" && !a.tokens.IsAgentMasterToken(tt.want.master) {
				t.Fatalf("%q should be the master token", tt.want.master)
			}
			if got, want := a.tokens.ACLReplicationToken(), tt.want.repl; got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}

	// This one returns an error that is interpreted by the HTTP wrapper, so
	// doesn't fit into our table above.
	t.Run("permission denied", func(t *testing.T) {
		resetTokens(tokens{})
		req, _ := http.NewRequest("PUT", "/v1/agent/token/acl_token", body("X"))
		if _, err := a.srv.AgentToken(nil, req); !acl.IsErrPermissionDenied(err) {
			t.Fatalf("err: %v", err)
		}
		if got, want := a.tokens.UserToken(), ""; got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}
