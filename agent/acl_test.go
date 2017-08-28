package agent

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	rawacl "github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"
)

func TestACL_Bad_Config(t *testing.T) {
	t.Parallel()

	dataDir := testutil.TempDir(t, "agent")
	defer os.Remove(dataDir)

	cfg := TestConfig(config.Source{
		Name:   "acl",
		Format: "hcl",
		Data: `
			acl_down_policy = "nope"
			data_dir = "` + dataDir + `"
		`,
	})

	// do not use TestAgent here since we want
	// the agent to fail during startup.
	_, err := New(cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid ACL down policy") {
		t.Fatalf("err: %v", err)
	}
}

type MockServer struct {
	getPolicyFn func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error
}

func (m *MockServer) GetPolicy(args *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
	if m.getPolicyFn != nil {
		return m.getPolicyFn(args, reply)
	}
	return fmt.Errorf("should not have called GetPolicy")
}

func TestACL_Version8(t *testing.T) {
	t.Parallel()

	t.Run("version 8 disabled", func(t *testing.T) {
		a := NewTestAgent(t.Name(), TestACLConfig()+`
 		acl_enforce_version_8 = false
 	`)
		defer a.Shutdown()

		m := MockServer{
			getPolicyFn: func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
				t.Fatalf("should not have called to server")
				return nil
			},
		}
		if err := a.registerEndpoint("ACL", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		if token, err := a.resolveToken("nope"); token != nil || err != nil {
			t.Fatalf("bad: %v err: %v", token, err)
		}
	})

	t.Run("version 8 enabled", func(t *testing.T) {
		a := NewTestAgent(t.Name(), TestACLConfig()+`
 		acl_enforce_version_8 = true
 	`)
		defer a.Shutdown()

		var called bool
		m := MockServer{
			getPolicyFn: func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
				called = true
				return fmt.Errorf("token not found")
			},
		}
		if err := a.registerEndpoint("ACL", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, err := a.resolveToken("nope"); err != nil {
			t.Fatalf("err: %v", err)
		}

		if !called {
			t.Fatalf("bad")
		}
	})
}

func TestACL_Disabled(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_disabled_ttl = "10ms"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{
		// Fetch a token without ACLs enabled and make sure the manager sees it.
		getPolicyFn: func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
			return rawacl.ErrDisabled
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	if a.acls.isDisabled() {
		t.Fatalf("should not be disabled yet")
	}
	if token, err := a.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
	if !a.acls.isDisabled() {
		t.Fatalf("should be disabled")
	}

	// Now turn on ACLs and check right away, it should still think ACLs are
	// disabled since we don't check again right away.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return rawacl.ErrNotFound
	}
	if token, err := a.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
	if !a.acls.isDisabled() {
		t.Fatalf("should be disabled")
	}

	// Wait the waiting period and make sure it checks again. Do a few tries
	// to make sure we don't think it's disabled.
	time.Sleep(2 * 10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		_, err := a.resolveToken("nope")
		if !rawacl.IsErrNotFound(err) {
			t.Fatalf("err: %v", err)
		}
		if a.acls.isDisabled() {
			t.Fatalf("should not be disabled")
		}
	}
}

