package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"os"
	"testing"
	"time"
)

func TestLeader_RegisterMember(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Wait until we have a leader
	time.Sleep(100 * time.Millisecond)

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := c1.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	// Client should be registered
	state := s1.fsm.State()
	found, _ := state.GetNode(c1.config.NodeName)
	if !found {
		t.Fatalf("client not registered")
	}

	// Should have a check
	checks := state.NodeChecks(c1.config.NodeName)
	if len(checks) != 1 {
		t.Fatalf("client missing check")
	}
	if checks[0].CheckID != serfCheckID {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Name != serfCheckName {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Status != structs.HealthPassing {
		t.Fatalf("bad check: %v", checks[0])
	}

	// Server should be registered
	found, _ = state.GetNode(s1.config.NodeName)
	if !found {
		t.Fatalf("server not registered")
	}

	// Service should be registered
	services := state.NodeServices(s1.config.NodeName)
	if _, ok := services.Services["consul"]; !ok {
		t.Fatalf("consul service not registered: %v", services)
	}
}

func TestLeader_FailedMember(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Wait until we have a leader
	time.Sleep(100 * time.Millisecond)

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := c1.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Fail the member
	c1.Shutdown()

	// Wait for failure detection
	time.Sleep(500 * time.Millisecond)

	// Should be registered
	state := s1.fsm.State()
	found, _ := state.GetNode(c1.config.NodeName)
	if !found {
		t.Fatalf("client not registered")
	}

	// Should have a check
	checks := state.NodeChecks(c1.config.NodeName)
	if len(checks) != 1 {
		t.Fatalf("client missing check")
	}
	if checks[0].CheckID != serfCheckID {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Name != serfCheckName {
		t.Fatalf("bad check: %v", checks[0])
	}
	if checks[0].Status != structs.HealthCritical {
		t.Fatalf("bad check: %v", checks[0])
	}
}

func TestLeader_LeftMember(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Wait until we have a leader
	time.Sleep(100 * time.Millisecond)

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := c1.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	// Should be registered
	state := s1.fsm.State()
	found, _ := state.GetNode(c1.config.NodeName)
	if !found {
		t.Fatalf("client not registered")
	}

	// Node should leave
	c1.Leave()
	c1.Shutdown()

	// Wait for failure detection
	time.Sleep(500 * time.Millisecond)

	// Should be deregistered
	found, _ = state.GetNode(c1.config.NodeName)
	if found {
		t.Fatalf("client registered")
	}
}

func TestLeader_Reconcile(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, c1 := testClient(t)
	defer os.RemoveAll(dir2)
	defer c1.Shutdown()

	// Join before we have a leader, this should cause a reconcile!
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := c1.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not be registered
	state := s1.fsm.State()
	found, _ := state.GetNode(c1.config.NodeName)
	if found {
		t.Fatalf("client registered")
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Should be registered
	found, _ = state.GetNode(c1.config.NodeName)
	if !found {
		t.Fatalf("client not registered")
	}
}
