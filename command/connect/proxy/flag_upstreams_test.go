// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/connect/proxy"
)

func TestFlagUpstreams_impl(t *testing.T) {
	var _ flag.Value = new(FlagUpstreams)
}

func TestFlagUpstreams(t *testing.T) {
	cases := []struct {
		Name     string
		Input    []string
		Expected map[string]proxy.UpstreamConfig
		Error    string
	}{
		{
			"bad format",
			[]string{"foo"},
			nil,
			"should be name:addr",
		},

		{
			"port not int",
			[]string{"db:hello"},
			nil,
			"invalid syntax",
		},

		{
			"4 parts",
			[]string{"db:127.0.0.1:8181:foo"},
			nil,
			"invalid syntax",
		},

		{
			"single value",
			[]string{"db:8181"},
			map[string]proxy.UpstreamConfig{
				"db": {
					LocalBindPort:   8181,
					DestinationName: "db",
					DestinationType: "service",
				},
			},
			"",
		},

		{
			"single value prepared query",
			[]string{"db.query:8181"},
			map[string]proxy.UpstreamConfig{
				"db": {
					LocalBindPort:   8181,
					DestinationName: "db",
					DestinationType: "prepared_query",
				},
			},
			"",
		},

		{
			"invalid type",
			[]string{"db.bad:8181"},
			nil,
			"Upstream type",
		},

		{
			"address specified",
			[]string{"db:127.0.0.55:8181"},
			map[string]proxy.UpstreamConfig{
				"db": {
					LocalBindAddress: "127.0.0.55",
					LocalBindPort:    8181,
					DestinationName:  "db",
					DestinationType:  "service",
				},
			},
			"",
		},

		{
			"repeat value, overwrite",
			[]string{"db:8181", "db:8282"},
			map[string]proxy.UpstreamConfig{
				"db": {
					LocalBindPort:   8282,
					DestinationName: "db",
					DestinationType: "service",
				},
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			var actual map[string]proxy.UpstreamConfig
			f := (*FlagUpstreams)(&actual)

			var err error
			for _, input := range tc.Input {
				err = f.Set(input)
				// Note we only test the last error. This could make some
				// test failures confusing but it shouldn't be too bad.
			}
			if tc.Error != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.Error)
				return
			}

			require.Equal(t, tc.Expected, actual)
		})
	}
}
