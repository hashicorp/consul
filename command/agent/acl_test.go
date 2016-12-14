package agent

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	rawacl "github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func TestACL_Bad_Config(t *testing.T) {
	config := nextConfig()
	config.ACLDownPolicy = "nope"

	var err error
	config.DataDir, err = ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(config.DataDir)

	_, err = Create(config, nil, nil, nil)
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
	} else {
		return fmt.Errorf("should not have called GetPolicy")
	}
}

func TestACL_Version8(t *testing.T) {
	config := nextConfig()
	config.ACLEnforceVersion8 = Bool(false)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// With version 8 enforcement off, this should not get called.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	if token, err := agent.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
}

func TestACL_Disabled(t *testing.T) {
	config := nextConfig()
	config.ACLDisabledTTL = 10 * time.Millisecond
	config.ACLEnforceVersion8 = Bool(true)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Fetch a token without ACLs enabled and make sure the manager sees it.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return errors.New(aclDisabled)
	}
	if agent.acls.isDisabled() {
		t.Fatalf("should not be disabled yet")
	}
	if token, err := agent.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
	if !agent.acls.isDisabled() {
		t.Fatalf("should be disabled")
	}

	// Now turn on ACLs and check right away, it should still think ACLs are
	// disabled since we don't check again right away.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return errors.New(aclNotFound)
	}
	if token, err := agent.resolveToken("nope"); token != nil || err != nil {
		t.Fatalf("bad: %v err: %v", token, err)
	}
	if !agent.acls.isDisabled() {
		t.Fatalf("should be disabled")
	}

	// Wait the waiting period and make sure it checks again. Do a few tries
	// to make sure we don't think it's disabled.
	time.Sleep(2 * config.ACLDisabledTTL)
	for i := 0; i < 10; i++ {
		_, err := agent.resolveToken("nope")
		if err == nil || !strings.Contains(err.Error(), aclNotFound) {
			t.Fatalf("err: %v", err)
		}
		if agent.acls.isDisabled() {
			t.Fatalf("should not be disabled")
		}
	}
}

func TestACL_Special_IDs(t *testing.T) {
	config := nextConfig()
	config.ACLEnforceVersion8 = Bool(true)
	config.ACLAgentMasterToken = "towel"

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// An empty ID should get mapped to the anonymous token.
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		if req.ACL != "anonymous" {
			t.Fatalf("bad: %#v", *req)
		}
		return errors.New(aclNotFound)
	}
	_, err := agent.resolveToken("")
	if err == nil || !strings.Contains(err.Error(), aclNotFound) {
		t.Fatalf("err: %v", err)
	}

	// A root ACL request should get rejected and not call the server.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		t.Fatalf("should not have called to server")
		return nil
	}
	_, err = agent.resolveToken("deny")
	if err == nil || !strings.Contains(err.Error(), rootDenied) {
		t.Fatalf("err: %v", err)
	}

	// The ACL master token should also not call the server, but should give
	// us a working agent token.
	acl, err := agent.resolveToken("towel")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should be able to read agent")
	}
	if !acl.AgentWrite(config.NodeName) {
		t.Fatalf("should be able to write agent")
	}
}

func TestACL_Down_Deny(t *testing.T) {
	config := nextConfig()
	config.ACLDownPolicy = "deny"
	config.ACLEnforceVersion8 = Bool(true)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve with ACLs down.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err := agent.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if acl.AgentRead(config.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Down_Allow(t *testing.T) {
	config := nextConfig()
	config.ACLDownPolicy = "allow"
	config.ACLEnforceVersion8 = Bool(true)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve with ACLs down.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err := agent.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
}

func TestACL_Down_Extend(t *testing.T) {
	config := nextConfig()
	config.ACLDownPolicy = "extend-cache"
	config.ACLEnforceVersion8 = Bool(true)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Populate the cache for one of the tokens.
	m.getPolicyFn = func(req *structs.ACLPolicyRequest, reply *structs.ACLPolicy) error {
		*reply = structs.ACLPolicy{
			Parent: "allow",
			Policy: &rawacl.Policy{
				Agents: []*rawacl.AgentPolicy{
					&rawacl.AgentPolicy{
						Node:   config.NodeName,
						Policy: "read",
					},
				},
			},
		}
		return nil
	}
	acl, err := agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(config.NodeName) {
		t.Fatalf("should deny")
	}

	// Now take down ACLs and make sure a new token fails to resolve.
	m.getPolicyFn = func(*structs.ACLPolicyRequest, *structs.ACLPolicy) error {
		return fmt.Errorf("ACLs are broken")
	}
	acl, err = agent.resolveToken("nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if acl.AgentRead(config.NodeName) {
		t.Fatalf("should deny")
	}
	if acl.AgentWrite(config.NodeName) {
		t.Fatalf("should deny")
	}

	// Read the token from the cache while ACLs are broken, which should
	// extend.
	acl, err = agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(config.NodeName) {
		t.Fatalf("should deny")
	}
}

func TestACL_Cache(t *testing.T) {
	config := nextConfig()
	config.ACLEnforceVersion8 = Bool(true)

	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	m := MockServer{}
	if err := agent.InjectEndpoint("ACL", &m); err != nil {
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
						Node:   config.NodeName,
						Policy: "read",
					},
				},
			},
			TTL: 10 * time.Millisecond,
		}
		return nil
	}
	acl, err := agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(config.NodeName) {
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
	acl, err = agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.AgentWrite(config.NodeName) {
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
	_, err = agent.resolveToken("yep")
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
						Node:   config.NodeName,
						Policy: "write",
					},
				},
			},
			TTL: 10 * time.Millisecond,
		}
		return nil
	}
	acl, err = agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if !acl.AgentWrite(config.NodeName) {
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
	acl, err = agent.resolveToken("yep")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("should not be nil")
	}
	if !acl.AgentRead(config.NodeName) {
		t.Fatalf("should allow")
	}
	if !acl.AgentWrite(config.NodeName) {
		t.Fatalf("should allow")
	}
	if acl.NodeRead("nope") {
		t.Fatalf("should deny")
	}
	if !didRefresh {
		t.Fatalf("should refresh")
	}
}
