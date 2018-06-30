package agent

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/copystructure"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Meta: map[string]string{
			"foo": "bar",
		},
		Port: 5000,
	}
	require.NoError(t, a.State.AddService(srv1, ""))

	// Add a managed proxy for that service
	prxy1 := &structs.ConnectManagedProxy{
		ExecMode: structs.ProxyExecModeScript,
		Command:  []string{"proxy.sh"},
		Config: map[string]interface{}{
			"bind_port": 1234,
			"foo":       "bar",
		},
		TargetServiceID: "mysql",
	}
	_, err := a.State.AddProxy(prxy1, "", "")
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[string]*api.AgentService)
	assert.Lenf(t, val, 1, "bad services: %v", obj)
	assert.Equal(t, 5000, val["mysql"].Port)
	assert.Equal(t, srv1.Meta, val["mysql"].Meta)
	assert.NotNil(t, val["mysql"].Connect)
	assert.NotNil(t, val["mysql"].Connect.Proxy)
	assert.Equal(t, prxy1.ExecMode.String(), string(val["mysql"].Connect.Proxy.ExecMode))
	assert.Equal(t, prxy1.Command, val["mysql"].Connect.Proxy.Command)
	assert.Equal(t, prxy1.Config, val["mysql"].Connect.Proxy.Config)
}

