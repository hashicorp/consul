// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

const (
	// Increment a counter when requests staler than this are served
	staleCounterThreshold = 5 * time.Second
)

// DNSCounters pre-registers the staleness metric.
// This value is used by both the V1 and V2 DNS (V1 Catalog-only) servers.
var DNSCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"dns", "stale_queries"},
		Help: "Increments when an agent serves a query within the allowed stale threshold.",
	},
}

// V1DataFetcherDynamicConfig is used to store the dynamic configuration of the V1 data fetcher.
type V1DataFetcherDynamicConfig struct {
	// Default request tenancy
	Datacenter string

	SegmentName   string
	NodeName      string
	NodePartition string

	// Catalog configuration
	AllowStale  bool
	MaxStale    time.Duration
	UseCache    bool
	CacheMaxAge time.Duration
	OnlyPassing bool
}

// V1DataFetcher is used to fetch data from the V1 catalog.
type V1DataFetcher struct {
	defaultEnterpriseMeta acl.EnterpriseMeta
	dynamicConfig         atomic.Value
	logger                hclog.Logger

	getFromCacheFunc         func(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error)
	rpcFunc                  func(ctx context.Context, method string, args interface{}, reply interface{}) error
	rpcFuncForServiceNodes   func(ctx context.Context, req structs.ServiceSpecificRequest) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error)
	rpcFuncForSamenessGroup  func(ctx context.Context, req *structs.ConfigEntryQuery) (structs.SamenessGroupConfigEntry, cache.ResultMeta, error)
	translateServicePortFunc func(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int
}

// NewV1DataFetcher creates a new V1 data fetcher.
func NewV1DataFetcher(config *config.RuntimeConfig,
	entMeta *acl.EnterpriseMeta,
	getFromCacheFunc func(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error),
	rpcFunc func(ctx context.Context, method string, args interface{}, reply interface{}) error,
	rpcFuncForServiceNodes func(ctx context.Context, req structs.ServiceSpecificRequest) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error),
	rpcFuncForSamenessGroup func(ctx context.Context, req *structs.ConfigEntryQuery) (structs.SamenessGroupConfigEntry, cache.ResultMeta, error),
	translateServicePortFunc func(dc string, port int, taggedAddresses map[string]structs.ServiceAddress) int,
	logger hclog.Logger) *V1DataFetcher {
	f := &V1DataFetcher{
		defaultEnterpriseMeta:    *entMeta,
		getFromCacheFunc:         getFromCacheFunc,
		rpcFunc:                  rpcFunc,
		rpcFuncForServiceNodes:   rpcFuncForServiceNodes,
		rpcFuncForSamenessGroup:  rpcFuncForSamenessGroup,
		translateServicePortFunc: translateServicePortFunc,
		logger:                   logger,
	}
	f.LoadConfig(config)
	return f
}

