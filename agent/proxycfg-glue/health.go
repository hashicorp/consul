// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
)

// ClientHealth satisfies the proxycfg.Health interface by sourcing data from
// the given health.Client.
func ClientHealth(client *health.Client) proxycfg.Health {
	return &clientHealth{client}
}

type clientHealth struct {
	client *health.Client
}

func (h *clientHealth) Notify(
	ctx context.Context,
	req *structs.ServiceSpecificRequest,
	correlationID string,
	ch chan<- proxycfg.UpdateEvent,
) error {
	return h.client.Notify(ctx, *req, correlationID, dispatchCacheUpdate(ch))
}

// ServerHealth satisfies the proxycfg.Health interface by sourcing data from
// a local materialized view (backed by an EventPublisher subscription).
//
// Requests for services in remote datacenters will be delegated to the given
// remoteSource (i.e. ClientHealth).
func ServerHealth(deps ServerDataSourceDeps, remoteSource proxycfg.Health) proxycfg.Health {
	return &serverHealth{deps, remoteSource}
}

type serverHealth struct {
	deps         ServerDataSourceDeps
	remoteSource proxycfg.Health
}

func (h *serverHealth) Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	if req.Datacenter != h.deps.Datacenter {
		return h.remoteSource.Notify(ctx, req, correlationID, ch)
	}

	return h.deps.ViewStore.NotifyCallback(
		ctx,
		&healthRequest{h.deps, *req},
		correlationID,
		dispatchCacheUpdate(ch),
	)
}

type healthRequest struct {
	deps ServerDataSourceDeps
	req  structs.ServiceSpecificRequest
}

func (r *healthRequest) CacheInfo() cache.RequestInfo { return r.req.CacheInfo() }

func (r *healthRequest) NewMaterializer() (submatview.Materializer, error) {
	view, err := health.NewHealthView(r.req)
	if err != nil {
		return nil, err
	}
	return submatview.NewLocalMaterializer(submatview.LocalMaterializerDeps{
		Backend:     r.deps.EventPublisher,
		ACLResolver: r.deps.ACLResolver,
		Deps: submatview.Deps{
			View:    view,
			Logger:  r.deps.Logger,
			Request: health.NewMaterializerRequest(r.req),
		},
	}), nil
}

func (r *healthRequest) Type() string { return "proxycfgglue.Health" }
