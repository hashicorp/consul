// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/rpcclient/configentry"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// CacheConfigEntry satisfies the proxycfg.ConfigEntry interface by sourcing
// data from the agent cache.
func CacheConfigEntry(c *cache.Cache) proxycfg.ConfigEntry {
	return &cacheProxyDataSource[*structs.ConfigEntryQuery]{c, cachetype.ConfigEntryName}
}

// CacheConfigEntryList satisfies the proxycfg.ConfigEntryList interface by
// sourcing data from the agent cache.
func CacheConfigEntryList(c *cache.Cache) proxycfg.ConfigEntryList {
	return &cacheProxyDataSource[*structs.ConfigEntryQuery]{c, cachetype.ConfigEntryListName}
}

// ServerConfigEntry satisfies the proxycfg.ConfigEntry interface by sourcing
// data from a local materialized view (backed by an EventPublisher subscription).
func ServerConfigEntry(deps ServerDataSourceDeps) proxycfg.ConfigEntry {
	return serverConfigEntry{deps}
}

// ServerConfigEntryList satisfies the proxycfg.ConfigEntry interface by sourcing
// data from a local materialized view (backed by an EventPublisher subscription).
func ServerConfigEntryList(deps ServerDataSourceDeps) proxycfg.ConfigEntryList {
	return serverConfigEntry{deps}
}

type serverConfigEntry struct {
	deps ServerDataSourceDeps
}

func (e serverConfigEntry) Notify(ctx context.Context, req *structs.ConfigEntryQuery, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	cfgReq, err := newConfigEntryRequest(req, e.deps)
	if err != nil {
		return err
	}
	return e.deps.ViewStore.NotifyCallback(ctx, cfgReq, correlationID, dispatchCacheUpdate(ch))
}

func newConfigEntryRequest(req *structs.ConfigEntryQuery, deps ServerDataSourceDeps) (*configEntryRequest, error) {
	var topic pbsubscribe.Topic
	switch req.Kind {
	case structs.MeshConfig:
		topic = pbsubscribe.Topic_MeshConfig
	case structs.ServiceResolver:
		topic = pbsubscribe.Topic_ServiceResolver
	case structs.IngressGateway:
		topic = pbsubscribe.Topic_IngressGateway
	case structs.ServiceDefaults:
		topic = pbsubscribe.Topic_ServiceDefaults
	case structs.APIGateway:
		topic = pbsubscribe.Topic_APIGateway
	case structs.HTTPRoute:
		topic = pbsubscribe.Topic_HTTPRoute
	case structs.TCPRoute:
		topic = pbsubscribe.Topic_TCPRoute
	case structs.InlineCertificate:
		topic = pbsubscribe.Topic_InlineCertificate
	case structs.BoundAPIGateway:
		topic = pbsubscribe.Topic_BoundAPIGateway
	case structs.RateLimitIPConfig:
		topic = pbsubscribe.Topic_IPRateLimit
	case structs.JWTProvider:
		topic = pbsubscribe.Topic_JWTProvider
	default:
		return nil, fmt.Errorf("cannot map config entry kind: %q to a topic", req.Kind)
	}
	return &configEntryRequest{
		topic: topic,
		req:   req,
		deps:  deps,
	}, nil
}

type configEntryRequest struct {
	topic pbsubscribe.Topic
	req   *structs.ConfigEntryQuery
	deps  ServerDataSourceDeps
}

func (r *configEntryRequest) CacheInfo() cache.RequestInfo { return r.req.CacheInfo() }

func (r *configEntryRequest) NewMaterializer() (submatview.Materializer, error) {
	var view submatview.View
	if r.req.Name == "" {
		view = configentry.NewConfigEntryListView(r.req.Kind, r.req.EnterpriseMeta)
	} else {
		view = &configentry.ConfigEntryView{}
	}

	return submatview.NewLocalMaterializer(submatview.LocalMaterializerDeps{
		Backend:     r.deps.EventPublisher,
		ACLResolver: r.deps.ACLResolver,
		Deps: submatview.Deps{
			View:    view,
			Logger:  r.deps.Logger,
			Request: r.Request,
		},
	}), nil
}

func (r *configEntryRequest) Type() string { return "proxycfgglue.ConfigEntry" }

func (r *configEntryRequest) Request(index uint64) *pbsubscribe.SubscribeRequest {
	req := &pbsubscribe.SubscribeRequest{
		Topic:      r.topic,
		Index:      index,
		Datacenter: r.req.Datacenter,
		Token:      r.req.QueryOptions.Token,
	}

	if name := r.req.Name; name == "" {
		req.Subject = &pbsubscribe.SubscribeRequest_WildcardSubject{
			WildcardSubject: true,
		}
	} else {
		req.Subject = &pbsubscribe.SubscribeRequest_NamedSubject{
			NamedSubject: &pbsubscribe.NamedSubject{
				Key:       name,
				Partition: r.req.PartitionOrDefault(),
				Namespace: r.req.NamespaceOrDefault(),
			},
		}
	}

	return req
}
