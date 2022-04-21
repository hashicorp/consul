package health

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// Client provides access to service health data.
type Client struct {
	NetRPC              NetRPC
	Cache               CacheGetter
	ViewStore           MaterializedViewStore
	MaterializerDeps    MaterializerDeps
	CacheName           string
	UseStreamingBackend bool
	QueryOptionDefaults func(options *structs.QueryOptions)
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
	if c.useStreaming(req) && (req.QueryOptions.UseCache || req.QueryOptions.MinQueryIndex > 0) {
		c.QueryOptionDefaults(&req.QueryOptions)

		result, err := c.ViewStore.Get(ctx, c.newServiceRequest(req))
		if err != nil {
			return structs.IndexedCheckServiceNodes{}, cache.ResultMeta{}, err
		}
		meta := cache.ResultMeta{Index: result.Index, Hit: result.Cached}
		return *result.Value.(*structs.IndexedCheckServiceNodes), meta, err
	}

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

func (c *Client) Notify(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
	correlationID string,
	ch chan<- cache.UpdateEvent,
) error {
	if c.useStreaming(req) {
		sr := c.newServiceRequest(req)
		return c.ViewStore.Notify(ctx, sr, correlationID, ch)
	}

	return c.Cache.Notify(ctx, c.CacheName, &req, correlationID, ch)
}

func (c *Client) useStreaming(req structs.ServiceSpecificRequest) bool {
	return c.UseStreamingBackend && !req.Ingress && req.Source.Node == ""
}

func (c *Client) newServiceRequest(req structs.ServiceSpecificRequest) serviceRequest {
	return serviceRequest{
		ServiceSpecificRequest: req,
		deps:                   c.MaterializerDeps,
	}
}

// Close any underlying connections used by the client.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.MaterializerDeps.Conn.Close()
}

type serviceRequest struct {
	structs.ServiceSpecificRequest
	deps MaterializerDeps
}

func (r serviceRequest) CacheInfo() cache.RequestInfo {
	return r.ServiceSpecificRequest.CacheInfo()
}

func (r serviceRequest) Type() string {
	return "agent.rpcclient.health.serviceRequest"
}

func (r serviceRequest) NewMaterializer() (submatview.Materializer, error) {
	view, err := newHealthView(r.ServiceSpecificRequest)
	if err != nil {
		return nil, err
	}
	deps := submatview.Deps{
		View:    view,
		Logger:  r.deps.Logger,
		Request: newMaterializerRequest(r.ServiceSpecificRequest),
	}

	return submatview.NewRPCMaterializer(pbsubscribe.NewStateChangeSubscriptionClient(r.deps.Conn), deps), nil
}
