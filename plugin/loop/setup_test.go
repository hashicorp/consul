package loop

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestSetup(t *testing.T) {
	c := caddy.NewTestController("dns", `loop`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `loop argument`)
	if err := setup(c); err == nil {
		t.Fatal("Expected errors, but got none")
	}
}
