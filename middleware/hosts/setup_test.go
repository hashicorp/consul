package hosts

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestHostsParse(t *testing.T) {
	tests := []struct {
		inputFileRules      string
		shouldErr           bool
		expectedPath        string
		expectedOrigins     []string
		expectedFallthrough bool
	}{
		{
			`hosts
`,
			false, "/etc/hosts", nil, false,
		},
		{
			`hosts /tmp`,
			false, "/tmp", nil, false,
		},
		{
			`hosts /etc/hosts miek.nl.`,
			false, "/etc/hosts", []string{"miek.nl."}, false,
		},
		{
			`hosts /etc/hosts miek.nl. pun.gent.`,
			false, "/etc/hosts", []string{"miek.nl.", "pun.gent."}, false,
		},
		{
			`hosts {
				fallthrough
			}`,
			false, "/etc/hosts", nil, true,
		},
		{
			`hosts /tmp {
				fallthrough
			}`,
			false, "/tmp", nil, true,
		},
		{
			`hosts /etc/hosts miek.nl. {
				fallthrough
			}`,
			false, "/etc/hosts", []string{"miek.nl."}, true,
		},
		{
			`hosts /etc/hosts miek.nl 10.0.0.9/8 {
				fallthrough
			}`,
			false, "/etc/hosts", []string{"miek.nl.", "10.in-addr.arpa."}, true,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		h, err := hostsParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		} else if !test.shouldErr {
			if h.path != test.expectedPath {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedPath, h.path)
			}
		} else {
			if h.Fallthrough != test.expectedFallthrough {
				t.Fatalf("Test %d expected fallthrough of %v, got %v", i, test.expectedFallthrough, h.Fallthrough)
			}
			if len(h.Origins) != len(test.expectedOrigins) {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedOrigins, h.Origins)
			}
			for j, name := range test.expectedOrigins {
				if h.Origins[j] != name {
					t.Fatalf("Test %d expected %v for %d th zone, got %v", i, name, j, h.Origins[j])
				}
			}
		}
	}
}
