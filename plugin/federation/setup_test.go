package federation

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input            string
		shouldErr        bool
		expectedLen      int
		expectedNameZone []string // contains only entry for now
	}{
		// ok
		{`federation {
			prod prod.example.org
		}`, false, 1, []string{"prod", "prod.example.org."}},

		{`federation {
			staging staging.example.org
			prod prod.example.org
		}`, false, 2, []string{"prod", "prod.example.org."}},
		{`federation {
			staging staging.example.org
			prod prod.example.org
		}`, false, 2, []string{"staging", "staging.example.org."}},
		{`federation example.com {
			staging staging.example.org
			prod prod.example.org
		}`, false, 2, []string{"staging", "staging.example.org."}},
		// errors
		{`federation {
		}`, true, 0, []string{}},
		{`federation {
			staging
		}`, true, 0, []string{}},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		fed, err := federationParse(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if x := len(fed.f); x != test.expectedLen {
			t.Errorf("Test %v: Expected map length of %d, got: %d", i, test.expectedLen, x)
		}
		if x, ok := fed.f[test.expectedNameZone[0]]; !ok {
			t.Errorf("Test %v: Expected name for %s, got nothing", i, test.expectedNameZone[0])
		} else {
			if x != test.expectedNameZone[1] {
				t.Errorf("Test %v: Expected zone: %s, got %s", i, test.expectedNameZone[1], x)
			}
		}
	}
}
