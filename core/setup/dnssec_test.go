package setup

import (
	"strings"
	"testing"
)

func TestDnssec(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedZones      []string
		expectedKeys       []string
		expectedErrContent string
	}{
		{
			`dnssec`, false, nil, nil, "",
		},
		{
			`dnssec miek.nl`, false, []string{"miek.nl."}, nil, "",
		},
	}

	for i, test := range tests {
		c := NewTestController(test.input)
		zones, keys, err := dnssecParse(c)

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
		if !test.shouldErr {
			for i, z := range test.expectedZones {
				if zones[i] != z {
					t.Errorf("Dnssec not correctly set for input %s. Expected: %s, actual: %s", test.input, z, zones[i])
				}
			}
			for i, k := range test.expectedKeys {
				if k != keys[i].K.Header().Name {
					t.Errorf("Dnssec not correctly set for input %s. Expected: '%s', actual: '%s'", test.input, k, keys[i].K.Header().Name)
				}
			}
		}
	}
}
