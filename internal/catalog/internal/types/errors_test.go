// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// update allows golden files to be updated based on the current output.
var update = flag.Bool("update", false, "update golden files")

func goldenError(t *testing.T, name string, actual string) {
	t.Helper()

	fpath := filepath.Join("testdata", name+".golden")

	if *update {
		require.NoError(t, os.WriteFile(fpath, []byte(actual), 0644))
	} else {
		expected, err := os.ReadFile(fpath)
		require.NoError(t, err)
		require.Equal(t, string(expected), actual)
	}
}

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
			goldenError(t, name, err.Error())
		})
	}
}
