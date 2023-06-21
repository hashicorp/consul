// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"sort"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// CacheServiceList satisfies the proxycfg.ServiceList interface by sourcing
// data from the agent cache.
func CacheServiceList(c *cache.Cache) proxycfg.ServiceList {
	return &cacheProxyDataSource[*structs.DCSpecificRequest]{c, cachetype.CatalogServiceListName}
}

func ServerServiceList(deps ServerDataSourceDeps, remoteSource proxycfg.ServiceList) proxycfg.ServiceList {
	return &serverServiceList{deps, remoteSource}
}

type serverServiceList struct {
	deps         ServerDataSourceDeps
	remoteSource proxycfg.ServiceList
}

func (s *serverServiceList) Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	if req.Datacenter != s.deps.Datacenter {
		return s.remoteSource.Notify(ctx, req, correlationID, ch)
	}
	return s.deps.ViewStore.NotifyCallback(
		ctx,
		&serviceListRequest{s.deps, req},
		correlationID,
		dispatchCacheUpdate(ch),
	)
}

type serviceListRequest struct {
	deps ServerDataSourceDeps
	req  *structs.DCSpecificRequest
}

func (r *serviceListRequest) Request(index uint64) *pbsubscribe.SubscribeRequest {
	return &pbsubscribe.SubscribeRequest{
		Topic:      pbsubscribe.Topic_ServiceList,
		Subject:    &pbsubscribe.SubscribeRequest_WildcardSubject{WildcardSubject: true},
		Index:      index,
		Datacenter: r.req.Datacenter,
		Token:      r.req.QueryOptions.Token,
	}
}

func (r *serviceListRequest) CacheInfo() cache.RequestInfo { return r.req.CacheInfo() }

func (r *serviceListRequest) NewMaterializer() (submatview.Materializer, error) {
	return submatview.NewLocalMaterializer(submatview.LocalMaterializerDeps{
		Backend:     r.deps.EventPublisher,
		ACLResolver: r.deps.ACLResolver,
		Deps: submatview.Deps{
			View:    newServiceListView(r.req.EnterpriseMeta),
			Logger:  r.deps.Logger,
			Request: r.Request,
		},
	}), nil
}

func (serviceListRequest) Type() string { return "proxycfgglue.ServiceList" }

func newServiceListView(entMeta acl.EnterpriseMeta) *serviceListView {
	view := &serviceListView{entMeta: entMeta}
	view.Reset()
	return view
}

type serviceListView struct {
	entMeta acl.EnterpriseMeta
	state   map[string]structs.ServiceName
}

func (v *serviceListView) Reset() { v.state = make(map[string]structs.ServiceName) }

func (v *serviceListView) Update(events []*pbsubscribe.Event) error {
	for _, event := range filterByEnterpriseMeta(events, v.entMeta) {
		update := event.GetService()
		if update == nil {
			continue
		}

		var entMeta acl.EnterpriseMeta
		pbcommon.EnterpriseMetaToStructs(update.EnterpriseMeta, &entMeta)
		name := structs.NewServiceName(update.Name, &entMeta)

		switch update.Op {
		case pbsubscribe.CatalogOp_Register:
			v.state[name.String()] = name
		case pbsubscribe.CatalogOp_Deregister:
			delete(v.state, name.String())
		}
	}
	return nil
}

func (v *serviceListView) Result(index uint64) any {
	serviceList := make(structs.ServiceList, 0, len(v.state))
	for _, name := range v.state {
		serviceList = append(serviceList, name)
	}
	sort.Slice(serviceList, func(a, b int) bool {
		return serviceList[a].String() < serviceList[b].String()
	})
	return &structs.IndexedServiceList{
		Services: serviceList,
		QueryMeta: structs.QueryMeta{
			Backend: structs.QueryBackendStreaming,
			Index:   index,
		},
	}
}

// filterByEnterpriseMeta filters the given set of events to remove those that
// don't match the request's enterprise meta - this is necessary because when
// subscribing to a topic with SubjectWildcard we'll get events for resources
// in all partitions and namespaces.
func filterByEnterpriseMeta(events []*pbsubscribe.Event, entMeta acl.EnterpriseMeta) []*pbsubscribe.Event {
	partition := entMeta.PartitionOrDefault()
	namespace := entMeta.NamespaceOrDefault()

	filtered := make([]*pbsubscribe.Event, 0, len(events))
	for _, event := range events {
		var eventEntMeta *pbcommon.EnterpriseMeta
		switch payload := event.Payload.(type) {
		case *pbsubscribe.Event_ConfigEntry:
			eventEntMeta = payload.ConfigEntry.ConfigEntry.GetEnterpriseMeta()
		case *pbsubscribe.Event_Service:
			eventEntMeta = payload.Service.GetEnterpriseMeta()
		default:
			continue
		}

		if partition != acl.WildcardName && !acl.EqualPartitions(partition, eventEntMeta.GetPartition()) {
			continue
		}
		if namespace != acl.WildcardName && !acl.EqualNamespaces(namespace, eventEntMeta.GetNamespace()) {
			continue
		}

		filtered = append(filtered, event)
	}
	return filtered
}
