package cache

import (
	"testing"

	"github.com/miekg/dns"
)

func TestKey(t *testing.T) {
	if noDataKey("miek.nl.", dns.TypeMX, false) != "0miek.nl...15" {
		t.Errorf("failed to create correct key")
	}
	if noDataKey("miek.nl.", dns.TypeMX, true) != "1miek.nl...15" {
		t.Errorf("failed to create correct key")
	}
	if nameErrorKey("miek.nl.", false) != "0miek.nl." {
		t.Errorf("failed to create correct key")
	}
	if nameErrorKey("miek.nl.", true) != "1miek.nl." {
		t.Errorf("failed to create correct key")
	}
	if noDataKey("miek.nl.", dns.TypeMX, false) != successKey("miek.nl.", dns.TypeMX, false) {
		t.Errorf("nameErrorKey and successKey should be the same")
	}
}