// This tests that the agent services endpoint (/v1/agent/services) returns
// Connect proxies.
func TestAgent_Services_ExternalConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		Kind:             structs.ServiceKindConnectProxy,
		ID:               "db-proxy",
		Service:          "db-proxy",
		Port:             5000,
		ProxyDestination: "db",
	}
	a.State.AddService(srv1, "")

	req, _ := http.NewRequest("GET", "/v1/agent/services", nil)
	obj, err := a.srv.AgentServices(nil, req)
	assert.Nil(err)
	val := obj.(map[string]*api.AgentService)
	assert.Len(val, 1)
	actual := val["db-proxy"]
	assert.Equal(api.ServiceKindConnectProxy, actual.Kind)
	assert.Equal("db", actual.ProxyDestination)
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
		val := obj.(map[string]*api.AgentService)
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
		val := obj.(map[string]*api.AgentService)
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
    limits = {
      rpc_rate=1
      rpc_max_burst=100
    }
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
      limits = {
        rpc_rate=2
        rpc_max_burst=200
      }
		`,
	})

	if err := a.ReloadConfig(cfg2); err != nil {
		t.Fatalf("got error %v want nil", err)
	}
	if a.State.Service("redis-reloaded") == nil {
		t.Fatal("missing redis-reloaded service")
	}

	if a.config.RPCRateLimit != 2 {
		t.Fatalf("RPC rate not set correctly.  Got %v. Want 2", a.config.RPCRateLimit)
	}

	if a.config.RPCMaxBurst != 200 {
		t.Fatalf("RPC max burst not set correctly.  Got %v. Want 200", a.config.RPCMaxBurst)
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
		Meta: map[string]string{"hello": "world"},
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

	// Ensure the service
	if _, ok := a.State.Services()["test"]; !ok {
		t.Fatalf("missing test service")
	}
	if val := a.State.Service("test").Meta["hello"]; val != "world" {
		t.Fatalf("Missing meta: %v", a.State.Service("test").Meta)
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

	json := `{"name":"test", "port":8000, "enable_tag_override": true, "meta": {"some": "meta"}}`
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
		Meta:              map[string]string{"some": "meta"},
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

// This tests local agent service registration with a managed proxy.
func TestAgent_RegisterService_ManagedConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t.Name(), `
		connect {
			proxy {
				allow_managed_api_registration = true
			}
		}
	`)
	defer a.Shutdown()

	// Register a proxy. Note that the destination doesn't exist here on
	// this agent or in the catalog at all. This is intended and part
	// of the design.
	args := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8000,
		Connect: &api.AgentServiceConnect{
			Proxy: &api.AgentServiceConnectProxy{
				ExecMode: "script",
				Command:  []string{"proxy.sh"},
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.NoError(err)
	assert.Nil(obj)
	require.Equal(200, resp.Code, "request failed with body: %s",
		resp.Body.String())

	// Ensure the target service
	_, ok := a.State.Services()["web"]
	assert.True(ok, "has service")

	// Ensure the proxy service was registered
	proxySvc, ok := a.State.Services()["web-proxy"]
	require.True(ok, "has proxy service")
	assert.Equal(structs.ServiceKindConnectProxy, proxySvc.Kind)
	assert.Equal("web", proxySvc.ProxyDestination)
	assert.NotEmpty(proxySvc.Port, "a port should have been assigned")

	// Ensure proxy itself was registered
	proxy := a.State.Proxy("web-proxy")
	require.NotNil(proxy)
	assert.Equal(structs.ProxyExecModeScript, proxy.Proxy.ExecMode)
	assert.Equal([]string{"proxy.sh"}, proxy.Proxy.Command)
	assert.Equal(args.Connect.Proxy.Config, proxy.Proxy.Config)

	// Ensure the token was configured
	assert.Equal("abc123", a.State.ServiceToken("web"))
	assert.Equal("abc123", a.State.ServiceToken("web-proxy"))
}

// This tests local agent service registration with a managed proxy with
// API registration disabled (default).
func TestAgent_RegisterService_ManagedConnectProxy_Disabled(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	// Register a proxy. Note that the destination doesn't exist here on
	// this agent or in the catalog at all. This is intended and part
	// of the design.
	args := &api.AgentServiceRegistration{
		Name: "web",
		Port: 8000,
		Connect: &api.AgentServiceConnect{
			Proxy: &api.AgentServiceConnectProxy{
				ExecMode: "script",
				Command:  []string{"proxy.sh"},
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentRegisterService(resp, req)
	assert.Error(err)

	// Ensure the target service does not exist
	_, ok := a.State.Services()["web"]
	assert.False(ok, "does not has service")
}

// This tests local agent service registration of a unmanaged connect proxy.
// This verifies that it is put in the local state store properly for syncing
// later. Note that _managed_ connect proxies are registered as part of the
// target service's registration.
func TestAgent_RegisterService_UnmanagedConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register a proxy. Note that the destination doesn't exist here on
	// this agent or in the catalog at all. This is intended and part
	// of the design.
	args := &structs.ServiceDefinition{
		Kind:             structs.ServiceKindConnectProxy,
		Name:             "connect-proxy",
		Port:             8000,
		ProxyDestination: "db",
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.Nil(err)
	assert.Nil(obj)

	// Ensure the service
	svc, ok := a.State.Services()["connect-proxy"]
	assert.True(ok, "has service")
	assert.Equal(structs.ServiceKindConnectProxy, svc.Kind)
	assert.Equal("db", svc.ProxyDestination)

	// Ensure the token was configured
	assert.Equal("abc123", a.State.ServiceToken("connect-proxy"))
}

// This tests that connect proxy validation is done for local agent
// registration. This doesn't need to test validation exhaustively since
// that is done via a table test in the structs package.
func TestAgent_RegisterService_UnmanagedConnectProxyInvalid(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ServiceDefinition{
		Kind:             structs.ServiceKindConnectProxy,
		Name:             "connect-proxy",
		ProxyDestination: "db",
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=abc123", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.Nil(err)
	assert.Nil(obj)
	assert.Equal(http.StatusBadRequest, resp.Code)
	assert.Contains(resp.Body.String(), "Port")

	// Ensure the service doesn't exist
	_, ok := a.State.Services()["connect-proxy"]
	assert.False(ok)
}

// Tests agent registration of a service that is connect native.
func TestAgent_RegisterService_ConnectNative(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register a proxy. Note that the destination doesn't exist here on
	// this agent or in the catalog at all. This is intended and part
	// of the design.
	args := &structs.ServiceDefinition{
		Name: "web",
		Port: 8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Connect: &structs.ServiceConnect{
			Native: true,
		},
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentRegisterService(resp, req)
	assert.Nil(err)
	assert.Nil(obj)

	// Ensure the service
	svc, ok := a.State.Services()["web"]
	assert.True(ok, "has service")
	assert.True(svc.Connect.Native)
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

func TestAgent_DeregisterService_withManagedProxy(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	a := NewTestAgent(t.Name(), `
		connect {
			proxy {
				allow_managed_api_registration = true
			}
		}
		`)

	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy ID
	require.Len(a.State.Proxies(), 1)
	var proxyID string
	for _, p := range a.State.Proxies() {
		proxyID = p.Proxy.ProxyService.ID
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/test-id", nil)
	obj, err := a.srv.AgentDeregisterService(nil, req)
	require.NoError(err)
	require.Nil(obj)

	// Ensure we have no service, check, managed proxy, or proxy service
	require.NotContains(a.State.Services(), "test-id")
	require.NotContains(a.State.Checks(), "test-id")
	require.NotContains(a.State.Services(), proxyID)
	require.Len(a.State.Proxies(), 0)
}

// Test that we can't deregister a managed proxy service directly.
func TestAgent_DeregisterService_managedProxyDirect(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	a := NewTestAgent(t.Name(), `
		connect {
			proxy {
				allow_managed_api_registration = true
			}
		}
		`)

	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy ID
	var proxyID string
	for _, p := range a.State.Proxies() {
		proxyID = p.Proxy.ProxyService.ID
	}

	req, _ := http.NewRequest("PUT", "/v1/agent/service/deregister/"+proxyID, nil)
	obj, err := a.srv.AgentDeregisterService(nil, req)
	require.Error(err)
	require.Nil(obj)
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

	// The behavior of this handler when ACLs are disabled is vetted over
	// in TestACL_Disabled_Response since there's already good infra set
	// up over there to test this, and it calls the common function.
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

func TestAgentConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t.Name(), "connect { enabled = false }")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCARoots(resp, req)
	require.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, "")
	assert.Len(value.Roots, 0)
}

func TestAgentConnectCARoots_list(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Set some CAs. Note that NewTestAgent already bootstraps one CA so this just
	// adds a second and makes it active.
	ca2 := connect.TestCAConfigSet(t, a, nil)

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/roots", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCARoots(resp, req)
	require.NoError(err)

	value := obj.(structs.IndexedCARoots)
	assert.Equal(value.ActiveRootID, ca2.ID)
	// Would like to assert that it's the same as the TestAgent domain but the
	// only way to access that state via this package is by RPC to the server
	// implementation running in TestAgent which is more or less a tautology.
	assert.NotEmpty(value.TrustDomain)
	assert.Len(value.Roots, 2)

	// We should never have the secret information
	for _, r := range value.Roots {
		assert.Equal("", r.SigningCert)
		assert.Equal("", r.SigningKey)
	}

	assert.Equal("MISS", resp.Header().Get("X-Cache"))

	// Test caching
	{
		// List it again
		resp2 := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCARoots(resp2, req)
		require.NoError(err)
		assert.Equal(obj, obj2)

		// Should cache hit this time and not make request
		assert.Equal("HIT", resp2.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			// List it again
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentConnectCARoots(resp, req)
			r.Check(err)

			value := obj.(structs.IndexedCARoots)
			if ca.ID != value.ActiveRootID {
				r.Fatalf("%s != %s", ca.ID, value.ActiveRootID)
			}
			// There are now 3 CAs because we didn't complete rotation on the original
			// 2
			if len(value.Roots) != 3 {
				r.Fatalf("bad len: %d", len(value.Roots))
			}

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

func TestAgentConnectCALeafCert_aclDefaultDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.Error(err)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectCALeafCert_aclProxyToken(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy token from the agent directly, since there is no API.
	proxy := a.State.Proxy("test-id-proxy")
	require.NotNil(proxy)
	token := proxy.ProxyToken
	require.NotEmpty(token)

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)

	// Get the issued cert
	_, ok := obj.(*structs.IssuedCert)
	require.True(ok)
}

func TestAgentConnectCALeafCert_aclProxyTokenOther(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Register another service
	{
		reg := &structs.ServiceDefinition{
			ID:      "wrong-id",
			Name:    "wrong",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy token from the agent directly, since there is no API.
	proxy := a.State.Proxy("wrong-id-proxy")
	require.NotNil(proxy)
	token := proxy.ProxyToken
	require.NotEmpty(token)

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.Error(err)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectCALeafCert_aclServiceWrite(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:write for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "write" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)

	// Get the issued cert
	_, ok := obj.(*structs.IssuedCert)
	require.True(ok)
}

func TestAgentConnectCALeafCert_aclServiceReadDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:read for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "read" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test?token="+token, nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.Error(err)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectCALeafCert_good(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// CA already setup by default by NewTestAgent but force a new one so we can
	// verify it was signed easily.
	ca1 := connect.TestCAConfigSet(t, a, nil)

	{
		// Register a local service
		args := &structs.ServiceDefinition{
			ID:      "foo",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(args))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		if !assert.Equal(200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	issued, ok := obj.(*structs.IssuedCert)
	assert.True(ok)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(issued.ModifyIndex > 0)
	assert.Equal(fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(err)
		require.Equal(obj, obj2)

		// Should cache hit this time and not make request
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			// Try and sign again (note no index/wait arg since cache should update in
			// background even if we aren't actively blocking)
			obj, err := a.srv.AgentConnectCALeafCert(resp, req)
			r.Check(err)

			issued2 := obj.(*structs.IssuedCert)
			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as differnt
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(t, issued2, ca)

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

// Test we can request a leaf cert for a service we have permission for
// but is not local to this agent.
func TestAgentConnectCALeafCert_goodNotLocal(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// CA already setup by default by NewTestAgent but force a new one so we can
	// verify it was signed easily.
	ca1 := connect.TestCAConfigSet(t, a, nil)

	{
		// Register a non-local service (central catalog)
		args := &structs.RegisterRequest{
			Node:    "foo",
			Address: "127.0.0.1",
			Service: &structs.NodeService{
				Service: "test",
				Address: "127.0.0.1",
				Port:    8080,
			},
		}
		req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))
		resp := httptest.NewRecorder()
		_, err := a.srv.CatalogRegister(resp, req)
		require.NoError(err)
		if !assert.Equal(200, resp.Code) {
			t.Log("Body: ", resp.Body.String())
		}
	}

	// List
	req, _ := http.NewRequest("GET", "/v1/agent/connect/ca/leaf/test", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectCALeafCert(resp, req)
	require.NoError(err)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	// Get the issued cert
	issued, ok := obj.(*structs.IssuedCert)
	assert.True(ok)

	// Verify that the cert is signed by the CA
	requireLeafValidUnderCA(t, issued, ca1)

	// Verify blocking index
	assert.True(issued.ModifyIndex > 0)
	assert.Equal(fmt.Sprintf("%d", issued.ModifyIndex),
		resp.Header().Get("X-Consul-Index"))

	// Test caching
	{
		// Fetch it again
		resp := httptest.NewRecorder()
		obj2, err := a.srv.AgentConnectCALeafCert(resp, req)
		require.NoError(err)
		require.Equal(obj, obj2)

		// Should cache hit this time and not make request
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Test that caching is updated in the background
	{
		// Set a new CA
		ca := connect.TestCAConfigSet(t, a, nil)

		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			// Try and sign again (note no index/wait arg since cache should update in
			// background even if we aren't actively blocking)
			obj, err := a.srv.AgentConnectCALeafCert(resp, req)
			r.Check(err)

			issued2 := obj.(*structs.IssuedCert)
			if issued.CertPEM == issued2.CertPEM {
				r.Fatalf("leaf has not updated")
			}

			// Got a new leaf. Sanity check it's a whole new key as well as different
			// cert.
			if issued.PrivateKeyPEM == issued2.PrivateKeyPEM {
				r.Fatalf("new leaf has same private key as before")
			}

			// Verify that the cert is signed by the new CA
			requireLeafValidUnderCA(t, issued2, ca)

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

func requireLeafValidUnderCA(t *testing.T, issued *structs.IssuedCert,
	ca *structs.CARoot) {

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(ca.RootCert)))
	leaf, err := connect.ParseCert(issued.CertPEM)
	require.NoError(t, err)
	_, err = leaf.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	require.NoError(t, err)

	// Verify the private key matches. tls.LoadX509Keypair does this for us!
	_, err = tls.X509KeyPair([]byte(issued.CertPEM), []byte(issued.PrivateKeyPEM))
	require.NoError(t, err)
}

func TestAgentConnectProxyConfig_Blocking(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), testAllowProxyConfig())
	defer a.Shutdown()

	// Define a local service with a managed proxy. It's registered in the test
	// loop to make sure agent state is predictable whatever order tests execute
	// since some alter this service config.
	reg := &structs.ServiceDefinition{
		Name:    "test",
		Address: "127.0.0.1",
		Port:    8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Connect: &structs.ServiceConnect{
			Proxy: &structs.ServiceDefinitionConnectProxy{
				Command: []string{"tubes.sh"},
				Config: map[string]interface{}{
					"bind_port":          1234,
					"connect_timeout_ms": 500,
					"upstreams": []map[string]interface{}{
						{
							"destination_name": "db",
							"local_port":       3131,
						},
					},
				},
			},
		},
	}

	expectedResponse := &api.ConnectProxyConfig{
		ProxyServiceID:    "test-proxy",
		TargetServiceID:   "test",
		TargetServiceName: "test",
		ContentHash:       "4662e51e78609569",
		ExecMode:          "daemon",
		Command:           []string{"tubes.sh"},
		Config: map[string]interface{}{
			"upstreams": []interface{}{
				map[string]interface{}{
					"destination_name": "db",
					"local_port":       float64(3131),
				},
			},
			"bind_address":          "127.0.0.1",
			"local_service_address": "127.0.0.1:8000",
			"bind_port":             int(1234),
			"connect_timeout_ms":    float64(500),
		},
	}

	ur, err := copystructure.Copy(expectedResponse)
	require.NoError(t, err)
	updatedResponse := ur.(*api.ConnectProxyConfig)
	updatedResponse.ContentHash = "23b5b6b3767601e1"
	upstreams := updatedResponse.Config["upstreams"].([]interface{})
	upstreams = append(upstreams,
		map[string]interface{}{
			"destination_name": "cache",
			"local_port":       float64(4242),
		})
	updatedResponse.Config["upstreams"] = upstreams

	tests := []struct {
		name       string
		url        string
		updateFunc func()
		wantWait   time.Duration
		wantCode   int
		wantErr    bool
		wantResp   *api.ConnectProxyConfig
	}{
		{
			name:     "simple fetch",
			url:      "/v1/agent/connect/proxy/test-proxy",
			wantCode: 200,
			wantErr:  false,
			wantResp: expectedResponse,
		},
		{
			name:     "blocking fetch timeout, no change",
			url:      "/v1/agent/connect/proxy/test-proxy?hash=" + expectedResponse.ContentHash + "&wait=100ms",
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantErr:  false,
			wantResp: expectedResponse,
		},
		{
			name:     "blocking fetch old hash should return immediately",
			url:      "/v1/agent/connect/proxy/test-proxy?hash=123456789abcd&wait=10m",
			wantCode: 200,
			wantErr:  false,
			wantResp: expectedResponse,
		},
		{
			name: "blocking fetch returns change",
			url:  "/v1/agent/connect/proxy/test-proxy?hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Re-register with new proxy config
				r2, err := copystructure.Copy(reg)
				require.NoError(t, err)
				reg2 := r2.(*structs.ServiceDefinition)
				reg2.Connect.Proxy.Config = updatedResponse.Config
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(r2))
				resp := httptest.NewRecorder()
				_, err = a.srv.AgentRegisterService(resp, req)
				require.NoError(t, err)
				require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
			},
			wantWait: 100 * time.Millisecond,
			wantCode: 200,
			wantErr:  false,
			wantResp: updatedResponse,
		},
		{
			// This test exercises a case that caused a busy loop to eat CPU for the
			// entire duration of the blocking query. If a service gets re-registered
			// wth same proxy config then the old proxy config chan is closed causing
			// blocked watchset.Watch to return false indicating a change. But since
			// the hash is the same when the blocking fn is re-called we should just
			// keep blocking on the next iteration. The bug hit was that the WatchSet
			// ws was not being reset in the loop and so when you try to `Watch` it
			// the second time it just returns immediately making the blocking loop
			// into a busy-poll!
			//
			// This test though doesn't catch that because busy poll still has the
			// correct external behaviour. I don't want to instrument the loop to
			// assert it's not executing too fast here as I can't think of a clean way
			// and the issue is fixed now so this test doesn't actually catch the
			// error, but does provide an easy way to verify the behaviour by hand:
			//  1. Make this test fail e.g. change wantErr to true
			//  2. Add a log.Println or similar into the blocking loop/function
			//  3. See whether it's called just once or many times in a tight loop.
			name: "blocking fetch interrupted with no change (same hash)",
			url:  "/v1/agent/connect/proxy/test-proxy?wait=200ms&hash=" + expectedResponse.ContentHash,
			updateFunc: func() {
				time.Sleep(100 * time.Millisecond)
				// Re-register with _same_ proxy config
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
				resp := httptest.NewRecorder()
				_, err = a.srv.AgentRegisterService(resp, req)
				require.NoError(t, err)
				require.Equal(t, 200, resp.Code, "body: %s", resp.Body.String())
			},
			wantWait: 200 * time.Millisecond,
			wantCode: 200,
			wantErr:  false,
			wantResp: expectedResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			// Register the basic service to ensure it's in a known state to start.
			{
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
				resp := httptest.NewRecorder()
				_, err := a.srv.AgentRegisterService(resp, req)
				require.NoError(err)
				require.Equal(200, resp.Code, "body: %s", resp.Body.String())
			}

			req, _ := http.NewRequest("GET", tt.url, nil)
			resp := httptest.NewRecorder()
			if tt.updateFunc != nil {
				go tt.updateFunc()
			}
			start := time.Now()
			obj, err := a.srv.AgentConnectProxyConfig(resp, req)
			elapsed := time.Now().Sub(start)

			if tt.wantErr {
				require.Error(err)
			} else {
				require.NoError(err)
			}
			if tt.wantCode != 0 {
				require.Equal(tt.wantCode, resp.Code, "body: %s", resp.Body.String())
			}
			if tt.wantWait != 0 {
				assert.True(elapsed >= tt.wantWait, "should have waited at least %s, "+
					"took %s", tt.wantWait, elapsed)
			} else {
				assert.True(elapsed < 10*time.Millisecond, "should not have waited, "+
					"took %s", elapsed)
			}

			assert.Equal(tt.wantResp, obj)

			assert.Equal(tt.wantResp.ContentHash, resp.Header().Get("X-Consul-ContentHash"))
		})
	}
}

func TestAgentConnectProxyConfig_aclDefaultDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	req, _ := http.NewRequest("GET", "/v1/agent/connect/proxy/test-id-proxy", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectProxyConfig(resp, req)
	require.True(acl.IsErrPermissionDenied(err))
}

func TestAgentConnectProxyConfig_aclProxyToken(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Get the proxy token from the agent directly, since there is no API
	// to expose this.
	proxy := a.State.Proxy("test-id-proxy")
	require.NotNil(proxy)
	token := proxy.ProxyToken
	require.NotEmpty(token)

	req, _ := http.NewRequest(
		"GET", "/v1/agent/connect/proxy/test-id-proxy?token="+token, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectProxyConfig(resp, req)
	require.NoError(err)
	proxyCfg := obj.(*api.ConnectProxyConfig)
	require.Equal("test-id-proxy", proxyCfg.ProxyServiceID)
	require.Equal("test-id", proxyCfg.TargetServiceID)
	require.Equal("test", proxyCfg.TargetServiceName)
}

func TestAgentConnectProxyConfig_aclServiceWrite(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:write for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "write" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest(
		"GET", "/v1/agent/connect/proxy/test-id-proxy?token="+token, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentConnectProxyConfig(resp, req)
	require.NoError(err)
	proxyCfg := obj.(*api.ConnectProxyConfig)
	require.Equal("test-id-proxy", proxyCfg.ProxyServiceID)
	require.Equal("test-id", proxyCfg.TargetServiceID)
	require.Equal("test", proxyCfg.TargetServiceName)
}

func TestAgentConnectProxyConfig_aclServiceReadDeny(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig()+testAllowProxyConfig())
	defer a.Shutdown()

	// Register a service with a managed proxy
	{
		reg := &structs.ServiceDefinition{
			ID:      "test-id",
			Name:    "test",
			Address: "127.0.0.1",
			Port:    8000,
			Check: structs.CheckType{
				TTL: 15 * time.Second,
			},
			Connect: &structs.ServiceConnect{
				Proxy: &structs.ServiceDefinitionConnectProxy{},
			},
		}

		req, _ := http.NewRequest("PUT", "/v1/agent/service/register?token=root", jsonReader(reg))
		resp := httptest.NewRecorder()
		_, err := a.srv.AgentRegisterService(resp, req)
		require.NoError(err)
		require.Equal(200, resp.Code, "body: %s", resp.Body.String())
	}

	// Create an ACL with service:read for our service
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "test" { policy = "read" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	req, _ := http.NewRequest(
		"GET", "/v1/agent/connect/proxy/test-id-proxy?token="+token, nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectProxyConfig(resp, req)
	require.True(acl.IsErrPermissionDenied(err))
}

func makeTelemetryDefaults(targetID string) lib.TelemetryConfig {
	return lib.TelemetryConfig{
		FilterDefault: true,
		MetricsPrefix: "consul.proxy." + targetID,
	}
}

func TestAgentConnectProxyConfig_ConfigHandling(t *testing.T) {
	t.Parallel()

	// Get the default command to compare below
	defaultCommand, err := defaultProxyCommand(nil)
	require.NoError(t, err)

	// Define a local service with a managed proxy. It's registered in the test
	// loop to make sure agent state is predictable whatever order tests execute
	// since some alter this service config.
	reg := &structs.ServiceDefinition{
		ID:      "test-id",
		Name:    "test",
		Address: "127.0.0.1",
		Port:    8000,
		Check: structs.CheckType{
			TTL: 15 * time.Second,
		},
		Connect: &structs.ServiceConnect{
			// Proxy is populated with the definition in the table below.
		},
	}

	tests := []struct {
		name         string
		globalConfig string
		proxy        structs.ServiceDefinitionConnectProxy
		useToken     string
		wantMode     api.ProxyExecMode
		wantCommand  []string
		wantConfig   map[string]interface{}
	}{
		{
			name: "defaults",
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			`,
			proxy:       structs.ServiceDefinitionConnectProxy{},
			wantMode:    api.ProxyExecModeDaemon,
			wantCommand: defaultCommand,
			wantConfig: map[string]interface{}{
				"bind_address":          "0.0.0.0",
				"bind_port":             10000,            // "randomly" chosen from our range of 1
				"local_service_address": "127.0.0.1:8000", // port from service reg
				"telemetry":             makeTelemetryDefaults(reg.ID),
			},
		},
		{
			name: "global defaults - script",
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
				proxy_defaults = {
					exec_mode = "script"
					script_command = ["script.sh"]
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			`,
			proxy:       structs.ServiceDefinitionConnectProxy{},
			wantMode:    api.ProxyExecModeScript,
			wantCommand: []string{"script.sh"},
			wantConfig: map[string]interface{}{
				"bind_address":          "0.0.0.0",
				"bind_port":             10000,            // "randomly" chosen from our range of 1
				"local_service_address": "127.0.0.1:8000", // port from service reg
				"telemetry":             makeTelemetryDefaults(reg.ID),
			},
		},
		{
			name: "global defaults - daemon",
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
				proxy_defaults = {
					exec_mode = "daemon"
					daemon_command = ["daemon.sh"]
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			`,
			proxy:       structs.ServiceDefinitionConnectProxy{},
			wantMode:    api.ProxyExecModeDaemon,
			wantCommand: []string{"daemon.sh"},
			wantConfig: map[string]interface{}{
				"bind_address":          "0.0.0.0",
				"bind_port":             10000,            // "randomly" chosen from our range of 1
				"local_service_address": "127.0.0.1:8000", // port from service reg
				"telemetry":             makeTelemetryDefaults(reg.ID),
			},
		},
		{
			name: "global default config merge",
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
				proxy_defaults = {
					config = {
						connect_timeout_ms = 1000
					}
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			telemetry {
				statsite_address = "localhost:8989"
			}
			`,
			proxy: structs.ServiceDefinitionConnectProxy{
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
			wantMode:    api.ProxyExecModeDaemon,
			wantCommand: defaultCommand,
			wantConfig: map[string]interface{}{
				"bind_address":          "0.0.0.0",
				"bind_port":             10000,            // "randomly" chosen from our range of 1
				"local_service_address": "127.0.0.1:8000", // port from service reg
				"connect_timeout_ms":    1000,
				"foo":                   "bar",
				"telemetry": lib.TelemetryConfig{
					FilterDefault: true,
					MetricsPrefix: "consul.proxy." + reg.ID,
					StatsiteAddr:  "localhost:8989",
				},
			},
		},
		{
			name: "overrides in reg",
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
				proxy_defaults = {
					exec_mode = "daemon"
					daemon_command = ["daemon.sh"]
					script_command = ["script.sh"]
					config = {
						connect_timeout_ms = 1000
					}
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			telemetry {
				statsite_address = "localhost:8989"
			}
			`,
			proxy: structs.ServiceDefinitionConnectProxy{
				ExecMode: "script",
				Command:  []string{"foo.sh"},
				Config: map[string]interface{}{
					"connect_timeout_ms":    2000,
					"bind_address":          "127.0.0.1",
					"bind_port":             1024,
					"local_service_address": "127.0.0.1:9191",
					"telemetry": map[string]interface{}{
						"statsite_address": "stats.it:10101",
						"metrics_prefix":   "foo", // important! checks that our prefix logic respects user customization
					},
				},
			},
			wantMode:    api.ProxyExecModeScript,
			wantCommand: []string{"foo.sh"},
			wantConfig: map[string]interface{}{
				"bind_address":          "127.0.0.1",
				"bind_port":             int(1024),
				"local_service_address": "127.0.0.1:9191",
				"connect_timeout_ms":    float64(2000),
				"telemetry": lib.TelemetryConfig{
					FilterDefault: true,
					MetricsPrefix: "foo",
					StatsiteAddr:  "stats.it:10101",
				},
			},
		},
		{
			name: "reg telemetry not compatible, preserved with no merge",
			globalConfig: `
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			telemetry {
				statsite_address = "localhost:8989"
			}
			`,
			proxy: structs.ServiceDefinitionConnectProxy{
				ExecMode: "script",
				Command:  []string{"foo.sh"},
				Config: map[string]interface{}{
					"telemetry": map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			wantMode:    api.ProxyExecModeScript,
			wantCommand: []string{"foo.sh"},
			wantConfig: map[string]interface{}{
				"bind_address":          "127.0.0.1",
				"bind_port":             10000,            // "randomly" chosen from our range of 1
				"local_service_address": "127.0.0.1:8000", // port from service reg
				"telemetry": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		{
			name:     "reg passed through with no agent config added if not proxy token auth",
			useToken: "foo", // no actual ACLs set so this any token will work but has to be non-empty to be used below
			globalConfig: `
			bind_addr = "0.0.0.0"
			connect {
				enabled = true
				proxy {
					allow_managed_api_registration = true
				}
				proxy_defaults = {
					exec_mode = "daemon"
					daemon_command = ["daemon.sh"]
					script_command = ["script.sh"]
					config = {
						connect_timeout_ms = 1000
					}
				}
			}
			ports {
				proxy_min_port = 10000
				proxy_max_port = 10000
			}
			telemetry {
				statsite_address = "localhost:8989"
			}
			`,
			proxy: structs.ServiceDefinitionConnectProxy{
				ExecMode: "script",
				Command:  []string{"foo.sh"},
				Config: map[string]interface{}{
					"connect_timeout_ms":    2000,
					"bind_address":          "127.0.0.1",
					"bind_port":             1024,
					"local_service_address": "127.0.0.1:9191",
					"telemetry": map[string]interface{}{
						"metrics_prefix": "foo",
					},
				},
			},
			wantMode:    api.ProxyExecModeScript,
			wantCommand: []string{"foo.sh"},
			wantConfig: map[string]interface{}{
				"bind_address":          "127.0.0.1",
				"bind_port":             int(1024),
				"local_service_address": "127.0.0.1:9191",
				"connect_timeout_ms":    float64(2000),
				"telemetry": map[string]interface{}{ // No defaults merged
					"metrics_prefix": "foo",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			a := NewTestAgent(t.Name(), tt.globalConfig)
			defer a.Shutdown()

			// Register the basic service with the required config
			{
				reg.Connect.Proxy = &tt.proxy
				req, _ := http.NewRequest("PUT", "/v1/agent/service/register", jsonReader(reg))
				resp := httptest.NewRecorder()
				_, err := a.srv.AgentRegisterService(resp, req)
				require.NoError(err)
				require.Equal(200, resp.Code, "body: %s", resp.Body.String())
			}

			proxy := a.State.Proxy("test-id-proxy")
			require.NotNil(proxy)
			require.NotEmpty(proxy.ProxyToken)

			req, _ := http.NewRequest("GET", "/v1/agent/connect/proxy/test-id-proxy", nil)
			if tt.useToken != "" {
				req.Header.Set("X-Consul-Token", tt.useToken)
			} else {
				req.Header.Set("X-Consul-Token", proxy.ProxyToken)
			}
			resp := httptest.NewRecorder()
			obj, err := a.srv.AgentConnectProxyConfig(resp, req)
			require.NoError(err)

			proxyCfg := obj.(*api.ConnectProxyConfig)
			assert.Equal("test-id-proxy", proxyCfg.ProxyServiceID)
			assert.Equal("test-id", proxyCfg.TargetServiceID)
			assert.Equal("test", proxyCfg.TargetServiceName)
			assert.Equal(tt.wantMode, proxyCfg.ExecMode)
			assert.Equal(tt.wantCommand, proxyCfg.Command)
			require.Equal(tt.wantConfig, proxyCfg.Config)
		})
	}
}

func TestAgentConnectAuthorize_badBody(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := []string{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(400, resp.Code)
	assert.Contains(resp.Body.String(), "decode")
}

func TestAgentConnectAuthorize_noTarget(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ConnectAuthorizeRequest{}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(400, resp.Code)
	assert.Contains(resp.Body.String(), "Target service")
}

// Client ID is not in the valid URI format
func TestAgentConnectAuthorize_idInvalidFormat(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "tubes",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "Invalid client")
}

// Client ID is a valid URI but its not a service URI
func TestAgentConnectAuthorize_idNotService(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	args := &structs.ConnectAuthorizeRequest{
		Target:        "web",
		ClientCertURI: "spiffe://1234.consul",
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "must be a valid")
}

// Test when there is an intention allowing the connection
func TestAgentConnectAuthorize_allow(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	target := "db"

	// Create some intentions
	var ixnId string
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		require.Nil(a.RPC("Intention.Apply", &req, &ixnId))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	require.Nil(err)
	require.Equal(200, resp.Code)
	require.Equal("MISS", resp.Header().Get("X-Cache"))

	obj := respRaw.(*connectAuthorizeResp)
	require.True(obj.Authorized)
	require.Contains(obj.Reason, "Matched")

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		require.True(obj.Authorized)
		require.Contains(obj.Reason, "Matched")

		// That should've been a cache hit.
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}

	// Change the intention
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpUpdate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.ID = ixnId
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		require.Nil(a.RPC("Intention.Apply", &req, &ixnId))
	}

	// Short sleep lets the cache background refresh happen
	time.Sleep(100 * time.Millisecond)

	// Make the request again
	{
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		require.Nil(err)
		require.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		require.False(obj.Authorized)
		require.Contains(obj.Reason, "Matched")

		// That should've been a cache hit, too, since it updated in the
		// background.
		require.Equal("HIT", resp.Header().Get("X-Cache"))
	}
}

// Test when there is an intention denying the connection
func TestAgentConnectAuthorize_deny(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	target := "db"

	// Create some intentions
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		var reply string
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        target,
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "Matched")
}

// Test when there is an intention allowing service but for a different trust
// domain.
func TestAgentConnectAuthorize_denyTrustDomain(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	target := "db"

	// Create some intentions
	{
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		var reply string
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: "spiffe://fake-domain.consul/ns/default/dc/dc1/svc/web",
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		assert.Nil(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		assert.False(obj.Authorized)
		assert.Contains(obj.Reason, "Identity from an external trust domain")
	}
}

func TestAgentConnectAuthorize_denyWildcard(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	target := "db"

	// Create some intentions
	{
		// Deny wildcard to DB
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "*"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionDeny

		var reply string
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}
	{
		// Allow web to DB
		req := structs.IntentionRequest{
			Datacenter: "dc1",
			Op:         structs.IntentionOpCreate,
			Intention:  structs.TestIntention(t),
		}
		req.Intention.SourceNS = structs.IntentionDefaultNamespace
		req.Intention.SourceName = "web"
		req.Intention.DestinationNS = structs.IntentionDefaultNamespace
		req.Intention.DestinationName = target
		req.Intention.Action = structs.IntentionActionAllow

		var reply string
		assert.Nil(a.RPC("Intention.Apply", &req, &reply))
	}

	// Web should be allowed
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		assert.Nil(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		assert.True(obj.Authorized)
		assert.Contains(obj.Reason, "Matched")
	}

	// API should be denied
	{
		args := &structs.ConnectAuthorizeRequest{
			Target:        target,
			ClientCertURI: connect.TestSpiffeIDService(t, "api").URI().String(),
		}
		req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize", jsonReader(args))
		resp := httptest.NewRecorder()
		respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
		assert.Nil(err)
		assert.Equal(200, resp.Code)

		obj := respRaw.(*connectAuthorizeResp)
		assert.False(obj.Authorized)
		assert.Contains(obj.Reason, "Matched")
	}
}

// Test that authorize fails without service:write for the target service.
func TestAgentConnectAuthorize_serviceWrite(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	// Create an ACL
	var token string
	{
		args := map[string]interface{}{
			"Name":  "User Token",
			"Type":  "client",
			"Rules": `service "foo" { policy = "read" }`,
		}
		req, _ := http.NewRequest("PUT", "/v1/acl/create?token=root", jsonReader(args))
		resp := httptest.NewRecorder()
		obj, err := a.srv.ACLCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		aclResp := obj.(aclCreateResponse)
		token = aclResp.ID
	}

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST",
		"/v1/agent/connect/authorize?token="+token, jsonReader(args))
	resp := httptest.NewRecorder()
	_, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.True(acl.IsErrPermissionDenied(err))
}

// Test when no intentions match w/ a default deny policy
func TestAgentConnectAuthorize_defaultDeny(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.False(obj.Authorized)
	assert.Contains(obj.Reason, "Default behavior")
}

// Test when no intentions match w/ a default allow policy
func TestAgentConnectAuthorize_defaultAllow(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), `
		acl_datacenter = "dc1"
		acl_default_policy = "allow"
		acl_master_token = "root"
		acl_agent_token = "root"
		acl_agent_master_token = "towel"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	args := &structs.ConnectAuthorizeRequest{
		Target:        "foo",
		ClientCertURI: connect.TestSpiffeIDService(t, "web").URI().String(),
	}
	req, _ := http.NewRequest("POST", "/v1/agent/connect/authorize?token=root", jsonReader(args))
	resp := httptest.NewRecorder()
	respRaw, err := a.srv.AgentConnectAuthorize(resp, req)
	assert.Nil(err)
	assert.Equal(200, resp.Code)

	obj := respRaw.(*connectAuthorizeResp)
	assert.True(obj.Authorized)
	assert.Contains(obj.Reason, "Default behavior")
}

// testAllowProxyConfig returns agent config to allow managed proxy API
// registration.
func testAllowProxyConfig() string {
	return `
		connect {
			enabled = true

			proxy {
				allow_managed_api_registration = true
			}
		}
	`
}
