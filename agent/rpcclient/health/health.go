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

	if !req.QueryOptions.UseCache {
		err := c.NetRPC.RPC("Health.ServiceNodes", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	raw, md, err := c.Cache.Get(ctx, c.CacheName, &req)
	if err != nil {
		return out, md, err
	}

	value, ok := raw.(*structs.IndexedCheckServiceNodes)
	if !ok {
		panic("wrong response type for cachetype.HealthServicesName")
	}

	return *value, md, nil
}
