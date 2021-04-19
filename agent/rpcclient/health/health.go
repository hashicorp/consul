package health

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
)

type Client struct {
	NetRPC           NetRPC
	Cache            CacheGetter
	ViewStore        MaterializedViewStore
	MaterializerDeps MaterializerDeps
	// CacheName to use for service health.
	CacheName string
	// CacheNameIngress is the name of the cache type to use for ingress
	// service health.
	CacheNameIngress string
}

type NetRPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

type CacheGetter interface {
	Get(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error)
	Notify(ctx context.Context, t string, r cache.Request, cID string, ch chan<- cache.UpdateEvent) error
}

type MaterializedViewStore interface {
	Get(ctx context.Context, req submatview.Request) (submatview.Result, error)
	Notify(ctx context.Context, req submatview.Request, cID string, ch chan<- cache.UpdateEvent) error
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

	// TODO: if UseStreaming, elif !UseCache, else cache

	if !req.QueryOptions.UseCache {
		err := c.NetRPC.RPC("Health.ServiceNodes", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	if req.Source.Node == "" {
		sr := serviceRequest{
			ServiceSpecificRequest: req,
			deps:                   c.MaterializerDeps,
		}

		result, err := c.ViewStore.Get(ctx, sr)
		if err != nil {
			return out, cache.ResultMeta{}, err
		}
		// TODO: can we store non-pointer
		return *result.Value.(*structs.IndexedCheckServiceNodes), cache.ResultMeta{Index: result.Index}, err
	}

	cacheName := c.CacheName
	if req.Ingress {
		cacheName = c.CacheNameIngress
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
	if req.Ingress {
		cacheName = c.CacheNameIngress
	}
	return c.Cache.Notify(ctx, cacheName, &req, correlationID, ch)
}

type serviceRequest struct {
	structs.ServiceSpecificRequest
	deps MaterializerDeps
}

func (r serviceRequest) CacheInfo() cache.RequestInfo {
	return r.ServiceSpecificRequest.CacheInfo()
}

func (r serviceRequest) Type() string {
	return "service-health"
}

func (r serviceRequest) NewMaterializer() (*submatview.Materializer, error) {
	view, err := newHealthView(r.ServiceSpecificRequest)
	if err != nil {
		return nil, err
	}
	return submatview.NewMaterializer(submatview.Deps{
		View:    view,
		Client:  r.deps.Client,
		Logger:  r.deps.Logger,
		Request: newMaterializerRequest(r.ServiceSpecificRequest),
	}), nil
}
