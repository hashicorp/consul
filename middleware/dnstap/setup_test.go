package dnstap

import (
	"github.com/mholt/caddy"
	"testing"
)

func TestConfig(t *testing.T) {
	file := "dnstap dnstap.sock full"
	c := caddy.NewTestController("dns", file)
	if path, full, err := parseConfig(&c.Dispenser); path != "dnstap.sock" || !full {
		t.Fatalf("%s: %s", file, err)
	}
	file = "dnstap dnstap.sock"
	c = caddy.NewTestController("dns", file)
	if path, full, err := parseConfig(&c.Dispenser); path != "dnstap.sock" || full {
		t.Fatalf("%s: %s", file, err)
	}
}
