// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAgentStructs_CheckTypes(t *testing.T) {
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
		t.Run(tc.desc, func(t *testing.T) {
			svc.Check = *tc.in
			checks, err := svc.CheckTypes()
			require.Error(t, err, tc.err.Error())
			require.Len(t, checks, 0)
		})

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
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			service := TestServiceDefinition(t)
			tc.Modify(service)

			err := service.Validate()
			if tc.Err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
			}
		})
	}
}
