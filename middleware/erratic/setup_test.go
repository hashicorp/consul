package erratic

import (
	"testing"

	"github.com/mholt/caddy"
)

func TestSetupErratic(t *testing.T) {
	c := caddy.NewTestController("dns", `erratic {
		drop
	}`)
	if err := setupErratic(c); err != nil {
		t.Fatalf("Test 1, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `erratic`)
	if err := setupErratic(c); err != nil {
		t.Fatalf("Test 2, expected no errors, but got: %q", err)
	}

	c = caddy.NewTestController("dns", `erratic {
		drop -1
	}`)
	if err := setupErratic(c); err == nil {
		t.Fatalf("Test 4, expected errors, but got: %q", err)
	}
}

func TestParseErratic(t *testing.T) {
	tests := []struct {
		input     string
		shouldErr bool
		drop      uint64
		delay     uint64
	}{
		// oks
		{`erratic`, false, 2, 0},
		{`erratic {
			drop 2
			delay 3 1ms

		}`, false, 2, 3},
		// fails
		{`erratic {
			drop -1
		}`, true, 0, 0},
		{`erraric {
			drop 3
			delay 3 bla
		}`, true, 0, 0},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		e, err := parseErratic(c)
		if test.shouldErr && err == nil {
			t.Errorf("Test %v: Expected error but found nil", i)
			continue
		} else if !test.shouldErr && err != nil {
			t.Errorf("Test %v: Expected no error but found error: %v", i, err)
			continue
		}

		if test.shouldErr {
			continue
		}

		if test.delay != e.delay {
			t.Errorf("Test %v: Expected delay %d but found: %d", i, test.delay, e.delay)
		}

		if test.drop != e.drop {
			t.Errorf("Test %v: Expected drop %d but found: %d", i, test.drop, e.drop)
		}
	}
}
