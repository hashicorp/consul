package structs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/require"
)

func TestAgentStructs_CheckTypes(t *testing.T) {
	t.Parallel()
	svc := new(ServiceDefinition)

	// Singular Check field works
	svc.Check = CheckType{
		ScriptArgs: []string{"/foo/bar"},
		Interval:   10 * time.Second,
	}

	// Returns HTTP checks
	svc.Checks = append(svc.Checks, &CheckType{
		HTTP:     "http://foo/bar",
		Interval: 10 * time.Second,
	})

	// Returns Script checks
	svc.Checks = append(svc.Checks, &CheckType{
		ScriptArgs: []string{"/foo/bar"},
		Interval:   10 * time.Second,
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
		{&CheckType{HTTP: "http://foo/baz"}, fmt.Errorf("Interval must be > 0 for Script, HTTP, or TCP checks"), "Missing interval"},
		{&CheckType{TTL: -1}, fmt.Errorf("TTL must be > 0 for TTL checks"), "Negative TTL"},
		{&CheckType{TTL: 20 * time.Second, Interval: 10 * time.Second}, fmt.Errorf("Interval and TTL cannot both be specified"), "Interval and TTL both set"},
	}
	for _, tc := range cases {
		svc.Check = *tc.in
		checks, err := svc.CheckTypes()
		verify.Values(t, tc.desc, err.Error(), tc.err.Error())
		if len(checks) != 0 {
			t.Fatalf("bad: %#v", svc)
		}
	}
}

func TestServiceDefinitionValidate(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*ServiceDefinition)
		Err    string
	}{
		{
			"valid",
			func(x *ServiceDefinition) {},
			"",
		},

		{
			"managed proxy with a port set",
			func(x *ServiceDefinition) {
				x.Port = 8080
				x.Connect = &ServiceConnect{
					Proxy: &ServiceDefinitionConnectProxy{},
				}
			},
			"",
		},

		{
			"managed proxy with no port set",
			func(x *ServiceDefinition) {
				x.Connect = &ServiceConnect{
					Proxy: &ServiceDefinitionConnectProxy{},
				}
			},
			"must have a port",
		},

		{
			"managed proxy with native set",
			func(x *ServiceDefinition) {
				x.Port = 8080
				x.Connect = &ServiceConnect{
					Native: true,
					Proxy:  &ServiceDefinitionConnectProxy{},
				}
			},
			"may not have a proxy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)
			service := TestServiceDefinition(t)
			tc.Modify(service)

			err := service.Validate()
			t.Logf("error: %s", err)
			require.Equal(err != nil, tc.Err != "")
			if err == nil {
				return
			}

			require.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}
