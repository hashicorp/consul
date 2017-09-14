package etcd

import (
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupEtcd(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedPath       string
		expectedEndpoint   string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		{
			`etcd`, false, "skydns", "http://localhost:2379", "",
		},
		{
			`etcd skydns.local {
	endpoint localhost:300
}
`, false, "skydns", "localhost:300", "",
		},
		// negative
		{
			`etcd {
	endpoints localhost:300
}
`, true, "", "", "unknown property 'endpoints'",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		etcd, _ /*stubzones*/, err := etcdParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
				continue
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
				continue
			}
		}

		if !test.shouldErr && etcd.PathPrefix != test.expectedPath {
			t.Errorf("Etcd not correctly set for input %s. Expected: %s, actual: %s", test.input, test.expectedPath, etcd.PathPrefix)
		}
		if !test.shouldErr && etcd.endpoints[0] != test.expectedEndpoint { // only checks the first
			t.Errorf("Etcd not correctly set for input %s. Expected: '%s', actual: '%s'", test.input, test.expectedEndpoint, etcd.endpoints[0])
		}
	}
}