func TestACL_Special_IDs(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
 		acl_enforce_version_8 = true
 		acl_agent_master_token = "towel"
 	`)
	defer a.Shutdown()

	m := MockServer{
		// An empty ID should get mapped to the anonymous token.
		getPolicyFn: func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
			if req.ACL != "anonymous" {
				t.Fatalf("bad: %#v", *req)
			}
			return rawacl.ErrNotFound
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err := a.resolveToken("")
	if !rawacl.IsErrNotFound(err) {
		t.Fatalf("err: %v", err)
	}

	// A root ACL request should get rejected and not call the server.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	_, err = a.resolveToken("deny")
	if !rawacl.IsErrRootDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The ACL master token should also not call the server, but should give
	// us a working agent token.
	acl, err := a.resolveToken("towel")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should be able to read agent")
	}
	if !acl.AgentWrite(a.config.NodeName) {
		t.Fatalf("should be able to write agent")
	}
	if !acl.NodeRead("hello") {
		t.Fatalf("should be able to read any node")
	}
	if acl.NodeWrite("hello", nil) {
		t.Fatalf("should not be able to write any node")
	}
}

func TestACL_Down_Deny(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_down_policy = "deny"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{
		// Resolve with ACLs down.
		getPolicyFn: func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
			return fmt.Errorf("ACLs are broken")
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := a.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Down_Allow(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_down_policy = "allow"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{
		// Resolve with ACLs down.
		getPolicyFn: func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
			return fmt.Errorf("ACLs are broken")
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := a.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
}

func TestACL_Down_Extend(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_down_policy = "extend-cache"
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{
		// Populate the cache for one of the tokens.
		getPolicyFn: func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
			*reply = structs.ACLPolicy{
				Parent: "allow",
				Policy: &rawacl.Policy{
					Agents: []*rawacl.AgentPolicy{
						&rawacl.AgentPolicy{
							Node:   a.config.NodeName,
							Policy: "read",
						},
					},
				},
			}
			return nil
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	acl, err := a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(a.config.NodeName) {
		t.Fatalf("should deny")
	}

	// Now take down ACLs and make sure a new token fails to resolve.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err = a.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should deny")
	}
	if acl.AgentWrite(a.config.NodeName) {
		t.Fatalf("should deny")
	}

	// Read the token from the cache while ACLs are broken, which should
	// extend.
	acl, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(a.config.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Cache(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{
		// Populate the cache for one of the tokens.
		getPolicyFn: func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
			*reply = structs.ACLPolicy{
				ETag:   "hash1",
				Parent: "deny",
				Policy: &rawacl.Policy{
					Agents: []*rawacl.AgentPolicy{
						&rawacl.AgentPolicy{
							Node:   a.config.NodeName,
							Policy: "read",
						},
					},
				},
				TTL: 10 * time.Millisecond,
			}
			return nil
		},
	}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	rule, err := a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rule == nil {
		t.Fatalf("should not be nil")
	}
	if !rule.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if rule.AgentWrite(a.config.NodeName) {
		t.Fatalf("should deny")
	}
	if rule.NodeRead("nope") {
		t.Fatalf("should deny")
	}

	// Fetch right away and make sure it uses the cache.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	rule, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rule == nil {
		t.Fatalf("should not be nil")
	}
	if !rule.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if rule.AgentWrite(a.config.NodeName) {
		t.Fatalf("should deny")
	}
	if rule.NodeRead("nope") {
		t.Fatalf("should deny")
	}

	// Wait for the TTL to expire and try again. This time the token will be
	// gone.
	time.Sleep(20 * time.Millisecond)
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		return rawacl.ErrNotFound
	}
	_, err = a.resolveToken("yep")
	if !rawacl.IsErrNotFound(err) {
		t.Fatalf("err: %v", err)
	}

	// Page it back in with a new tag and different policy
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		*reply = structs.ACLPolicy{
			ETag:   "hash2",
			Parent: "deny",
			Policy: &rawacl.Policy{
				Agents: []*rawacl.AgentPolicy{
					&rawacl.AgentPolicy{
						Node:   a.config.NodeName,
						Policy: "write",
					},
				},
			},
			TTL: 10 * time.Millisecond,
		}
		return nil
	}
	rule, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rule == nil {
		t.Fatalf("should not be nil")
	}
	if !rule.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if !rule.AgentWrite(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if rule.NodeRead("nope") {
		t.Fatalf("should deny")
	}

	// Wait for the TTL to expire and try again. This will match the tag
	// and not send the policy back, but we should have the old token
	// behavior.
	time.Sleep(20 * time.Millisecond)
	var didRefresh bool
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		*reply = structs.ACLPolicy{
			ETag: "hash2",
			TTL:  10 * time.Millisecond,
		}
		didRefresh = true
		return nil
	}
	rule, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rule == nil {
		t.Fatalf("should not be nil")
	}
	if !rule.AgentRead(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if !rule.AgentWrite(a.config.NodeName) {
		t.Fatalf("should allow")
	}
	if rule.NodeRead("nope") {
		t.Fatalf("should deny")
	}
	if !didRefresh {
		t.Fatalf("should refresh")
	}
}

// catalogPolicy supplies some standard policies to help with testing the
// catalog-related vet and filter functions.
func catalogPolicy(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
	reply.Policy = &rawacl.Policy{}

	switch req.ACL {

	case "node-ro":
		reply.Policy.Nodes = append(reply.Policy.Nodes,
			&rawacl.NodePolicy{Name: "Node", Policy: "read"})

	case "node-rw":
		reply.Policy.Nodes = append(reply.Policy.Nodes,
			&rawacl.NodePolicy{Name: "Node", Policy: "write"})

	case "service-ro":
		reply.Policy.Services = append(reply.Policy.Services,
			&rawacl.ServicePolicy{Name: "service", Policy: "read"})

	case "service-rw":
		reply.Policy.Services = append(reply.Policy.Services,
			&rawacl.ServicePolicy{Name: "service", Policy: "write"})

	case "other-rw":
		reply.Policy.Services = append(reply.Policy.Services,
			&rawacl.ServicePolicy{Name: "other", Policy: "write"})

	default:
		return fmt.Errorf("unknown token %q", req.ACL)
	}

	return nil
}

func TestACL_vetServiceRegister(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a new service, with permission.
	err := a.vetServiceRegister("service-rw", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a new service without write privs.
	err = a.vetServiceRegister("service-ro", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a service without write privs to the existing
	// service.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "other",
	}, "")
	err = a.vetServiceRegister("service-rw", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetServiceUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update a service that doesn't exist.
	err := a.vetServiceUpdate("service-rw", "my-service")
	if err == nil || !strings.Contains(err.Error(), "Unknown service") {
		t.Fatalf("err: %v", err)
	}

	// Update with write privs.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	err = a.vetServiceUpdate("service-rw", "my-service")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update without write privs.
	err = a.vetServiceUpdate("service-ro", "my-service")
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetCheckRegister(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a new service check with write privs.
	err := a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a new service check without write privs.
	err = a.vetCheckRegister("service-ro", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Register a new node check with write privs.
	err = a.vetCheckRegister("node-rw", &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a new node check without write privs.
	err = a.vetCheckRegister("node-ro", &structs.HealthCheck{
		CheckID: types.CheckID("my-check"),
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a service check without write privs to the
	// existing service.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "other",
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a node check without write privs to the node.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-node-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetCheckUpdate(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update a check that doesn't exist.
	err := a.vetCheckUpdate("node-rw", "my-check")
	if err == nil || !strings.Contains(err.Error(), "Unknown check") {
		t.Fatalf("err: %v", err)
	}

	// Update service check with write privs.
	a.State.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.State.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-service-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	}, "")
	err = a.vetCheckUpdate("service-rw", "my-service-check")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update service check without write privs.
	err = a.vetCheckUpdate("service-ro", "my-service-check")
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Update node check with write privs.
	a.State.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckUpdate("node-rw", "my-node-check")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update without write privs.
	err = a.vetCheckUpdate("node-ro", "my-node-check")
	if !rawacl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_filterMembers(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	var members []serf.Member
	if err := a.filterMembers("node-ro", &members); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("bad: %#v", members)
	}

	members = []serf.Member{
		serf.Member{Name: "Node 1"},
		serf.Member{Name: "Nope"},
		serf.Member{Name: "Node 2"},
	}
	if err := a.filterMembers("node-ro", &members); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(members) != 2 ||
		members[0].Name != "Node 1" ||
		members[1].Name != "Node 2" {
		t.Fatalf("bad: %#v", members)
	}
}

func TestACL_filterServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	services := make(map[string]*structs.NodeService)
	if err := a.filterServices("node-ro", &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	services["my-service"] = &structs.NodeService{ID: "my-service", Service: "service"}
	services["my-other"] = &structs.NodeService{ID: "my-other", Service: "other"}
	if err := a.filterServices("service-ro", &services); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["my-service"]; !ok {
		t.Fatalf("bad: %#v", services)
	}
	if _, ok := services["my-other"]; ok {
		t.Fatalf("bad: %#v", services)
	}
}

func TestACL_filterChecks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_enforce_version_8 = true
	`)
	defer a.Shutdown()

	m := MockServer{catalogPolicy}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks := make(map[types.CheckID]*structs.HealthCheck)
	if err := a.filterChecks("node-ro", &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	if err := a.filterChecks("service-ro", &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["my-node"]; ok {
		t.Fatalf("bad: %#v", checks)
	}
	if _, ok := checks["my-service"]; !ok {
		t.Fatalf("bad: %#v", checks)
	}
	if _, ok := checks["my-other"]; ok {
		t.Fatalf("bad: %#v", checks)
	}

	checks["my-node"] = &structs.HealthCheck{}
	checks["my-service"] = &structs.HealthCheck{ServiceName: "service"}
	checks["my-other"] = &structs.HealthCheck{ServiceName: "other"}
	if err := a.filterChecks("node-ro", &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["my-node"]; !ok {
		t.Fatalf("bad: %#v", checks)
	}
	if _, ok := checks["my-service"]; ok {
		t.Fatalf("bad: %#v", checks)
	}
	if _, ok := checks["my-other"]; ok {
		t.Fatalf("bad: %#v", checks)
	}
}
