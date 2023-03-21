// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inspect

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormat(t *testing.T) {
	m := []typeStats{{
		Name:  "msg",
		Sum:   1,
		Count: 2,
	}}
	mkv := []typeStats{{
		Name:  "msgKV",
		Sum:   1,
		Count: 2,
	}}
	info := OutputFormat{
		Meta: &MetadataInfo{
			ID:      "one",
			Size:    2,
			Index:   3,
			Term:    4,
			Version: 1,
		},
		Stats:       m,
		StatsKV:     mkv,
		TotalSize:   1,
		TotalSizeKV: 1,
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

			expected := golden(t, gName, actual)
			require.Equal(t, expected, actual)
		})
	}
}
