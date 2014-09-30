package consul

import (
	"fmt"
	"os"
	"testing"
)

func TestUserEventNames(t *testing.T) {
	out := userEventName("foo")
	if out != "consul:event:foo" {
		t.Fatalf("bad: %v", out)
	}
	if !isUserEvent(out) {
		t.Fatalf("bad")
	}
	if isUserEvent("foo") {
		t.Fatalf("bad")
	}
	if raw := rawUserEventName(out); raw != "foo" {
		t.Fatalf("bad: %v", raw)
	}
}

func TestKeyringRPCError(t *testing.T) {
	dir1, s1 := testServerDC(t, "dc1")
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// RPC error from remote datacenter is returned
	if err := s1.keyringRPC("Bad.Method", nil, nil); err == nil {
		t.Fatalf("bad")
	}
}
