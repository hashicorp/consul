package bufsize

import (
	"strings"
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetupBufsize(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedData       int
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		{`bufsize`, false, 512, ""},
		{`bufsize "1232"`, false, 1232, ""},
		{`bufsize "5000"`, true, -1, "plugin"},
		{`bufsize "512 512"`, true, -1, "plugin"},
		{`bufsize "abc123"`, true, -1, "plugin"},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		bufsize, err := parse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Error found for input %s. Error: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
		}

		if !test.shouldErr && bufsize != test.expectedData {
			t.Errorf("Test %d: Bufsize not correctly set for input %s. Expected: %d, actual: %d", i, test.input, test.expectedData, bufsize)
		}
	}
}
