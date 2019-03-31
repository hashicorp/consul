package chaos

import (
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupChaos(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedVersion    string // expected version.
		expectedAuthor     string // expected author (string, although we get a slice).
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		{
			`chaos v2`, false, "v2", "", "",
		},
		{
			`chaos v3 "Miek Gieben"`, false, "v3", "Miek Gieben", "",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		version, authors, err := parse(c)

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

		if !test.shouldErr && version != test.expectedVersion {
			t.Errorf("Test %d: Chaos not correctly set for input %s. Expected: %s, actual: %s", i, test.input, test.expectedVersion, version)
		}
		if !test.shouldErr && authors != nil && test.expectedAuthor != "" {
			if authors[0] != test.expectedAuthor {
				t.Errorf("Test %d: Chaos not correctly set for input %s. Expected: '%s', actual: '%s'", i, test.input, test.expectedAuthor, authors[0])
			}
		}
	}
}
