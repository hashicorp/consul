package errors

import (
	"testing"

	"github.com/mholt/caddy"
	"github.com/miekg/coredns/middleware/pkg/roller"
)

func TestErrorsParse(t *testing.T) {
	tests := []struct {
		inputErrorsRules     string
		shouldErr            bool
		expectedErrorHandler ErrorHandler
	}{
		{`errors`, false, ErrorHandler{
			LogFile: "",
		}},
		{`errors errors.txt`, false, ErrorHandler{
			LogFile: "errors.txt",
		}},
		{`errors visible`, false, ErrorHandler{
			LogFile: "",
			Debug:   true,
		}},
		{`errors { log visible }`, false, ErrorHandler{
			LogFile: "",
			Debug:   true,
		}},
		{`errors { log errors.txt { size 2 age 10 keep 3 } }`, false, ErrorHandler{
			LogFile: "errors.txt",
			LogRoller: &roller.LogRoller{
				MaxSize:    2,
				MaxAge:     10,
				MaxBackups: 3,
				LocalTime:  true,
			},
		}},
		{`errors { log errors.txt {
            size 3
            age 11
            keep 5
        }
}`, false, ErrorHandler{
			LogFile: "errors.txt",
			LogRoller: &roller.LogRoller{
				MaxSize:    3,
				MaxAge:     11,
				MaxBackups: 5,
				LocalTime:  true,
			},
		}},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputErrorsRules)
		actualErrorsRule, err := errorsParse(c)

		if err == nil && test.shouldErr {
			t.Errorf("Test %d didn't error, but it should have", i)
		} else if err != nil && !test.shouldErr {
			t.Errorf("Test %d errored, but it shouldn't have; got '%v'", i, err)
		}
		if actualErrorsRule.LogFile != test.expectedErrorHandler.LogFile {
			t.Errorf("Test %d expected LogFile to be %s, but got %s",
				i, test.expectedErrorHandler.LogFile, actualErrorsRule.LogFile)
		}
		if actualErrorsRule.Debug != test.expectedErrorHandler.Debug {
			t.Errorf("Test %d expected Debug to be %v, but got %v",
				i, test.expectedErrorHandler.Debug, actualErrorsRule.Debug)
		}
		if actualErrorsRule.LogRoller != nil && test.expectedErrorHandler.LogRoller == nil || actualErrorsRule.LogRoller == nil && test.expectedErrorHandler.LogRoller != nil {
			t.Fatalf("Test %d expected LogRoller to be %v, but got %v",
				i, test.expectedErrorHandler.LogRoller, actualErrorsRule.LogRoller)
		}
		if actualErrorsRule.LogRoller != nil && test.expectedErrorHandler.LogRoller != nil {
			if actualErrorsRule.LogRoller.Filename != test.expectedErrorHandler.LogRoller.Filename {
				t.Fatalf("Test %d expected LogRoller Filename to be %s, but got %s",
					i, test.expectedErrorHandler.LogRoller.Filename, actualErrorsRule.LogRoller.Filename)
			}
			if actualErrorsRule.LogRoller.MaxAge != test.expectedErrorHandler.LogRoller.MaxAge {
				t.Fatalf("Test %d expected LogRoller MaxAge to be %d, but got %d",
					i, test.expectedErrorHandler.LogRoller.MaxAge, actualErrorsRule.LogRoller.MaxAge)
			}
			if actualErrorsRule.LogRoller.MaxBackups != test.expectedErrorHandler.LogRoller.MaxBackups {
				t.Fatalf("Test %d expected LogRoller MaxBackups to be %d, but got %d",
					i, test.expectedErrorHandler.LogRoller.MaxBackups, actualErrorsRule.LogRoller.MaxBackups)
			}
			if actualErrorsRule.LogRoller.MaxSize != test.expectedErrorHandler.LogRoller.MaxSize {
				t.Fatalf("Test %d expected LogRoller MaxSize to be %d, but got %d",
					i, test.expectedErrorHandler.LogRoller.MaxSize, actualErrorsRule.LogRoller.MaxSize)
			}
			if actualErrorsRule.LogRoller.LocalTime != test.expectedErrorHandler.LogRoller.LocalTime {
				t.Fatalf("Test %d expected LogRoller LocalTime to be %t, but got %t",
					i, test.expectedErrorHandler.LogRoller.LocalTime, actualErrorsRule.LogRoller.LocalTime)
			}
		}
	}
}
