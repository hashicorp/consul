package structs

import (
	"fmt"
	"testing"
	"time"

	"github.com/pascaldekloe/goe/verify"
)

func TestAgentStructs_CheckTypes(t *testing.T) {
	t.Parallel()
	svc := new(ServiceDefinition)

	// Singular Check field works
	svc.Check = CheckType{
		Script:   "/foo/bar",
		Interval: 10 * time.Second,
	}

	// Returns HTTP checks
	svc.Checks = append(svc.Checks, &CheckType{
		HTTP:     "http://foo/bar",
		Interval: 10 * time.Second,
	})

	// Returns Script checks
	svc.Checks = append(svc.Checks, &CheckType{
		Script:   "/foo/bar",
		Interval: 10 * time.Second,
	})

	// Returns TTL checks
	svc.Checks = append(svc.Checks, &CheckType{
		TTL: 10 * time.Second,
	})

	// Validate checks
	cases := []struct {
		in   *CheckType
		err  error
		desc string
	}{
		{&CheckType{HTTP: "http://foo/baz"}, fmt.Errorf("Interval must be > 0"), "Missing interval"},
		{&CheckType{TTL: -1}, fmt.Errorf("TTL must be > 0"), "Negative TTL"},
	}
	for _, tc := range cases {
		svc.Check = *tc.in
		checks, err := svc.CheckTypes()
		verify.Values(t, tc.desc, tc.err.Error(), err.Error())
		if len(checks) != 0 {
			t.Fatalf("bad: %#v", svc)
		}
	}
}
