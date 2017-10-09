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

	// Does not return empty checks
	svc.Checks = append(svc.Checks, &CheckType{})
	checks, err := svc.CheckTypes()
	if err != nil {
		t.Fatalf("Got error:%v", err)
	}

	if len(checks) != 4 {
		t.Fatalf("bad: %#v", svc)
	}

	//Error on invalid interval
	svc.Checks = append(svc.Checks, &CheckType{
		HTTP: "http://foo/baz",
	})

	checks, err = svc.CheckTypes()
	expectedErr := fmt.Errorf("invalid check definition:Interval must be >0")
	verify.Values(t, "", expectedErr.Error(), err.Error())

	if len(checks) != 0 {
		t.Fatalf("bad: %#v", svc)
	}

	//Error on invalid ttl
	svc.Checks = []*CheckType{}
	svc.Checks = append(svc.Checks, &CheckType{
		TTL: -1,
	})

	checks, err = svc.CheckTypes()
	expectedErr = fmt.Errorf("invalid check definition:TTL must be >0")
	verify.Values(t, "", expectedErr.Error(), err.Error())

	if len(checks) != 0 {
		t.Fatalf("bad: %#v", svc)
	}
}
