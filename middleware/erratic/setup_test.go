package erratic

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupWhoami(t *testing.T) {
	c := caddy.NewTestController("dns", `erratic {
		drop
	}`)
	if err := setupErratic(c); err != nil {
		t.Fatalf("Test 1, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `erratic`)
	if err := setupErratic(c); err != nil {
		t.Fatalf("Test 2, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `erratic {
		drop -1
	}`)
	if err := setupErratic(c); err == nil {
		t.Fatalf("Test 4, expected errors, but got: %q", err)
	}
}
