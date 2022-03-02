package logtest

import (
	"bufio"
	"fmt"
	"strings"
)

type TestingT interface {
	Helper()
	Fatalf(format string, args ...interface{})
}

type Expectation interface {
	Match(line string) bool
}

type substringExpectation struct {
	Substrings []string
}

func (p substringExpectation) Match(line string) bool {
	for _, sub := range p.Substrings {
		if !strings.Contains(line, sub) {
			return false
		}
	}
	return true
}

func (p substringExpectation) String() string {
	return fmt.Sprintf("contains substrings: %v", strings.Join(p.Substrings, ", "))
}

func Line(patterns ...string) Expectation {
	return substringExpectation{Substrings: patterns}
}

// Assert scans the log and tries to match each Expectation to a line in the log.
// Expectations are matched in order, so if one is not found, the rest will be
// ignored.
// Assert fails the test if any of the expectations are not met.
func Assert(t TestingT, log string, expectations ...Expectation) {
	t.Helper()

	if len(expectations) == 0 {
		t.Fatalf("no expectations")
	}

	scan := bufio.NewScanner(strings.NewReader(log))
	var count int
	var index int

	for scan.Scan() && index < len(expectations) {
		count++
		line := scan.Text()
		if expectations[index].Match(line) {
			index++
		}
	}
	if scan.Err() != nil {
		t.Fatalf("scanning the log failed :%v", scan.Err())
	}
	if index != len(expectations) {
		t.Fatalf("scanned %d lines, but did not find one that matched expectation %d: %v",
			count,
			index+1,
			expectations[index])
	}
}
