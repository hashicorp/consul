// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package configentry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestSortSlice(t *testing.T) {
	newDefaults := func(name, protocol string) *structs.ServiceConfigEntry {
		return &structs.ServiceConfigEntry{
			Kind:     structs.ServiceDefaults,
			Name:     name,
			Protocol: protocol,
		}
	}
	newResolver := func(name string, timeout time.Duration) *structs.ServiceResolverConfigEntry {
		return &structs.ServiceResolverConfigEntry{
			Kind:           structs.ServiceResolver,
			Name:           name,
			ConnectTimeout: timeout,
		}
	}

	type testcase struct {
		configs []structs.ConfigEntry
		expect  []structs.ConfigEntry
	}

	cases := map[string]testcase{
		"none": {},
		"one": {
			configs: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
			},
		},
		"just kinds": {
			configs: []structs.ConfigEntry{
				newResolver("web", 33*time.Second),
				newDefaults("web", "grpc"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("web", "grpc"),
				newResolver("web", 33*time.Second),
			},
		},
		"just names": {
			configs: []structs.ConfigEntry{
				newDefaults("db", "grpc"),
				newDefaults("api", "http2"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("api", "http2"),
				newDefaults("db", "grpc"),
			},
		},
		"all": {
			configs: []structs.ConfigEntry{
				newResolver("web", 33*time.Second),
				newDefaults("web", "grpc"),
				newDefaults("db", "grpc"),
				newDefaults("api", "http2"),
			},
			expect: []structs.ConfigEntry{
				newDefaults("api", "http2"),
				newDefaults("db", "grpc"),
				newDefaults("web", "grpc"),
				newResolver("web", 33*time.Second),
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			SortSlice(tc.configs)
			require.Equal(t, tc.expect, tc.configs)
			// and it should be stable
			SortSlice(tc.configs)
			require.Equal(t, tc.expect, tc.configs)
		})
	}
}
