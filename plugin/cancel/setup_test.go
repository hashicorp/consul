package cancel

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetup(t *testing.T) {
	c := caddy.NewTestController("dns", `cancel`)
	if err := setup(c); err != nil {
		t.Errorf("Test 1, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `cancel 5s`)
	if err := setup(c); err != nil {
		t.Errorf("Test 2, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `cancel 5`)
	if err := setup(c); err == nil {
		t.Errorf("Test 3, expected errors, but got none")
	}

	c = caddy.NewTestController("dns", `cancel -1s`)
	if err := setup(c); err == nil {
		t.Errorf("Test 4, expected errors, but got none")
	}
}
