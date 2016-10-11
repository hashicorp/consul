package secondary

import (
	"testing"

	"github.com/mholt/caddy"
)

// TODO(miek): this only check the syntax.
func TestSecondaryParse(t *testing.T) {
	tests := []struct {
		inputFileRules string
		shouldErr      bool
		transferFrom   string
	}{
		{
			`secondary`,
			false, // TODO(miek): should actually be true, because without transfer lines this does not make sense
			"",
		},
		{
			`secondary {
				transfer from 127.0.0.1
				transfer to 127.0.0.1
			}`,
			false,
			"",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		_, err := secondaryParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		}
	}
}
