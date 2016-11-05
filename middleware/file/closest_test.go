package file

import (
	"strings"
	"testing"
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

	for _, tc := range tests {
		ce, _ := z.ClosestEncloser(tc.in)
		if ce == nil {
			if z.origin != tc.out {
				t.Errorf("Expected ce to be %s for %s, got %s", tc.out, tc.in, ce.Name())
			}
			continue
		}
		if ce.Name() != tc.out {
			t.Errorf("Expected ce to be %s for %s, got %s", tc.out, tc.in, ce.Name())
		}
	}
}
