package server_details

import (
	"testing"
)

func TestServerDetails_Key_Equal(t *testing.T) {
	tests := []struct {
		name  string
		k1    *Key
		k2    *Key
		equal bool
	}{
		{
			name: "Key equality",
			k1: &Key{
				name: "s1",
			},
			k2: &Key{
				name: "s1",
			},
			equal: true,
		},
		{
			name: "Key Inequality",
			k1: &Key{
				name: "s1",
			},
			k2: &Key{
				name: "s2",
			},
			equal: false,
		},
	}

	for _, test := range tests {
		if test.k1.Equal(test.k2) != test.equal {
			t.Errorf("Expected a %v result from test %s", test.equal, test.name)
		}

		// Test Key to make sure it actually works as a key
		m := make(map[Key]bool)
		m[*test.k1] = true
		if _, found := m[*test.k2]; found != test.equal {
			t.Errorf("Expected a %v result from map test %s", test.equal, test.name)
		}
	}
}

func TestServerDetails_Key(t *testing.T) {
	tests := []struct {
		name  string
		sd    *ServerDetails
		k     *Key
		equal bool
	}{
		{
			name: "Key equality",
			sd: &ServerDetails{
				Name: "s1",
			},
			k: &Key{
				name: "s1",
			},
			equal: true,
		},
		{
			name: "Key inequality",
			sd: &ServerDetails{
				Name: "s1",
			},
			k: &Key{
				name: "s2",
			},
			equal: false,
		},
	}

	for _, test := range tests {
		if test.k.Equal(test.sd.Key()) != test.equal {
			t.Errorf("Expected a %v result from test %s", test.equal, test.name)
		}
	}
}
