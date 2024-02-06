// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package version

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/stretchr/testify/require"
)

func TestFormat(t *testing.T) {
	buildDate, _ := time.Parse(time.RFC3339, "2022-06-01T13:18:45Z")
	info := VersionInfo{
		HumanVersion: "1.99.3-beta1",
		Version:      "1.99.3",
		Prerelease:   "beta1",
		Revision:     "5e5dbedd47a5f875b60e241c5555a9caab595246",
		BuildDate:    buildDate,
		RPC: RPCVersionInfo{
			Default: 2,
			Min:     1,
			Max:     3,
		},
	}

	formatters := map[string]Formatter{
		"pretty": newPrettyFormatter(),
		// the JSON formatter ignores the showMeta
		"json": newJSONFormatter(),
	}

	for fmtName, formatter := range formatters {
		t.Run(fmtName, func(t *testing.T) {
			actual, err := formatter.Format(&info)
			require.NoError(t, err)

			gName := fmt.Sprintf("%s", fmtName)

			expected := golden.Get(t, actual, gName)
			require.Equal(t, expected, actual)
		})
	}
}
