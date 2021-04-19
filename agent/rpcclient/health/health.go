package health

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

type Client struct {
	NetRPC NetRPC
	Cache  CacheGetter
	// CacheName to use for service health.
	CacheName string
	// CacheNameNotStreaming is the name of the cache type to use for any requests
	// that are not supported by the streaming backend (ex: Ingress=true).
	CacheNameNotStreaming string
}

type NetRPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

type CacheGetter interface {
	Get(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error)
	Notify(ctx context.Context, t string, r cache.Request, cID string, ch chan<- cache.UpdateEvent) error
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

	if !req.QueryOptions.UseCache {
		err := c.NetRPC.RPC("Health.ServiceNodes", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	cacheName := c.CacheName
	if req.Ingress || req.Source.Node != "" {
		cacheName = c.CacheNameNotStreaming
	}

	raw, md, err := c.Cache.Get(ctx, cacheName, &req)
	if err != nil {
		return out, md, err
	}

	value, ok := raw.(*structs.IndexedCheckServiceNodes)
	if !ok {
		panic("wrong response type for cachetype.HealthServicesName")
	}

	return *value, md, nil
}

func (c *Client) Notify(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
	correlationID string,
	ch chan<- cache.UpdateEvent,
) error {
	cacheName := c.CacheName
	if req.Ingress || req.Source.Node != "" {
		cacheName = c.CacheNameNotStreaming
	}
	return c.Cache.Notify(ctx, cacheName, &req, correlationID, ch)
}

func (c *Client) UseStreaming(req structs.ServiceSpecificRequest) bool {
	if req.Ingress || req.Source.Node != "" {
		return false
	}

	return req.QueryOptions.UseCache || req.QueryOptions.MinQueryIndex > 0
}
