package ready

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetupReady(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
	}{
		{`ready`, false},
		{`ready localhost:1234`, false},
		{`ready localhost:1234 b`, true},
		{`ready bla`, true},
		{`ready bla bla`, true},
	}

	for i, test := range tests {
		_, err := parse(caddy.NewTestController("dns", test.input))

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found none for input %s", i, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}
		}
	}
}
