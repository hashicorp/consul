// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/testing/golden"
)

func TestErrorStrings(t *testing.T) {
	type testCase struct {
		err      error
		expected string
	}

	cases := map[string]error{
		"errInvalidWorkloadHostFormat": errInvalidWorkloadHostFormat{
			Host: "-foo-bar-",
		},
		"errInvalidNodeHostFormat": errInvalidNodeHostFormat{
			Host: "unix:///node.sock",
		},
		"errInvalidPortReference": errInvalidPortReference{
			Name: "http",
		},
		"errVirtualPortReused": errVirtualPortReused{
			Index: 3,
			Value: 8080,
		},
		"errTooMuchMesh": errTooMuchMesh{
			Ports: []string{"http", "grpc"},
		},
		"errInvalidEndpointsOwnerName": errInvalidEndpointsOwnerName{
			Name:      "foo",
			OwnerName: "bar",
		},
		"errNotDNSLabel":                errNotDNSLabel,
		"errNotIPAddress":               errNotIPAddress,
		"errUnixSocketMultiport":        errUnixSocketMultiport,
		"errInvalidPhysicalPort":        errInvalidPhysicalPort,
		"errInvalidVirtualPort":         errInvalidVirtualPort,
		"errDNSWarningWeightOutOfRange": errDNSWarningWeightOutOfRange,
		"errDNSPassingWeightOutOfRange": errDNSPassingWeightOutOfRange,
		"errLocalityZoneNoRegion":       errLocalityZoneNoRegion,
	}

	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			golden.Get(t, err.Error(), name)
		})
	}
}
