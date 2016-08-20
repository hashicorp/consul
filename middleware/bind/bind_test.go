package bind

import (
	"testing"

	"github.com/miekg/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

func TestSetupBind(t *testing.T) {
	c := caddy.NewTestController("dns", `bind 1.2.3.4`)
	err := setupBind(c)
	if err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	cfg := dnsserver.GetConfig(c)
	if got, want := cfg.ListenHost, "1.2.3.4"; got != want {
		t.Errorf("Expected the config's ListenHost to be %s, was %s", want, got)
	}
}

func TestBindAddress(t *testing.T) {
	c := caddy.NewTestController("dns", `bind 1.2.3.bla`)
	err := setupBind(c)
	if err == nil {
		t.Fatalf("Expected errors, but got none")
	}
}
