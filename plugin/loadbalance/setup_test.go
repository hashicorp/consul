package loadbalance

import (
	"strings"
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedPolicy     string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		{`loadbalance`, false, "round_robin", ""},
		{`loadbalance round_robin`, false, "round_robin", ""},
		// negative
		{`loadbalance fleeb`, true, "", "unknown policy"},
		{`loadbalance a b`, true, "", "argument count or unexpected line"},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		err := parse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
		}
	}
}
