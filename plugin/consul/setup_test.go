// +build consul

package consul

import (
	"strings"
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetupConsul(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedPath       string
		address            string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		{
			`consul`, false, "skydns", "http://localhost:8500", "",
		},
		{
			`consul {
	address http://localhost:8500

}`, false, "skydns", "http://localhost:8500", "",
		},
		{
			`consul skydns.local {
	address localhost:300
}
`, false, "skydns", "localhost:300", "",
		},
		// negative
		{
			`consul {
	addresss localhost:300
}
`, true, "", "", "unknown property 'addresss'",
		},
		// with valid credentials
		{
			`consul {
			address http://localhost:8500
		}
			`, false, "skydns", "http://localhost:8500", "",
		},

		// with credentials, missing username and  password
		{
			`consul {
			address

		}
			`, true, "skydns", "http://localhost:8500", "Wrong argument count",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		consul, err := consulParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
				continue
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err.Error(), test.input)
				continue
			}
		}

		if !test.shouldErr && consul.PathPrefix != test.expectedPath {
			t.Errorf("Etcd not correctly set for input %s. Expected: %s, actual: %s", test.input, test.expectedPath, consul.PathPrefix)
		}

	}
}
