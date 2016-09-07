package log

import (
	"testing"

	"github.com/miekg/coredns/middleware/pkg/roller"

	"github.com/mholt/caddy"
)

func TestLogParse(t *testing.T) {
	tests := []struct {
		inputLogRules    string
		shouldErr        bool
		expectedLogRules []Rule
	}{
		{`log`, false, []Rule{{
			NameScope:  ".",
			OutputFile: DefaultLogFilename,
			Format:     DefaultLogFormat,
		}}},
		{`log log.txt`, false, []Rule{{
			NameScope:  ".",
			OutputFile: "log.txt",
			Format:     DefaultLogFormat,
		}}},
		{`log example.org log.txt`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     DefaultLogFormat,
		}}},
		{`log example.org. stdout`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "stdout",
			Format:     DefaultLogFormat,
		}}},
		{`log example.org log.txt {common}`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     CommonLogFormat,
		}}},
		{`log example.org accesslog.txt {combined}`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "accesslog.txt",
			Format:     CombinedLogFormat,
		}}},
		{`log example.org. log.txt
		  log example.net accesslog.txt {combined}`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     DefaultLogFormat,
		}, {
			NameScope:  "example.net.",
			OutputFile: "accesslog.txt",
			Format:     CombinedLogFormat,
		}}},
		{`log example.org stdout {host}
		  log example.org log.txt {when}`, false, []Rule{{
			NameScope:  "example.org.",
			OutputFile: "stdout",
			Format:     "{host}",
		}, {
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     "{when}",
		}}},
		{`log access.log { rotate { size 2 age 10 keep 3 } }`, false, []Rule{{
			NameScope:  ".",
			OutputFile: "access.log",
			Format:     DefaultLogFormat,
			Roller: &roller.LogRoller{
				MaxSize:    2,
				MaxAge:     10,
				MaxBackups: 3,
				LocalTime:  true,
			},
		}}},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputLogRules)
		actualLogRules, err := logParse(c)

		if err == nil && test.shouldErr {
			t.Errorf("Test %d didn't error, but it should have", i)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d errored, but it shouldn't have; got '%v'", i, err)
		}
		if len(actualLogRules) != len(test.expectedLogRules) {
			t.Fatalf("Test %d expected %d no of Log rules, but got %d ",
				i, len(test.expectedLogRules), len(actualLogRules))
		}
		for j, actualLogRule := range actualLogRules {

			if actualLogRule.NameScope != test.expectedLogRules[j].NameScope {
				t.Errorf("Test %d expected %dth LogRule NameScope to be  %s  , but got %s",
					i, j, test.expectedLogRules[j].NameScope, actualLogRule.NameScope)
			}

			if actualLogRule.OutputFile != test.expectedLogRules[j].OutputFile {
				t.Errorf("Test %d expected %dth LogRule OutputFile to be  %s  , but got %s",
					i, j, test.expectedLogRules[j].OutputFile, actualLogRule.OutputFile)
			}

			if actualLogRule.Format != test.expectedLogRules[j].Format {
				t.Errorf("Test %d expected %dth LogRule Format to be  %s  , but got %s",
					i, j, test.expectedLogRules[j].Format, actualLogRule.Format)
			}
			if actualLogRule.Roller != nil && test.expectedLogRules[j].Roller == nil || actualLogRule.Roller == nil && test.expectedLogRules[j].Roller != nil {
				t.Fatalf("Test %d expected %dth LogRule Roller to be %v, but got %v",
					i, j, test.expectedLogRules[j].Roller, actualLogRule.Roller)
			}
			if actualLogRule.Roller != nil && test.expectedLogRules[j].Roller != nil {
				if actualLogRule.Roller.Filename != test.expectedLogRules[j].Roller.Filename {
					t.Fatalf("Test %d expected %dth LogRule Roller Filename to be %s, but got %s",
						i, j, test.expectedLogRules[j].Roller.Filename, actualLogRule.Roller.Filename)
				}
				if actualLogRule.Roller.MaxAge != test.expectedLogRules[j].Roller.MaxAge {
					t.Fatalf("Test %d expected %dth LogRule Roller MaxAge to be %d, but got %d",
						i, j, test.expectedLogRules[j].Roller.MaxAge, actualLogRule.Roller.MaxAge)
				}
				if actualLogRule.Roller.MaxBackups != test.expectedLogRules[j].Roller.MaxBackups {
					t.Fatalf("Test %d expected %dth LogRule Roller MaxBackups to be %d, but got %d",
						i, j, test.expectedLogRules[j].Roller.MaxBackups, actualLogRule.Roller.MaxBackups)
				}
				if actualLogRule.Roller.MaxSize != test.expectedLogRules[j].Roller.MaxSize {
					t.Fatalf("Test %d expected %dth LogRule Roller MaxSize to be %d, but got %d",
						i, j, test.expectedLogRules[j].Roller.MaxSize, actualLogRule.Roller.MaxSize)
				}
				if actualLogRule.Roller.LocalTime != test.expectedLogRules[j].Roller.LocalTime {
					t.Fatalf("Test %d expected %dth LogRule Roller LocalTime to be %t, but got %t",
						i, j, test.expectedLogRules[j].Roller.LocalTime, actualLogRule.Roller.LocalTime)
				}
			}
		}
	}

}
