package httpproxy

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupChaos(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedFrom       string // expected from.
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// ok
		{
			`httpproxy . dns.google.com`, false, "", "",
		},
		{
			`httpproxy . dns.google.com {
				upstream 8.8.8.8:53
			}`, false, "", "",
		},
		{
			`httpproxy . dns.google.com {
				upstream resolv.conf
			}`, false, "", "",
		},
		// fail
		{
			`httpproxy`, true, "", "Wrong argument count or unexpected line ending after",
		},
		{
			`httpproxy . wns.google.com`, true, "", "unknown http proxy",
		},
	}

	// Write fake resolv.conf for test
	err := ioutil.WriteFile("resolv.conf", []byte("nameserver 127.0.0.1\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to write test resolv.conf")
	}
	defer os.Remove("resolv.conf")

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		_, err := httpproxyParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			t.Logf("%q", err)
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
		}
	}
}
