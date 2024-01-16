// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"net"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/config"
)

type v2DataFetcherDynamicConfig struct {
	onlyPassing bool
}

type V2DataFetcher struct {
	dynamicConfig atomic.Value
}

func NewV2DataFetcher(config *config.RuntimeConfig) *V2DataFetcher {
	f := &V2DataFetcher{}
	f.LoadConfig(config)
	return f
}

func (f *V2DataFetcher) LoadConfig(config *config.RuntimeConfig) {
	dynamicConfig := &v2DataFetcherDynamicConfig{
		onlyPassing: config.DNSOnlyPassing,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

// TODO (v2-dns): Implementation of the V2 data fetcher

func (f *V2DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}

func (f *V2DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	return nil, nil
}

func (f *V2DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

func (f *V2DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	return nil, nil
}

func (f *V2DataFetcher) FetchWorkload(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

func (f *V2DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}
