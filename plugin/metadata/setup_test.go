package metadata

import (
	"reflect"
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input     string
		zones     []string
		shouldErr bool
	}{
		{"metadata", []string{}, false},
		{"metadata example.com.", []string{"example.com."}, false},
		{"metadata example.com. net.", []string{"example.com.", "net."}, false},

		{"metadata example.com. { some_param }", []string{}, true},
		{"metadata\nmetadata", []string{}, true},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		err := setup(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Setup call expected error but found none for input %s", i, test.input)
		}

		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: Setup call expected no error but found one for input %s. Error was: %v", i, test.input, err)
		}
	}
}

func TestSetupHealth(t *testing.T) {
	tests := []struct {
		input     string
		zones     []string
		shouldErr bool
	}{
		{"metadata", []string{}, false},
		{"metadata example.com.", []string{"example.com."}, false},
		{"metadata example.com. net.", []string{"example.com.", "net."}, false},

		{"metadata example.com. { some_param }", []string{}, true},
		{"metadata\nmetadata", []string{}, true},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		m, err := metadataParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found none for input %s", i, test.input)
		}

		if !test.shouldErr && err != nil {
			t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
		}

		if !test.shouldErr && err == nil {
			if !reflect.DeepEqual(test.zones, m.Zones) {
				t.Errorf("Test %d: Expected zones %s. Zones were: %v", i, test.zones, m.Zones)
			}
		}
	}
}
