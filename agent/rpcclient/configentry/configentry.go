// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package configentry

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/rpcclient"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// Client provides access to config entry data.
type Client struct {
	rpcclient.Client
}

// GetSamenessGroup returns the sameness group config entry (if possible) given the
// provided config entry query
func (c *Client) GetSamenessGroup(
	ctx context.Context,
	req *structs.ConfigEntryQuery,
) (structs.SamenessGroupConfigEntry, cache.ResultMeta, error) {
	if req.Kind != structs.SamenessGroup {
		return structs.SamenessGroupConfigEntry{}, cache.ResultMeta{}, fmt.Errorf("wrong kind in query %s, expected %s", req.Kind, structs.SamenessGroup)
	}

	out, meta, err := c.GetConfigEntry(ctx, req)
	if err != nil {
		return structs.SamenessGroupConfigEntry{}, cache.ResultMeta{}, err
	}

	sg, ok := out.Entry.(*structs.SamenessGroupConfigEntry)
	if !ok {
		return structs.SamenessGroupConfigEntry{}, cache.ResultMeta{}, fmt.Errorf("%s config entry with name %s not found", structs.SamenessGroup, req.Name)
	}
	return *sg, meta, nil
}

// GetConfigEntry returns the config entry (if possible) given the
// provided config entry query
func (c *Client) GetConfigEntry(
	ctx context.Context,
	req *structs.ConfigEntryQuery,
) (structs.ConfigEntryResponse, cache.ResultMeta, error) {
	if c.UseStreamingBackend && (req.QueryOptions.UseCache || req.QueryOptions.MinQueryIndex > 0) {
		c.QueryOptionDefaults(&req.QueryOptions)
		cfgReq, err := c.newConfigEntryRequest(req)
		if err != nil {
			return structs.ConfigEntryResponse{}, cache.ResultMeta{}, err
		}
		result, err := c.ViewStore.Get(ctx, cfgReq)
		if err != nil {
			return structs.ConfigEntryResponse{}, cache.ResultMeta{}, err
		}
		meta := cache.ResultMeta{Index: result.Index, Hit: result.Cached}
		return *result.Value.(*structs.ConfigEntryResponse), meta, err
	}

	out, md, err := c.getConfigEntryRPC(ctx, req)
	if err != nil {
		return out, md, err
	}

	if req.QueryOptions.AllowStale && req.QueryOptions.MaxStaleDuration > 0 && out.LastContact > req.MaxStaleDuration {
		req.AllowStale = false
		err := c.NetRPC.RPC(ctx, "ConfigEntry.Get", &req, &out)
		return out, cache.ResultMeta{}, err
	}

	return out, md, err
}

func (c *Client) getConfigEntryRPC(
	ctx context.Context,
	req *structs.ConfigEntryQuery,
) (structs.ConfigEntryResponse, cache.ResultMeta, error) {
	var out structs.ConfigEntryResponse
	if !req.QueryOptions.UseCache {
		err := c.NetRPC.RPC(context.Background(), "ConfigEntry.Get", req, &out)
		return out, cache.ResultMeta{}, err
	}

	raw, md, err := c.Cache.Get(ctx, c.CacheName, req)
	if err != nil {
		return out, md, err
	}

	value, ok := raw.(*structs.ConfigEntryResponse)
	if !ok {
		panic("wrong response type for cachetype.HealthServicesName")
	}

	return *value, md, nil
}

var _ submatview.Request = (*configEntryRequest)(nil)

type configEntryRequest struct {
	Topic pbsubscribe.Topic
	req   *structs.ConfigEntryQuery
	deps  rpcclient.MaterializerDeps
}

func (c *Client) newConfigEntryRequest(req *structs.ConfigEntryQuery) (*configEntryRequest, error) {
	var topic pbsubscribe.Topic
	switch req.Kind {
	case structs.SamenessGroup:
		topic = pbsubscribe.Topic_SamenessGroup
	default:
		return nil, fmt.Errorf("cannot map config entry kind: %q to a topic", req.Kind)
	}
	return &configEntryRequest{
		Topic: topic,
		req:   req,
		deps:  c.MaterializerDeps,
	}, nil
}

// CacheInfo returns information used for caching the config entry request.
func (r *configEntryRequest) CacheInfo() cache.RequestInfo {
	return r.req.CacheInfo()
}

// Type returns a string which uniquely identifies the config entry of request.
// The returned value is used as the prefix of the key used to index
// entries in the Store.
func (r *configEntryRequest) Type() string {
	return "agent.rpcclient.configentry.configentryrequest"
}

// Request creates a new pbsubscribe.SubscribeRequest for a config entry including
// wildcards and enterprise fields
func (r *configEntryRequest) Request(index uint64) *pbsubscribe.SubscribeRequest {
	req := &pbsubscribe.SubscribeRequest{
		Topic:      r.Topic,
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

// NewMaterializer will be called if there is no active materializer to fulfill
// the request. It returns a Materializer appropriate for streaming
// data to fulfil the config entry request.
func (r *configEntryRequest) NewMaterializer() (submatview.Materializer, error) {
	var view submatview.View
	if r.req.Name == "" {
		view = NewConfigEntryListView(r.req.Kind, r.req.EnterpriseMeta)
	} else {
		view = &ConfigEntryView{}
	}

	deps := submatview.Deps{
		View:    view,
		Logger:  r.deps.Logger,
		Request: r.Request,
	}

	return submatview.NewRPCMaterializer(pbsubscribe.NewStateChangeSubscriptionClient(r.deps.Conn), deps), nil
}
