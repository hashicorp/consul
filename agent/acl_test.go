package agent

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	rawacl "github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/serf"
)

func TestACL_Bad_Config(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLDownPolicy = "nope"
	cfg.DataDir = testutil.TempDir(t, "agent")

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
	cfg := TestConfig()
	cfg.ACLEnforceVersion8 = Bool(false)
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// With version 8 enforcement off, this should not get called.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	if token, err := a.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
}

func TestACL_Disabled(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLDisabledTTL = 10 * time.Millisecond
	cfg.ACLEnforceVersion8 = Bool(true)
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Fetch a token without ACLs enabled and make sure the manager sees it.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return errors.New(aclDisabled)
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
		return errors.New(aclNotFound)
	}
	if token, err := a.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
	if !a.acls.isDisabled() {
		t.Fatalf("should be disabled")
	}

	// Wait the waiting period and make sure it checks again. Do a few tries
	// to make sure we don't think it's disabled.
	time.Sleep(2 * cfg.ACLDisabledTTL)
	for i := 0; i < 10; i++ {
		_, err := a.resolveToken("nope")
		if err == nil || !strings.Contains(err.Error(), aclNotFound) {
			t.Fatalf("err: %v", err)
		}
		if a.acls.isDisabled() {
			t.Fatalf("should not be disabled")
		}
	}
}

func TestACL_Special_IDs(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)
	cfg.ACLAgentMasterToken = "towel"
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// An empty ID should get mapped to the anonymous token.
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		if req.ACL != "anonymous" {
			t.Fatalf("bad: %#v", *req)
		}
		return errors.New(aclNotFound)
	}
	_, err := a.resolveToken("")
	if err == nil || !strings.Contains(err.Error(), aclNotFound) {
		t.Fatalf("err: %v", err)
	}

	// A root ACL request should get rejected and not call the server.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	_, err = a.resolveToken("deny")
	if err == nil || !strings.Contains(err.Error(), rootDenied) {
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
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should be able to read agent")
	}
	if !acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should be able to write agent")
	}
}

func TestACL_Down_Deny(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLDownPolicy = "deny"
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve with ACLs down.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err := a.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Down_Allow(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLDownPolicy = "allow"
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve with ACLs down.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err := a.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
}

func TestACL_Down_Extend(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLDownPolicy = "extend-cache"
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Populate the cache for one of the tokens.
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		*reply = structs.ACLPolicy{
			Parent: "allow",
			Policy: &rawacl.Policy{
				Agents: []*rawacl.AgentPolicy{
					&rawacl.AgentPolicy{
						Node:   cfg.NodeName,
						Policy: "read",
					},
				},
			},
		}
		return nil
	}
	acl, err := a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(cfg.NodeName) {
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
	if acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should deny")
	}
	if acl.AgentWrite(cfg.NodeName) {
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
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Cache(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	m := MockServer{}
	if err := a.registerEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Populate the cache for one of the tokens.
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		*reply = structs.ACLPolicy{
			ETag:   "hash1",
			Parent: "deny",
			Policy: &rawacl.Policy{
				Agents: []*rawacl.AgentPolicy{
					&rawacl.AgentPolicy{
						Node:   cfg.NodeName,
						Policy: "read",
					},
				},
			},
			TTL: 10 * time.Millisecond,
		}
		return nil
	}
	acl, err := a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should deny")
	}
	if acl.NodeRead("nope") {
		t.Fatalf("should deny")
	}

	// Fetch right away and make sure it uses the cache.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	acl, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should deny")
	}
	if acl.NodeRead("nope") {
		t.Fatalf("should deny")
	}

	// Wait for the TTL to expire and try again. This time the token will be
	// gone.
	time.Sleep(20 * time.Millisecond)
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		return errors.New(aclNotFound)
	}
	_, err = a.resolveToken("yep")
	if err == nil || !strings.Contains(err.Error(), aclNotFound) {
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
						Node:   cfg.NodeName,
						Policy: "write",
					},
				},
			},
			TTL: 10 * time.Millisecond,
		}
		return nil
	}
	acl, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if !acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.NodeRead("nope") {
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
	acl, err = a.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if !acl.AgentWrite(cfg.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.NodeRead("nope") {
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
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a service without write privs to the existing
	// service.
	a.state.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "other",
	}, "")
	err = a.vetServiceRegister("service-rw", &structs.NodeService{
		ID:      "my-service",
		Service: "service",
	})
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetServiceUpdate(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	a.state.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	err = a.vetServiceUpdate("service-rw", "my-service")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update without write privs.
	err = a.vetServiceUpdate("service-ro", "my-service")
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetCheckRegister(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	if !isPermissionDenied(err) {
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
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a service check without write privs to the
	// existing service.
	a.state.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.state.AddCheck(&structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "other",
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try to register over a node check without write privs to the node.
	a.state.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckRegister("service-rw", &structs.HealthCheck{
		CheckID:     types.CheckID("my-node-check"),
		ServiceID:   "my-service",
		ServiceName: "service",
	})
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_vetCheckUpdate(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	a.state.AddService(&structs.NodeService{
		ID:      "my-service",
		Service: "service",
	}, "")
	a.state.AddCheck(&structs.HealthCheck{
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
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Update node check with write privs.
	a.state.AddCheck(&structs.HealthCheck{
		CheckID: types.CheckID("my-node-check"),
	}, "")
	err = a.vetCheckUpdate("node-rw", "my-node-check")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update without write privs.
	err = a.vetCheckUpdate("node-ro", "my-node-check")
	if !isPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestACL_filterMembers(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
	cfg := TestACLConfig()
	cfg.ACLEnforceVersion8 = Bool(true)

	a := NewTestAgent(t.Name(), cfg)
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
