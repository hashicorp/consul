package cache

import (
	"testing"

	"github.com/miekg/dns"
)

func TestKey(t *testing.T) {
	if x := rawKey("miek.nl.", dns.TypeMX, false); x != "0miek.nl..15" {
		t.Errorf("failed to create correct key, got %s", x)
	}
	if x := rawKey("miek.nl.", dns.TypeMX, true); x != "1miek.nl..15" {
		t.Errorf("failed to create correct key, got %s", x)
	}
	// rawKey does not lowercase.
	if x := rawKey("miEK.nL.", dns.TypeMX, true); x != "1miEK.nL..15" {
		t.Errorf("failed to create correct key, got %s", x)
	}
}
