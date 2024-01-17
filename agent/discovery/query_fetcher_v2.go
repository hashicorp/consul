// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"net"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/config"
)

// v2DataFetcherDynamicConfig is used to store the dynamic configuration of the V2 data fetcher.
type v2DataFetcherDynamicConfig struct {
	onlyPassing bool
}

// V2DataFetcher is used to fetch data from the V2 catalog.
type V2DataFetcher struct {
	dynamicConfig atomic.Value
}

// NewV2DataFetcher creates a new V2 data fetcher.
func NewV2DataFetcher(config *config.RuntimeConfig) *V2DataFetcher {
	f := &V2DataFetcher{}
	f.LoadConfig(config)
	return f
}

// LoadConfig loads the configuration for the V2 data fetcher.
func (f *V2DataFetcher) LoadConfig(config *config.RuntimeConfig) {
	dynamicConfig := &v2DataFetcherDynamicConfig{
		onlyPassing: config.DNSOnlyPassing,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

// TODO (v2-dns): Implementation of the V2 data fetcher

// FetchNodes fetches A/AAAA/CNAME
func (f *V2DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}

// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
func (f *V2DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	return nil, nil
}

// FetchVirtualIP fetches A/AAAA records for virtual IPs
func (f *V2DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

// FetchRecordsByIp is used for PTR requests to look up a service/node from an IP.
func (f *V2DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	return nil, nil
}

// FetchWorkload is used to fetch a single workload from the V2 catalog.
// V2-only.
func (f *V2DataFetcher) FetchWorkload(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

// FetchPreparedQuery is used to fetch a prepared query from the V2 catalog.
// Deprecated in V2.
func (f *V2DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}
