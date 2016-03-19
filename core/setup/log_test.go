package setup

import (
	"testing"

	"github.com/miekg/coredns/middleware"
	corednslog "github.com/miekg/coredns/middleware/log"
)

func TestLog(t *testing.T) {

	c := NewTestController(`log`)

	mid, err := Log(c)

	if err != nil {
		t.Errorf("Expected no errors, got: %v", err)
	}

	if mid == nil {
		t.Fatal("Expected middleware, was nil instead")
	}

	handler := mid(EmptyNext)
	myHandler, ok := handler.(corednslog.Logger)

	if !ok {
		t.Fatalf("Expected handler to be type Logger, got: %#v", handler)
	}

	if myHandler.Rules[0].NameScope != "." {
		t.Errorf("Expected . as the default NameScope")
	}
	if myHandler.Rules[0].OutputFile != corednslog.DefaultLogFilename {
		t.Errorf("Expected %s as the default OutputFile", corednslog.DefaultLogFilename)
	}
	if myHandler.Rules[0].Format != corednslog.DefaultLogFormat {
		t.Errorf("Expected %s as the default Log Format", corednslog.DefaultLogFormat)
	}
	if myHandler.Rules[0].Roller != nil {
		t.Errorf("Expected Roller to be nil, got: %v", *myHandler.Rules[0].Roller)
	}
	if !SameNext(myHandler.Next, EmptyNext) {
		t.Error("'Next' field of handler was not set properly")
	}

}

func TestLogParse(t *testing.T) {
	tests := []struct {
		inputLogRules    string
		shouldErr        bool
		expectedLogRules []corednslog.Rule
	}{
		{`log`, false, []corednslog.Rule{{
			NameScope:  ".",
			OutputFile: corednslog.DefaultLogFilename,
			Format:     corednslog.DefaultLogFormat,
		}}},
		{`log log.txt`, false, []corednslog.Rule{{
			NameScope:  ".",
			OutputFile: "log.txt",
			Format:     corednslog.DefaultLogFormat,
		}}},
		{`log example.org log.txt`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     corednslog.DefaultLogFormat,
		}}},
		{`log example.org. stdout`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "stdout",
			Format:     corednslog.DefaultLogFormat,
		}}},
		{`log example.org log.txt {common}`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     corednslog.CommonLogFormat,
		}}},
		{`log example.org accesslog.txt {combined}`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "accesslog.txt",
			Format:     corednslog.CombinedLogFormat,
		}}},
		{`log example.org. log.txt
		  log example.net accesslog.txt {combined}`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     corednslog.DefaultLogFormat,
		}, {
			NameScope:  "example.net.",
			OutputFile: "accesslog.txt",
			Format:     corednslog.CombinedLogFormat,
		}}},
		{`log example.org stdout {host}
		  log example.org log.txt {when}`, false, []corednslog.Rule{{
			NameScope:  "example.org.",
			OutputFile: "stdout",
			Format:     "{host}",
		}, {
			NameScope:  "example.org.",
			OutputFile: "log.txt",
			Format:     "{when}",
		}}},
		{`log access.log { rotate { size 2 age 10 keep 3 } }`, false, []corednslog.Rule{{
			NameScope:  ".",
			OutputFile: "access.log",
			Format:     corednslog.DefaultLogFormat,
			Roller: &middleware.LogRoller{
				MaxSize:    2,
				MaxAge:     10,
				MaxBackups: 3,
				LocalTime:  true,
			},
		}}},
	}
	for i, test := range tests {
		c := NewTestController(test.inputLogRules)
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
