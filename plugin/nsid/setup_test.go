package nsid

import (
	"os"
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupNsid(t *testing.T) {
	defaultNsid, err := os.Hostname()
	if err != nil {
		defaultNsid = "localhost"
	}
	tests := []struct {
		input              string
		shouldErr          bool
		expectedData       string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		{`nsid`, false, defaultNsid, ""},
		{`nsid "ps0"`, false, "ps0", ""},
		{`nsid "worker1"`, false, "worker1", ""},
		{`nsid "tf 2"`, false, "tf 2", ""},
		{`nsid
		nsid`, true, "", "plugin"},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		nsid, err := nsidParse(c)

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

		if !test.shouldErr && nsid != test.expectedData {
			t.Errorf("Nsid not correctly set for input %s. Expected: %s, actual: %s", test.input, test.expectedData, nsid)
		}
	}
}
