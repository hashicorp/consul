package consul

import (
	"fmt"
	"testing"
	"time"
)

const (
	day  = time.Hour * 24
	year = day * 365
)

func TestExpiresSoon(t *testing.T) {
	// ExpiresSoon() should return true if 'after' is <= 28 days
	// OR if before+after is < 70 days and after <= 40% of that
	testCases := []struct {
		name          string
		before, after time.Duration
		expiresSoon   bool
	}{
		{name: "base-pass", before: year, after: year, expiresSoon: false},
		{name: "base-expire", before: year, after: (day * 27), expiresSoon: true},
		{name: "expires", before: (day * 50), after: (day * 20), expiresSoon: true},
		{name: "passes", before: (day * 20), after: (day * 50), expiresSoon: false},
		{name: "just-expires", before: (day * 43), after: (day * 27), expiresSoon: true},
		{name: "just-passes", before: (day * 27), after: (day * 43), expiresSoon: false},
		{name: "40%-expire", before: (day * 20), after: (day * 10), expiresSoon: true},
		{name: "40%-pass", before: (day * 18), after: (day * 12), expiresSoon: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Print(tc.name + ": ")
			if expiresSoon(tc.before, tc.after) != tc.expiresSoon {
				t.Errorf("test case failed, should return `%t`", tc.expiresSoon)
			}
		})
	}
}
