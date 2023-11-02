// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package extensioncommon

import (
	"testing"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/stretchr/testify/require"
)

func TestInsertHTTPFilter(t *testing.T) {
	cases := map[string]struct {
		inputFilters    []*envoy_http_v3.HttpFilter
		insertOptions   InsertOptions
		filterName      string
		expectedFilters []*envoy_http_v3.HttpFilter
		errStr          string
	}{
		"insert first": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertFirst},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "test.filter", "a", "b", "b", "b", "c"),
		},
		"insert last": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertLast},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "b", "b", "b", "c", "test.filter"),
		},
		"insert before first match": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertBeforeFirstMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "test.filter", "b", "b", "b", "c"),
		},
		"insert after first match": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertAfterFirstMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "b", "test.filter", "b", "b", "c"),
		},
		"insert before last match": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertBeforeLastMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "b", "b", "test.filter", "b", "c"),
		},
		"insert after last match": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertAfterLastMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "b", "b", "b", "test.filter", "c"),
		},
		"insert last after last match": {
			inputFilters:    makeHttpFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertAfterLastMatch, FilterName: "c"},
			filterName:      "test.filter",
			expectedFilters: makeHttpFilters(t, "a", "b", "b", "b", "c", "test.filter"),
		},
	}

	t.Parallel()
	for name, c := range cases {
		c := c

		t.Run(name, func(t *testing.T) {
			filters := []*envoy_listener_v3.Filter{makeHttpConMgr(t, c.inputFilters)}
			newFilter := &envoy_http_v3.HttpFilter{Name: c.filterName}
			obsFilters, err := InsertHTTPFilter(filters, newFilter, c.insertOptions)
			if c.errStr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errStr)
			} else {
				require.NoError(t, err)
				httpConMgr, idx, err := GetHTTPConnectionManager(obsFilters...)
				require.NoError(t, err)
				require.NotNil(t, httpConMgr)
				require.Equal(t, 0, idx)
				require.ElementsMatch(t, c.expectedFilters, httpConMgr.HttpFilters)
			}
		})
	}
}

func TestInsertFilter(t *testing.T) {
	cases := map[string]struct {
		inputFilters    []*envoy_listener_v3.Filter
		filterName      string
		insertOptions   InsertOptions
		expectedFilters []*envoy_listener_v3.Filter
		errStr          string
	}{
		"insert first": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertFirst},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "test.filter", "a", "b", "b", "b", "c"),
		},
		"insert last": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertLast},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "a", "b", "b", "b", "c", "test.filter"),
		},
		"insert before first match": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertBeforeFirstMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "a", "test.filter", "b", "b", "b", "c"),
		},
		"insert after first match": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertAfterFirstMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "a", "b", "test.filter", "b", "b", "c"),
		},
		"insert before last match": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertBeforeLastMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "a", "b", "b", "test.filter", "b", "c"),
		},
		"insert after last match": {
			inputFilters:    makeFilters(t, "a", "b", "b", "b", "c"),
			insertOptions:   InsertOptions{Location: InsertAfterLastMatch, FilterName: "b"},
			filterName:      "test.filter",
			expectedFilters: makeFilters(t, "a", "b", "b", "b", "test.filter", "c"),
		},
	}

	t.Parallel()
	for name, c := range cases {
		c := c

		t.Run(name, func(t *testing.T) {
			filter := &envoy_listener_v3.Filter{Name: c.filterName}
			obsFilters, err := InsertNetworkFilter(c.inputFilters, filter, c.insertOptions)
			if c.errStr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errStr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, c.expectedFilters, obsFilters)
			}
		})
	}
}

func makeHttpConMgr(t *testing.T, filters []*envoy_http_v3.HttpFilter) *envoy_listener_v3.Filter {
	t.Helper()
	httpConMgr := &envoy_http_v3.HttpConnectionManager{HttpFilters: filters}
	filter, err := MakeFilter("envoy.filters.network.http_connection_manager", httpConMgr)
	require.NoError(t, err)
	return filter
}

func makeHttpFilters(t *testing.T, names ...string) []*envoy_http_v3.HttpFilter {
	var filters []*envoy_http_v3.HttpFilter
	for _, name := range names {
		filters = append(filters, &envoy_http_v3.HttpFilter{Name: name})
	}
	return filters
}

func makeFilters(t *testing.T, names ...string) []*envoy_listener_v3.Filter {
	var filters []*envoy_listener_v3.Filter
	for _, name := range names {
		filters = append(filters, &envoy_listener_v3.Filter{Name: name})
	}
	return filters
}
