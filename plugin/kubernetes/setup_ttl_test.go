package kubernetes

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestKubernetesParseTTL(t *testing.T) {
	tests := []struct {
		input       string // Corefile data as string
		expectedTTL uint32 // expected count of defined zones.
		shouldErr   bool
	}{
		{`kubernetes cluster.local {
			ttl 56
		}`, 56, false},
		{`kubernetes cluster.local`, defaultTTL, false},
		{`kubernetes cluster.local {
			ttl -1
		}`, 0, true},
		{`kubernetes cluster.local {
			ttl 3601
		}`, 0, true},
	}

	for i, tc := range tests {
		c := caddy.NewTestController("dns", tc.input)
		k, err := kubernetesParse(c)
		if err != nil && !tc.shouldErr {
			t.Fatalf("Test %d: Expected no error, got %q", i, err)
		}
		if err == nil && tc.shouldErr {
			t.Fatalf("Test %d: Expected error, got none", i)
		}
		if err != nil && tc.shouldErr {
			// input should error
			continue
		}

		if k.ttl != tc.expectedTTL {
			t.Errorf("Test %d: Expected TTl to be %d, got %d", i, tc.expectedTTL, k.ttl)
		}
	}
}
