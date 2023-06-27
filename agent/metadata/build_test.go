// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		desc string
		m    *serf.Member
		ver  *version.Version
		err  bool
	}{
		{
			"no version",
			&serf.Member{},
			nil,
			true,
		},
		{
			"bad version",
			&serf.Member{
				Tags: map[string]string{
					"build": "nope",
				},
			},
			nil,
			true,
		},
		{
			"good version",
			&serf.Member{
				Tags: map[string]string{
					"build": "0.8.5",
				},
			},
			version.Must(version.NewVersion("0.8.5")),
			false,
		},
		{
			"rc version",
			&serf.Member{
				Tags: map[string]string{
					"build": "0.9.3rc1:d62743c",
				},
			},
			version.Must(version.NewVersion("0.9.3")),
			false,
		},
		{
			"ent version",
			&serf.Member{
				Tags: map[string]string{
					"build": "0.9.3+ent:d62743c",
				},
			},
			version.Must(version.NewVersion("0.9.3")),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ver, err := Build(tt.m)
			gotErr := err != nil
			if wantErr := tt.err; gotErr != wantErr {
				t.Fatalf("got %v want %v", gotErr, wantErr)
			}
			require.Equal(t, tt.ver, ver)
		})
	}
}
