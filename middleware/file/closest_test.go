package file

import (
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestClosestEncloser(t *testing.T) {
	z, err := Parse(strings.NewReader(dbMiekNL), testzone, "stdin")
	if err != nil {
		t.Fatalf("expect no error when reading zone, got %q", err)
	}

	tests := []struct {
		in, out string
	}{
		{"miek.nl.", "miek.nl."},
		{"www.miek.nl.", "www.miek.nl."},

		{"blaat.miek.nl.", "miek.nl."},
		{"blaat.www.miek.nl.", "www.miek.nl."},
		{"www.blaat.miek.nl.", "miek.nl."},
		{"blaat.a.miek.nl.", "a.miek.nl."},
	}

	mk, _ := dns.TypeToRR[dns.TypeA]
	rr := mk()
	for _, tc := range tests {
		rr.Header().Name = tc.in
		ce := z.ClosestEncloser(rr)
		if ce != tc.out {
			t.Errorf("expected ce to be %s for %s, got %s", tc.out, tc.in, ce)
		}
	}
}
