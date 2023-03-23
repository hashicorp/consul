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
	// ExpiresSoon() should return true if 'notAfter' is <= 28 days
	// OR if notBefore+notAfter is < 70 days and notAfter <= 40% of that
	testCases := []struct {
		name                string
		notBefore, notAfter time.Duration
		expiresSoon         bool
	}{
		{name: "base-pass", notBefore: year, notAfter: year, expiresSoon: false},
		{name: "base-expire", notBefore: year, notAfter: (day * 27), expiresSoon: true},
		{name: "expires", notBefore: (day * 50), notAfter: (day * 20), expiresSoon: true},
		{name: "passes", notBefore: (day * 20), notAfter: (day * 50), expiresSoon: false},
		{name: "just-expires", notBefore: (day * 43), notAfter: (day * 27), expiresSoon: true},
		{name: "just-passes", notBefore: (day * 27), notAfter: (day * 43), expiresSoon: false},
		{name: "40%-expire", notBefore: (day * 20), notAfter: (day * 10), expiresSoon: true},
		{name: "40%-pass", notBefore: (day * 18), notAfter: (day * 12), expiresSoon: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Print(tc.name + ": ")
			if expiresSoon(tc.notBefore, tc.notAfter) != tc.expiresSoon {
				t.Errorf("test case failed, should return `%t`", tc.expiresSoon)
			}
		})
	}
}
