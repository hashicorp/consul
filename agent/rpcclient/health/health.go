// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package health

import (
	"context"

	"google.golang.org/grpc/connectivity"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// Client provides access to service health data.
type Client struct {
	rpcclient.Client
}

// IsReadyForStreaming will indicate if the underlying gRPC connection is ready.
func (c *Client) IsReadyForStreaming() bool {
	conn := c.MaterializerDeps.Conn
	if conn == nil {
		return false
	}

	return conn.GetState() == connectivity.Ready
}

func (c *Client) ServiceNodes(
	ctx context.Context,
	req structs.ServiceSpecificRequest,
) (structs.IndexedCheckServiceNodes, cache.ResultMeta, error) {
	// Note: if MergeCentralConfig is requested, default to using the RPC backend for now
	// as the streaming backend and materializer does not have support for merging yet.
	if c.useStreaming(req) && (req.QueryOptions.UseCache || req.QueryOptions.MinQueryIndex > 0) && !req.MergeCentralConfig {
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
		err := c.NetRPC.RPC(context.Background(), "Health.ServiceNodes", &req, &out)
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
		err := c.NetRPC.RPC(context.Background(), "Health.ServiceNodes", &req, &out)
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
	cb cache.Callback,
) error {
	if c.useStreaming(req) {
		sr := c.newServiceRequest(req)
		return c.ViewStore.NotifyCallback(ctx, sr, correlationID, cb)
	}

	return c.Cache.NotifyCallback(ctx, c.CacheName, &req, correlationID, cb)
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

var _ submatview.Request = (*serviceRequest)(nil)

type serviceRequest struct {
	structs.ServiceSpecificRequest
	deps rpcclient.MaterializerDeps
}

func (r serviceRequest) CacheInfo() cache.RequestInfo {
	return r.ServiceSpecificRequest.CacheInfo()
}

func (r serviceRequest) Type() string {
	return "agent.rpcclient.health.serviceRequest"
}

func (r serviceRequest) NewMaterializer() (submatview.Materializer, error) {
	view, err := NewHealthView(r.ServiceSpecificRequest)
	if err != nil {
		return nil, err
	}
	deps := submatview.Deps{
		View:    view,
		Logger:  r.deps.Logger,
		Request: NewMaterializerRequest(r.ServiceSpecificRequest),
	}

	return submatview.NewRPCMaterializer(pbsubscribe.NewStateChangeSubscriptionClient(r.deps.Conn), deps), nil
}