// LoadConfig loads the configuration for the V1 data fetcher.
func (f *V1DataFetcher) LoadConfig(config *config.RuntimeConfig) {
	dynamicConfig := &V1DataFetcherDynamicConfig{
		AllowStale:  config.DNSAllowStale,
		MaxStale:    config.DNSMaxStale,
		UseCache:    config.DNSUseCache,
		CacheMaxAge: config.DNSCacheMaxAge,
		OnlyPassing: config.DNSOnlyPassing,
		Datacenter:  config.Datacenter,
		SegmentName: config.SegmentName,
		NodeName:    config.NodeName,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

func (f *V1DataFetcher) GetConfig() *V1DataFetcherDynamicConfig {
	return f.dynamicConfig.Load().(*V1DataFetcherDynamicConfig)
}

// FetchNodes fetches A/AAAA/CNAME
func (f *V1DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	if req.Tenancy.Namespace != "" && req.Tenancy.Namespace != acl.DefaultNamespaceName {
		// Nodes are not namespaced, so this is a name error
		return nil, ErrNotFound
	}
	cfg := f.dynamicConfig.Load().(*V1DataFetcherDynamicConfig)

	// If no datacenter is passed, default to our own
	datacenter := cfg.Datacenter
	if req.Tenancy.Datacenter != "" {
		datacenter = req.Tenancy.Datacenter
	}

	// Make an RPC request
	args := &structs.NodeSpecificRequest{
		Datacenter: datacenter,
		PeerName:   req.Tenancy.Peer,
		Node:       req.Name,
		QueryOptions: structs.QueryOptions{
			Token:      ctx.Token,
			AllowStale: cfg.AllowStale,
		},
		EnterpriseMeta: queryTenancyToEntMeta(req.Tenancy),
	}
	out, err := f.fetchNode(cfg, args)
	if err != nil {
		return nil, fmt.Errorf("failed rpc request: %w", err)
	}

	// If we have no out.NodeServices.Nodeaddress, return not found!
	if out.NodeServices == nil {
		return nil, ErrNotFound
	}

	results := make([]*Result, 0, 1)
	n := out.NodeServices.Node

	results = append(results, &Result{
		Node: &Location{
			Name:            n.Node,
			Address:         n.Address,
			TaggedAddresses: makeTaggedAddressesFromStrings(n.TaggedAddresses),
		},
		Type:     ResultTypeNode,
		Metadata: n.Meta,

		Tenancy: ResultTenancy{
			// Namespace is not required because nodes are not namespaced
			Partition:  n.GetEnterpriseMeta().PartitionOrDefault(),
			Datacenter: n.Datacenter,
		},
	})

	return results, nil
}

// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
func (f *V1DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	f.logger.Trace(fmt.Sprintf("FetchEndpoints - req: %+v / lookupType: %+v", req, lookupType))
	cfg := f.dynamicConfig.Load().(*V1DataFetcherDynamicConfig)
	return f.fetchService(ctx, req, cfg, lookupType)
}

// FetchVirtualIP fetches A/AAAA records for virtual IPs
func (f *V1DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	args := structs.ServiceSpecificRequest{
		// The Datacenter of the request is not specified because cross-Datacenter virtual IP
		// queries are not supported. This guard rail is in place because virtual IPs are allocated
		// within a DC, therefore their uniqueness is not guaranteed globally.
		PeerName:       req.Tenancy.Peer,
		ServiceName:    req.Name,
		EnterpriseMeta: queryTenancyToEntMeta(req.Tenancy),
		QueryOptions: structs.QueryOptions{
			Token: ctx.Token,
		},
	}

	var out string
	if err := f.rpcFunc(context.Background(), "Catalog.VirtualIPForService", &args, &out); err != nil {
		return nil, err
	}

	result := &Result{
		Service: &Location{
			Name:    req.Name,
			Address: out,
		},
		Type: ResultTypeVirtual,
	}
	return result, nil
}

// FetchRecordsByIp is used for PTR requests to look up a service/node from an IP.
// The search is performed in the agent's partition and over all namespaces (or those allowed by the ACL token).
func (f *V1DataFetcher) FetchRecordsByIp(reqCtx Context, ip net.IP) ([]*Result, error) {
	if ip == nil {
		return nil, ErrNotSupported
	}

	configCtx := f.dynamicConfig.Load().(*V1DataFetcherDynamicConfig)
	targetIP := ip.String()

	var results []*Result

	args := structs.DCSpecificRequest{
		Datacenter: configCtx.Datacenter,
		QueryOptions: structs.QueryOptions{
			Token:      reqCtx.Token,
			AllowStale: configCtx.AllowStale,
		},
	}
	var out structs.IndexedNodes

	// TODO: Replace ListNodes with an internal RPC that can do the filter
	// server side to avoid transferring the entire node list.
	if err := f.rpcFunc(context.Background(), "Catalog.ListNodes", &args, &out); err == nil {
		for _, n := range out.Nodes {
			if targetIP == n.Address {
				results = append(results, &Result{
					Node: &Location{
						Name:            n.Node,
						Address:         n.Address,
						TaggedAddresses: makeTaggedAddressesFromStrings(n.TaggedAddresses),
					},
					Type: ResultTypeNode,
					Tenancy: ResultTenancy{
						Namespace:  f.defaultEnterpriseMeta.NamespaceOrDefault(),
						Partition:  f.defaultEnterpriseMeta.PartitionOrDefault(),
						Datacenter: configCtx.Datacenter,
					},
				})
				return results, nil
			}
		}
	}

	// only look into the services if we didn't find a node
	sargs := structs.ServiceSpecificRequest{
		Datacenter: configCtx.Datacenter,
		QueryOptions: structs.QueryOptions{
			Token:      reqCtx.Token,
			AllowStale: configCtx.AllowStale,
		},
		ServiceAddress: targetIP,
		EnterpriseMeta: *f.defaultEnterpriseMeta.WithWildcardNamespace(),
	}

	var sout structs.IndexedServiceNodes
	if err := f.rpcFunc(context.Background(), "Catalog.ServiceNodes", &sargs, &sout); err == nil {
		if len(sout.ServiceNodes) == 0 {
			return nil, ErrNotFound
		}

		for _, n := range sout.ServiceNodes {
			if n.ServiceAddress == targetIP {
				results = append(results, &Result{
					Service: &Location{
						Name:    n.ServiceName,
						Address: n.ServiceAddress,
					},
					Type: ResultTypeService,
					Node: &Location{
						Name:    n.Node,
						Address: n.Address,
					},
					Tenancy: ResultTenancy{
						Namespace:  n.NamespaceOrEmpty(),
						Partition:  n.PartitionOrEmpty(),
						Datacenter: n.Datacenter,
					},
				})
				return results, nil
			}
		}
	}

	// nothing found locally, recurse
	// TODO: (v2-dns) implement recursion (NET-7883)
	//d.handleRecurse(resp, req)

	return nil, fmt.Errorf("unhandled error in FetchRecordsByIp")
}

// FetchWorkload fetches a single Result associated with
// V2 Workload. V2-only.
func (f *V1DataFetcher) FetchWorkload(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, ErrNotSupported
}

// FetchPreparedQuery evaluates the results of a prepared query.
// deprecated in V2
func (f *V1DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	cfg := f.dynamicConfig.Load().(*V1DataFetcherDynamicConfig)

	// If no datacenter is passed, default to our own
	datacenter := cfg.Datacenter
	if req.Tenancy.Datacenter != "" {
		datacenter = req.Tenancy.Datacenter
	}

	// Execute the prepared query.
	args := structs.PreparedQueryExecuteRequest{
		Datacenter:    datacenter,
		QueryIDOrName: req.Name,
		QueryOptions: structs.QueryOptions{
			Token:      ctx.Token,
			AllowStale: cfg.AllowStale,
			MaxAge:     cfg.CacheMaxAge,
		},

		// Always pass the local agent through. In the DNS interface, there
		// is no provision for passing additional query parameters, so we
		// send the local agent's data through to allow distance sorting
		// relative to ourself on the server side.
		Agent: structs.QuerySource{
			Datacenter:    cfg.Datacenter,
			Segment:       cfg.SegmentName,
			Node:          cfg.NodeName,
			NodePartition: cfg.NodePartition,
		},
		Source: structs.QuerySource{
			Ip: req.SourceIP.String(),
		},
	}

	out, err := f.executePreparedQuery(cfg, args)
	if err != nil {
		// errors.Is() doesn't work with errors.New() so we need to check the error message.
		if err.Error() == structs.ErrQueryNotFound.Error() {
			err = ErrNotFound
		}
		return nil, ECSNotGlobalError{err}
	}

	// TODO (slackpad) - What's a safe limit we can set here? It seems like
	// with dup filtering done at this level we need to get everything to
	// match the previous behavior. We can optimize by pushing more filtering
	// into the query execution, but for now I think we need to get the full
	// response. We could also choose a large arbitrary number that will
	// likely work in practice, like 10*maxUDPAnswerLimit which should help
	// reduce bandwidth if there are thousands of nodes available.
	// Determine the TTL. The parse should never fail since we vet it when
	// the query is created, but we check anyway. If the query didn't
	// specify a TTL then we will try to use the agent's service-specific
	// TTL configs.

	// Check is there is a TTL provided as part of the prepared query
	var ttlOverride *uint32
	if out.DNS.TTL != "" {
		ttl, err := time.ParseDuration(out.DNS.TTL)
		if err == nil {
			ttlSec := uint32(ttl / time.Second)
			ttlOverride = &ttlSec
		}
		f.logger.Warn("Failed to parse TTL for prepared query , ignoring",
			"ttl", out.DNS.TTL,
			"prepared_query", req.Name,
		)
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		return nil, ECSNotGlobalError{ErrNotFound}
	}

	// Perform a random shuffle
	out.Nodes.Shuffle()
	return f.buildResultsFromServiceNodes(out.Nodes, req, ttlOverride), ECSNotGlobalError{}
}

// executePreparedQuery is used to execute a PreparedQuery against the Consul catalog.
// If the config is set to UseCache, it will use agent cache.
func (f *V1DataFetcher) executePreparedQuery(cfg *V1DataFetcherDynamicConfig, args structs.PreparedQueryExecuteRequest) (*structs.PreparedQueryExecuteResponse, error) {
	var out structs.PreparedQueryExecuteResponse

RPC:
	if cfg.UseCache {
		raw, m, err := f.getFromCacheFunc(context.TODO(), cachetype.PreparedQueryName, &args)
		if err != nil {
			return nil, err
		}
		reply, ok := raw.(*structs.PreparedQueryExecuteResponse)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, err
		}

		f.logger.Trace("cache results for prepared query",
			"cache_hit", m.Hit,
			"prepared_query", args.QueryIDOrName,
		)

		out = *reply
	} else {
		if err := f.rpcFunc(context.Background(), "PreparedQuery.Execute", &args, &out); err != nil {
			return nil, err
		}
	}

	// Verify that request is not too stale, redo the request.
	if args.AllowStale {
		if out.LastContact > cfg.MaxStale {
			args.AllowStale = false
			f.logger.Warn("Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"dns", "stale_queries"}, 1)
		}
	}

	return &out, nil
}

func (f *V1DataFetcher) ValidateRequest(_ Context, req *QueryPayload) error {
	if req.EnableFailover {
		return ErrNotSupported
	}
	if req.PortName != "" {
		return ErrNotSupported
	}
	return validateEnterpriseTenancy(req.Tenancy)
}

// buildResultsFromServiceNodes builds a list of results from a list of nodes.
func (f *V1DataFetcher) buildResultsFromServiceNodes(nodes []structs.CheckServiceNode, req *QueryPayload, ttlOverride *uint32) []*Result {
	// Convert the service endpoints to results up to the limit
	limit := req.Limit
	if len(nodes) < limit || limit == 0 {
		limit = len(nodes)
	}

	results := make([]*Result, 0, limit)
	for idx := 0; idx < limit; idx++ {
		n := nodes[idx]
		results = append(results, &Result{
			Service: &Location{
				Name:            n.Service.Service,
				Address:         n.Service.Address,
				TaggedAddresses: makeTaggedAddressesFromServiceAddresses(n.Service.TaggedAddresses),
			},
			Node: &Location{
				Name:            n.Node.Node,
				Address:         n.Node.Address,
				TaggedAddresses: makeTaggedAddressesFromStrings(n.Node.TaggedAddresses),
			},
			Type: ResultTypeService,
			DNS: DNSConfig{
				TTL:    ttlOverride,
				Weight: uint32(findWeight(n)),
			},
			Ports: []Port{
				{Number: uint32(f.translateServicePortFunc(n.Node.Datacenter, n.Service.Port, n.Service.TaggedAddresses))},
			},
			Metadata: n.Node.Meta,
			Tenancy: ResultTenancy{
				Namespace:  n.Service.NamespaceOrEmpty(),
				Partition:  n.Service.PartitionOrEmpty(),
				Datacenter: n.Node.Datacenter,
				PeerName:   n.Service.PeerName,
			},
		})
	}
	return results
}

// makeTaggedAddressesFromServiceAddresses is used to convert a map of service addresses to a map of Locations.
func makeTaggedAddressesFromServiceAddresses(tagged map[string]structs.ServiceAddress) map[string]*TaggedAddress {
	taggedAddresses := make(map[string]*TaggedAddress)
	for k, v := range tagged {
		taggedAddresses[k] = &TaggedAddress{
			Name:    k,
			Address: v.Address,
			Port: Port{
				Number: uint32(v.Port),
			},
		}
	}
	return taggedAddresses
}

// makeTaggedAddressesFromStrings is used to convert a map of strings to a map of Locations.
func makeTaggedAddressesFromStrings(tagged map[string]string) map[string]*TaggedAddress {
	taggedAddresses := make(map[string]*TaggedAddress)
	for k, v := range tagged {
		taggedAddresses[k] = &TaggedAddress{
			Name:    k,
			Address: v,
		}
	}
	return taggedAddresses
}

// fetchNode is used to look up a node in the Consul catalog within NodeServices.
// If the config is set to UseCache, it will get the record from the agent cache.
func (f *V1DataFetcher) fetchNode(cfg *V1DataFetcherDynamicConfig, args *structs.NodeSpecificRequest) (*structs.IndexedNodeServices, error) {
	var out structs.IndexedNodeServices

	useCache := cfg.UseCache
RPC:
	if useCache {
		raw, _, err := f.getFromCacheFunc(context.TODO(), cachetype.NodeServicesName, args)
		if err != nil {
			return nil, err
		}
		reply, ok := raw.(*structs.IndexedNodeServices)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
		if err := f.rpcFunc(context.Background(), "Catalog.NodeServices", &args, &out); err != nil {
			return nil, err
		}
	}

	// Verify that request is not too stale, redo the request
	if args.AllowStale {
		if out.LastContact > cfg.MaxStale {
			args.AllowStale = false
			useCache = false
			f.logger.Warn("Query results too stale, re-requesting")
			goto RPC
		} else if out.LastContact > staleCounterThreshold {
			metrics.IncrCounter([]string{"dns", "stale_queries"}, 1)
		}
	}

	return &out, nil
}

// fetchService is used to look up a service in the Consul catalog.
func (f *V1DataFetcher) fetchService(ctx Context, req *QueryPayload,
	cfg *V1DataFetcherDynamicConfig, lookupType LookupType) ([]*Result, error) {
	f.logger.Trace(fmt.Sprintf("fetchService - req: %+v", req))

	// If no datacenter is passed, default to our own
	datacenter := cfg.Datacenter
	if req.Tenancy.Datacenter != "" {
		datacenter = req.Tenancy.Datacenter
	}
	if req.Tenancy.Peer != "" {
		datacenter = ""
	}

	serviceTags := []string{}
	if req.Tag != "" {
		serviceTags = []string{req.Tag}
	}
	healthFilterType := structs.HealthFilterExcludeCritical
	if cfg.OnlyPassing {
		healthFilterType = structs.HealthFilterIncludeOnlyPassing
	}
	args := structs.ServiceSpecificRequest{
		PeerName:         req.Tenancy.Peer,
		SamenessGroup:    req.Tenancy.SamenessGroup,
		Connect:          lookupType == LookupTypeConnect,
		Ingress:          lookupType == LookupTypeIngress,
		Datacenter:       datacenter,
		ServiceName:      req.Name,
		ServiceTags:      serviceTags,
		TagFilter:        req.Tag != "",
		HealthFilterType: healthFilterType,
		QueryOptions: structs.QueryOptions{
			Token:            ctx.Token,
			AllowStale:       cfg.AllowStale,
			MaxAge:           cfg.CacheMaxAge,
			UseCache:         cfg.UseCache,
			MaxStaleDuration: cfg.MaxStale,
		},
		EnterpriseMeta: queryTenancyToEntMeta(req.Tenancy),
	}

	out, _, err := f.rpcFuncForServiceNodes(context.TODO(), args)
	if err != nil {
		if strings.Contains(err.Error(), structs.ErrNoDCPath.Error()) {
			return nil, ErrNoPathToDatacenter
		}
		return nil, fmt.Errorf("rpc request failed: %w", err)
	}

	// If we have no nodes, return not found!
	if len(out.Nodes) == 0 {
		return nil, ErrNotFound
	}

	// Perform a random shuffle
	out.Nodes.Shuffle()
	return f.buildResultsFromServiceNodes(out.Nodes, req, nil), nil
}

// findWeight returns the weight of a service node.
func findWeight(node structs.CheckServiceNode) int {
	// By default, when only_passing is false, warning and passing nodes are returned
	// Those values will be used if using a client with support while server has no
	// support for weights
	weightPassing := 1
	weightWarning := 1
	if node.Service.Weights != nil {
		weightPassing = node.Service.Weights.Passing
		weightWarning = node.Service.Weights.Warning
	}
	serviceChecks := make(api.HealthChecks, 0, len(node.Checks))
	for _, c := range node.Checks {
		if c.ServiceName == node.Service.Service || c.ServiceName == "" {
			healthCheck := &api.HealthCheck{
				Node:        c.Node,
				CheckID:     string(c.CheckID),
				Name:        c.Name,
				Status:      c.Status,
				Notes:       c.Notes,
				Output:      c.Output,
				ServiceID:   c.ServiceID,
				ServiceName: c.ServiceName,
				ServiceTags: c.ServiceTags,
			}
			serviceChecks = append(serviceChecks, healthCheck)
		}
	}
	status := serviceChecks.AggregatedStatus()
	switch status {
	case api.HealthWarning:
		return weightWarning
	case api.HealthPassing:
		return weightPassing
	case api.HealthMaint:
		// Not used in theory
		return 0
	case api.HealthCritical:
		// Should not happen since already filtered
		return 0
	default:
		// When non-standard status, return 1
		return 1
	}
}
