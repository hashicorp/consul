// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// TODO (v2-dns): can we move the recursion into the data fetcher?
	maxRecursionLevelDefault = 3 // This field comes from the V1 DNS server and affects V1 catalog lookups
	maxRecurseRecords        = 5
)

// v1DataFetcherDynamicConfig is used to store the dynamic configuration of the V1 data fetcher.
type v1DataFetcherDynamicConfig struct {
	allowStale  bool
	maxStale    time.Duration
	useCache    bool
	cacheMaxAge time.Duration
	onlyPassing bool
}

// V1DataFetcher is used to fetch data from the V1 catalog.
type V1DataFetcher struct {
	dynamicConfig atomic.Value
	logger        hclog.Logger

	rpcFunc func(ctx context.Context, method string, args interface{}, reply interface{}) error
}

// NewV1DataFetcher creates a new V1 data fetcher.
func NewV1DataFetcher(config *config.RuntimeConfig,
	rpcFunc func(ctx context.Context, method string, args interface{}, reply interface{}) error,
	logger hclog.Logger) *V1DataFetcher {
	f := &V1DataFetcher{
		rpcFunc: rpcFunc,
		logger:  logger,
	}
	f.LoadConfig(config)
	return f
}

// LoadConfig loads the configuration for the V1 data fetcher.
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

// FetchNodes fetches A/AAAA/CNAME
func (f *V1DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}

// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
func (f *V1DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	return nil, nil
}

// FetchVirtualIP fetches A/AAAA records for virtual IPs
func (f *V1DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	args := structs.ServiceSpecificRequest{
		// The datacenter of the request is not specified because cross-datacenter virtual IP
		// queries are not supported. This guard rail is in place because virtual IPs are allocated
		// within a DC, therefore their uniqueness is not guaranteed globally.
		PeerName:       req.Tenancy.Peer,
		ServiceName:    req.Name,
		EnterpriseMeta: req.Tenancy.EnterpriseMeta,
		QueryOptions: structs.QueryOptions{
			Token: ctx.Token,
		},
	}

	var out string
	if err := f.rpcFunc(context.Background(), "Catalog.VirtualIPForService", &args, &out); err != nil {
		return nil, err
	}

	result := &Result{
		Address: out,
		Type:    ResultTypeVirtual,
	}
	return result, nil
}

// FetchRecordsByIp is used for PTR requests to look up a service/node from an IP.
func (f *V1DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	return nil, nil
}

// FetchWorkload fetches a single Result associated with
// V2 Workload. V2-only.
func (f *V1DataFetcher) FetchWorkload(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

// FetchPreparedQuery evaluates the results of a prepared query.
// deprecated in V2
func (f *V1DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}
