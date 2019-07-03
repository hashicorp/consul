package auto

import (
	"testing"
	"time"

	"github.com/caddyserver/caddy"
)

func TestAutoParse(t *testing.T) {
	tests := []struct {
		inputFileRules         string
		shouldErr              bool
		expectedDirectory      string
		expectedTempl          string
		expectedRe             string
		expectedReloadInterval time.Duration
		expectedTo             []string
	}{
		{
			`auto example.org {
				directory /tmp
				transfer to 127.0.0.1
			}`,
			false, "/tmp", "${1}", `db\.(.*)`, 60 * time.Second, []string{"127.0.0.1:53"},
		},
		{
			`auto 10.0.0.0/24 {
				directory /tmp
			}`,
			false, "/tmp", "${1}", `db\.(.*)`, 60 * time.Second, nil,
		},
		{
			`auto {
				directory /tmp
				reload 0
			}`,
			false, "/tmp", "${1}", `db\.(.*)`, 0 * time.Second, nil,
		},
		{
			`auto {
				directory /tmp (.*) bliep
			}`,
			false, "/tmp", "bliep", `(.*)`, 60 * time.Second, nil,
		},
		{
			`auto {
				directory /tmp (.*) bliep
				reload 10s
			}`,
			false, "/tmp", "bliep", `(.*)`, 10 * time.Second, nil,
		},
		{
			`auto {
				directory /tmp (.*) bliep
				transfer to 127.0.0.1
				transfer to 127.0.0.2
			}`,
			false, "/tmp", "bliep", `(.*)`, 60 * time.Second, []string{"127.0.0.1:53", "127.0.0.2:53"},
		},
		// errors
		// NO_RELOAD has been deprecated.
		{
			`auto {
				directory /tmp
				no_reload
			}`,
			true, "/tmp", "${1}", `db\.(.*)`, 0 * time.Second, nil,
		},
		// TIMEOUT has been deprecated.
		{
			`auto {
				directory /tmp (.*) bliep 10
			}`,
			true, "/tmp", "bliep", `(.*)`, 10 * time.Second, nil,
		},
		// no template specified.
		{
			`auto {
				directory /tmp (.*)
			}`,
			true, "/tmp", "", `(.*)`, 60 * time.Second, nil,
		},
		// no directory specified.
		{
			`auto example.org {
				directory
			}`,
			true, "", "${1}", `db\.(.*)`, 60 * time.Second, nil,
		},
		// illegal REGEXP.
		{
			`auto example.org {
				directory /tmp * {1}
			}`,
			true, "/tmp", "${1}", ``, 60 * time.Second, nil,
		},
		// unexpected argument.
		{
			`auto example.org {
				directory /tmp (.*) {1} aa
			}`,
			true, "/tmp", "${1}", ``, 60 * time.Second, nil,
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputFileRules)
		a, err := autoParse(c)

		if err == nil && test.shouldErr {
			t.Fatalf("Test %d expected errors, but got no error", i)
		} else if err != nil && !test.shouldErr {
			t.Fatalf("Test %d expected no errors, but got '%v'", i, err)
		} else if !test.shouldErr {
			if a.loader.directory != test.expectedDirectory {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedDirectory, a.loader.directory)
			}
			if a.loader.template != test.expectedTempl {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedTempl, a.loader.template)
			}
			if a.loader.re.String() != test.expectedRe {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedRe, a.loader.re)
			}
			if a.loader.ReloadInterval != test.expectedReloadInterval {
				t.Fatalf("Test %d expected %v, got %v", i, test.expectedReloadInterval, a.loader.ReloadInterval)
			}
			if test.expectedTo != nil {
				for j, got := range a.loader.transferTo {
					if got != test.expectedTo[j] {
						t.Fatalf("Test %d expected %v, got %v", i, test.expectedTo[j], got)
					}
				}
			}
		}
	}
}
