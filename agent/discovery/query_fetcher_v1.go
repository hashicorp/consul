// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/config"
)

const (
	// TODO (v2-dns): can we move the recursion into the data fetcher?
	maxRecursionLevelDefault = 3 // This field comes from the V1 DNS server and affects V1 catalog lookups
	maxRecurseRecords        = 5
)

type v1DataFetcherDynamicConfig struct {
	allowStale  bool
	maxStale    time.Duration
	useCache    bool
	cacheMaxAge time.Duration
	onlyPassing bool
}

type V1DataFetcher struct {
	dynamicConfig atomic.Value
}

func NewV1DataFetcher(config *config.RuntimeConfig) *V1DataFetcher {
	f := &V1DataFetcher{}
	f.LoadConfig(config)
	return f
}

func (f *V1DataFetcher) LoadConfig(config *config.RuntimeConfig) {
	dynamicConfig := &v1DataFetcherDynamicConfig{
		allowStale:  config.DNSAllowStale,
		maxStale:    config.DNSMaxStale,
		useCache:    config.DNSUseCache,
		cacheMaxAge: config.DNSCacheMaxAge,
		onlyPassing: config.DNSOnlyPassing,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

// TODO (v2-dns): Implementation of the V1 data fetcher

func (f *V1DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}

func (f *V1DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	return nil, nil
}

func (f *V1DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

func (f *V1DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	return nil, nil
}

func (f *V1DataFetcher) FetchWorkload(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

func (f *V1DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}
