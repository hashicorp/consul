package consul

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func TestACL_Disabled(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	acl, err := s1.resolveToken("does not exist")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl != nil {
		t.Fatalf("got acl")
	}
}

func TestACL_Authority_NotFound(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1" // Enable ACLs!
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	acl, err := s1.resolveToken("does not exist")
	if err == nil || err.Error() != aclNotFound {
		t.Fatalf("err: %v", err)
	}
	if acl != nil {
		t.Fatalf("got acl")
	}
}

func TestACL_Authority_Found(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1" // Enable ACLs!
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	// Create a new token
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
			Rules: testACLPolicy,
		},
	}
	var id string
	if err := client.Call("ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Resolve the token
	acl, err := s1.resolveToken(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("missing acl")
	}

	// Check the policy
	if acl.KeyRead("bar") {
		t.Fatalf("unexpected read")
	}
	if !acl.KeyRead("foo/test") {
		t.Fatalf("unexpected failed read")
	}
}

func TestACL_NonAuthority_NotFound(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1" // Enable ACLs!
		c.Bootstrap = false     // Disable bootstrap
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		p1, _ := s1.raftPeers.Peers()
		return len(p1) == 2, errors.New(fmt.Sprintf("%v", p1))
	}, func(err error) {
		t.Fatalf("should have 2 peers: %v", err)
	})

	client := rpcClient(t, s1)
	defer client.Close()
	testutil.WaitForLeader(t, client.Call, "dc1")

	// find the non-authoritative server
	var nonAuth *Server
	if !s1.IsLeader() {
		nonAuth = s1
	} else {
		nonAuth = s2
	}

	acl, err := nonAuth.resolveToken("does not exist")
	if err == nil || err.Error() != aclNotFound {
		t.Fatalf("err: %v", err)
	}
	if acl != nil {
		t.Fatalf("got acl")
	}
}

func TestACL_NonAuthority_Found(t *testing.T) {
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1" // Enable ACLs!
		c.Bootstrap = false     // Disable bootstrap
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		p1, _ := s1.raftPeers.Peers()
		return len(p1) == 2, errors.New(fmt.Sprintf("%v", p1))
	}, func(err error) {
		t.Fatalf("should have 2 peers: %v", err)
	})
	testutil.WaitForLeader(t, client.Call, "dc1")

	// Create a new token
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
			Rules: testACLPolicy,
		},
	}
	var id string
	if err := client.Call("ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// find the non-authoritative server
	var nonAuth *Server
	if !s1.IsLeader() {
		nonAuth = s1
	} else {
		nonAuth = s2
	}

	// Token should resolve
	acl, err := nonAuth.resolveToken(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("missing acl")
	}

	// Check the policy
	if acl.KeyRead("bar") {
		t.Fatalf("unexpected read")
	}
	if !acl.KeyRead("foo/test") {
		t.Fatalf("unexpected failed read")
	}
}

var testACLPolicy = `
key "" {
	policy = "deny"
}
key "foo/" {
	policy = "write"
}
`
