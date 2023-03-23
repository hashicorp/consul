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
	// OR if 40% of lifetime if it is less than 28 days
	testCases := []struct {
		name               string
		lifetime, notAfter time.Duration
		expiresSoon        bool
	}{
		{name: "base-pass", lifetime: year, notAfter: year, expiresSoon: false},
		{name: "base-expire", lifetime: year, notAfter: (day * 27), expiresSoon: true},
		{name: "expires", lifetime: (day * 70), notAfter: (day * 20), expiresSoon: true},
		{name: "passes", lifetime: (day * 70), notAfter: (day * 50), expiresSoon: false},
		{name: "just-expires", lifetime: (day * 70), notAfter: (day * 27), expiresSoon: true},
		{name: "just-passes", lifetime: (day * 70), notAfter: (day * 43), expiresSoon: false},
		{name: "40%-expire", lifetime: (day * 30), notAfter: (day * 10), expiresSoon: true},
		{name: "40%-pass", lifetime: (day * 30), notAfter: (day * 12), expiresSoon: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Print(tc.name + ": ")
			if expiresSoon(tc.lifetime, tc.notAfter) != tc.expiresSoon {
				t.Errorf("test case failed, should return `%t`", tc.expiresSoon)
			}
		})
	}
}
