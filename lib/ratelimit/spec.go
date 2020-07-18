package ratelimit

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Spec defines how rate limit is expressed
type Spec struct {
	// burstSize defines the size of bursts for the period
	BurstSize int
	// Period of ticker
	Period time.Duration
}

// parseTimeUnit parse a timeunit, possibly without number
// in such case, 1 will be assumed.
func parseTimeUnit(unitStr string) (time.Duration, error) {
	u, err := time.ParseDuration(unitStr)
	if err != nil {
		u, err = time.ParseDuration(fmt.Sprintf("1%s", unitStr))
		if err != nil {
			return 0, fmt.Errorf("Cannot parse time unit %s", unitStr)
		}
	}
	return u, nil
}

// ParseRateLimit compute a value expressing a period as a frequency.
// It supports: 5/s or 5/2s with all golang Duration units.
// For now, there is not burst, so all calls will have the same duration
// between calls, but it might be possible to parse more cleverly, aka:
// 16/3s => Add 16 units every 3s
func ParseRateLimit(rateLimitStr string) (*Spec, error) {
	msgErr := "Spec must be calls / [unit], eg: 10/m, was %s: %s"
	spec := strings.Split(rateLimitStr, "/")
	if len(spec) != 2 {
		return nil, fmt.Errorf(msgErr, rateLimitStr, "Must have 1 slash")
	}
	unit, err := parseTimeUnit(strings.Trim(spec[1], " "))
	if err != nil {
		return nil, fmt.Errorf(msgErr, rateLimitStr, err.Error())
	}
	if unit < 1 {
		return nil, fmt.Errorf(msgErr, rateLimitStr, "value after slash MUST be positive")
	}
	val, err := strconv.ParseUint(strings.Trim(spec[0], " "), 10, 64)
	if err != nil {
		return nil, fmt.Errorf(msgErr, rateLimitStr, err.Error())
	}
	if val < 1 {
		return nil, fmt.Errorf(msgErr, rateLimitStr, "value before slash must be positive")
	}
	period := unit / time.Duration(val)
	return NewSpec(period), nil
}

// NewSpec builds a new Spec with default burst of 2.
func NewSpec(period time.Duration) *Spec {
	return &Spec{
		2,
		period,
	}
}
