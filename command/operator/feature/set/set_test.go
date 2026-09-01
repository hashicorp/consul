// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package set

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseBoolValue(t *testing.T) {
	cases := map[string]struct {
		input   string
		want    bool
		wantErr bool
	}{
		"enabled":  {input: "enabled", want: true},
		"disabled": {input: "disabled", want: false},
		"true":     {input: "true", want: true},
		"false":    {input: "false", want: false},
		"1":        {input: "1", want: true},
		"0":        {input: "0", want: false},
		"invalid":  {input: "yes", wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseBoolValue(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseInterspersed(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantPos []string
		wantCAS uint64
	}{
		{
			name:    "flags before positionals",
			args:    []string{"-cas=42", "my-feature", "enabled"},
			wantPos: []string{"my-feature", "enabled"},
			wantCAS: 42,
		},
		{
			name:    "flags after positionals",
			args:    []string{"my-feature", "enabled", "-cas=42"},
			wantPos: []string{"my-feature", "enabled"},
			wantCAS: 42,
		},
		{
			name:    "flags between positionals",
			args:    []string{"my-feature", "-cas=42", "enabled"},
			wantPos: []string{"my-feature", "enabled"},
			wantCAS: 42,
		},
		{
			name:    "no flags",
			args:    []string{"my-feature", "enabled"},
			wantPos: []string{"my-feature", "enabled"},
			wantCAS: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			var cas uint64
			fs.Uint64Var(&cas, "cas", 0, "")

			pos, err := parseInterspersed(fs, tc.args)
			require.NoError(t, err)
			require.Equal(t, tc.wantPos, pos)
			require.Equal(t, tc.wantCAS, cas)
		})
	}
}
