package health

import (
	"context"
	"strings"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

type Client struct {
	NetRPC NetRPC
	Cache  CacheGetter
	// CacheName to use for service health.
	CacheName string
	// CacheNameConnect is the name of the cache to use for connect service health.
	CacheNameConnect string
}

type NetRPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

type CacheGetter interface {
	Get(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error)
}

func (c *Client) ServiceNodes(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error) {
	out, md, err := c.getServiceNodes(ctx, req)
	if err != nil {
		return out, md, err
	}

	// TODO: DNSServer emitted a metric here, do we still need it?
	if req.QueryOptions.AllowStale && req.QueryOptions.MaxStaleDuration > 0 && out.QueryMeta.LastContact > req.MaxStaleDuration {
		req.AllowStale = false
		err := c.NetRPC.RPC("Health.ServiceNodes", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	return out, md, err
}

func (c *Client) getServiceNodes(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error) {
	var out structs.IndexedCheckServiceNodes

	req.ServiceName = strings.ToLower(req.ServiceName)
	if !req.QueryOptions.UseCache {
		err := c.NetRPC.RPC("Health.ServiceNodes", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	cacheName := c.CacheName
	if req.Connect {
		cacheName = c.CacheNameConnect
	}

	raw, md, err := c.Cache.Get(ctx, cacheName, &req)
	if err != nil {
		return out, md, err
	}

	value, ok := raw.(*structs.IndexedCheckServiceNodes)
	if !ok {
		panic("wrong response type for cachetype.HealthServicesName")
	}

	return filterTags(filterNodeMeta(value, req), req), md, nil
}

func filterTags(out *structs.IndexedCheckServiceNodes, req structs.ServiceSpecificRequest) structs.IndexedCheckServiceNodes {
	if len(req.ServiceTags) == 0 || len(out.Nodes) == 0 {
		return *out
	}
	tags := make([]string, 0, len(req.ServiceTags))
	for _, r := range req.ServiceTags {
		// DNS has the bad habit to setting [""] for ServiceTags
		if r != "" {
			tags = append(tags, strings.ToLower(r))
		}
	}
	// No need to filter
	if len(tags) == 0 {
		return *out
	}
	results := make(structs.CheckServiceNodes, 0, len(out.Nodes))
	for _, service := range out.Nodes {
		svc := service.Service
		if !serviceTagsFilter(svc, tags) {
			results = append(results, service)
		}
	}
	out.Nodes = results
	return *out
}

// serviceTagsFilter return true if service does not contains all the given tags
func serviceTagsFilter(sn *structs.NodeService, tags []string) bool {
	for _, tag := range tags {
		if serviceTagFilter(sn, tag) {
			// If any one of the expected tags was not found, filter the service
			return true
		}
	}

	// If all tags were found, don't filter the service
	return false
}

// serviceTagFilter returns true (should filter) if the given service node
// doesn't contain the given tag.
func serviceTagFilter(sn *structs.NodeService, tag string) bool {
	// Look for the lower cased version of the tag.
	for _, t := range sn.Tags {
		if strings.ToLower(t) == tag {
			return false
		}
	}

	// If we didn't hit the tag above then we should filter.
	return true
}

func filterNodeMeta(out *structs.IndexedCheckServiceNodes, req structs.ServiceSpecificRequest) *structs.IndexedCheckServiceNodes {
	if len(req.NodeMetaFilters) == 0 || len(out.Nodes) == 0 {
		return out
	}
	results := make(structs.CheckServiceNodes, 0, len(out.Nodes))
	for _, service := range out.Nodes {
		serviceNode := service.Node
		if structs.SatisfiesMetaFilters(serviceNode.Meta, req.NodeMetaFilters) {
			results = append(results, service)
		}
	}
	out.Nodes = results
	return out
}
