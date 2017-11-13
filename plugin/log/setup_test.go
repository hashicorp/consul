package log

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/response"

	"github.com/mholt/caddy"
)

func TestLogParse(t *testing.T) {
	tests := []struct {
		inputLogRules    string
		shouldErr        bool
		expectedLogRules []Rule
	}{
		{`log`, false, []Rule{{
			NameScope: ".",
			Format:    DefaultLogFormat,
		}}},
		{`log example.org`, false, []Rule{{
			NameScope: "example.org.",
			Format:    DefaultLogFormat,
		}}},
		{`log example.org. {common}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    CommonLogFormat,
		}}},
		{`log example.org {combined}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    CombinedLogFormat,
		}}},
		{`log example.org.
		log example.net {combined}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    DefaultLogFormat,
		}, {
			NameScope: "example.net.",
			Format:    CombinedLogFormat,
		}}},
		{`log example.org {host}
			  log example.org {when}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    "{host}",
		}, {
			NameScope: "example.org.",
			Format:    "{when}",
		}}},

		{`log example.org {
				class all
			}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    CommonLogFormat,
			Class:     response.All,
		}}},
		{`log example.org {
			class denial
		}`, false, []Rule{{
			NameScope: "example.org.",
			Format:    CommonLogFormat,
			Class:     response.Denial,
		}}},
		{`log {
			class denial
		}`, false, []Rule{{
			NameScope: ".",
			Format:    CommonLogFormat,
			Class:     response.Denial,
		}}},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputLogRules)
		actualLogRules, err := logParse(c)

		if err == nil && test.shouldErr {
			t.Errorf("Test %d with input '%s' didn't error, but it should have", i, test.inputLogRules)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d with input '%s' errored, but it shouldn't have; got '%v'",
				i, test.inputLogRules, err)
		}
		if len(actualLogRules) != len(test.expectedLogRules) {
			t.Fatalf("Test %d expected %d no of Log rules, but got %d ",
				i, len(test.expectedLogRules), len(actualLogRules))
		}
		for j, actualLogRule := range actualLogRules {

			if actualLogRule.NameScope != test.expectedLogRules[j].NameScope {
				t.Errorf("Test %d expected %dth LogRule NameScope for '%s' to be  %s  , but got %s",
					i, j, test.inputLogRules, test.expectedLogRules[j].NameScope, actualLogRule.NameScope)
			}

			if actualLogRule.Format != test.expectedLogRules[j].Format {
				t.Errorf("Test %d expected %dth LogRule Format for '%s' to be  %s  , but got %s",
					i, j, test.inputLogRules, test.expectedLogRules[j].Format, actualLogRule.Format)
			}

			if actualLogRule.Class != test.expectedLogRules[j].Class {
				t.Errorf("Test %d expected %dth LogRule Class to be  %s  , but got %s",
					i, j, test.expectedLogRules[j].Class, actualLogRule.Class)
			}
		}
	}

}
