package whoami

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupWhoami(t *testing.T) {
	c := caddy.NewTestController("dns", `whoami`)
	if err := setupWhoami(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `whoami example.org`)
	if err := setupWhoami(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
