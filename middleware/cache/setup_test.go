package cache

import (
	"testing"
	"time"

	"github.com/mholt/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input        string
		shouldErr    bool
		expectedNcap int
		expectedPcap int
		expectedNttl time.Duration
		expectedPttl time.Duration
	}{
		{`cache`, false, defaultCap, defaultCap, maxNTTL, maxTTL},
		{`cache {}`, false, defaultCap, defaultCap, maxNTTL, maxTTL},
		{`cache example.nl {
			success 10
		}`, false, defaultCap, 10, maxNTTL, maxTTL},
		{`cache example.nl {
			success 10
			denial 10 15
		}`, false, 10, 10, 15 * time.Second, maxTTL},
		{`cache 25 example.nl {
			success 10
			denial 10 15
		}`, false, 10, 10, 15 * time.Second, 25 * time.Second},
		{`cache aaa example.nl`, false, defaultCap, defaultCap, maxNTTL, maxTTL},

		// fails
		{`cache example.nl {
			success
			denial 10 15
		}`, true, defaultCap, defaultCap, maxTTL, maxTTL},
		{`cache example.nl {
			success 15
			denial aaa
		}`, true, defaultCap, defaultCap, maxTTL, maxTTL},
		{`cache example.nl {
			positive 15
			negative aaa
		}`, true, defaultCap, defaultCap, maxTTL, maxTTL},
		{`cache 0 example.nl`, true, defaultCap, defaultCap, maxTTL, maxTTL},
		{`cache -1 example.nl`, true, defaultCap, defaultCap, maxTTL, maxTTL},
		{`cache 1 example.nl {
			positive 0
		}`, true, defaultCap, defaultCap, maxTTL, maxTTL},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		ca, err := cacheParse(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}
		if test.shouldErr && err != nil {
			continue
		}

		if ca.ncap != test.expectedNcap {
			t.Errorf("Test %v: Expected ncap %v but found: %v", i, test.expectedNcap, ca.ncap)
		}
		if ca.pcap != test.expectedPcap {
			t.Errorf("Test %v: Expected pcap %v but found: %v", i, test.expectedPcap, ca.pcap)
		}
		if ca.nttl != test.expectedNttl {
			t.Errorf("Test %v: Expected nttl %v but found: %v", i, test.expectedNttl, ca.nttl)
		}
		if ca.pttl != test.expectedPttl {
			t.Errorf("Test %v: Expected pttl %v but found: %v", i, test.expectedPttl, ca.pttl)
		}
	}
}
