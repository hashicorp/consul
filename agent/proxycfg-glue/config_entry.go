package proxycfgglue

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbconfigentry"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// ServerDataSourceDeps contains the dependencies needed for sourcing data from
// server-local sources (e.g. materialized views).
type ServerDataSourceDeps struct {
	Datacenter     string
	ViewStore      *submatview.Store
	EventPublisher *stream.EventPublisher
	Logger         hclog.Logger
	ACLResolver    submatview.ACLResolver
	GetStore       func() Store
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
	default:
		return nil, fmt.Errorf("cannot map config entry kind: %s to a topic", req.Kind)
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
		view = newConfigEntryListView(r.req.Kind, r.req.EnterpriseMeta)
	} else {
		view = &configEntryView{}
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

// configEntryView implements a submatview.View for a single config entry.
type configEntryView struct {
	state structs.ConfigEntry
}

func (v *configEntryView) Reset() {
	v.state = nil
}

func (v *configEntryView) Result(index uint64) any {
	return &structs.ConfigEntryResponse{
		QueryMeta: structs.QueryMeta{
			Index:   index,
			Backend: structs.QueryBackendStreaming,
		},
		Entry: v.state,
	}
}

func (v *configEntryView) Update(events []*pbsubscribe.Event) error {
	for _, event := range events {
		update := event.GetConfigEntry()
		if update == nil {
			continue
		}
		switch update.Op {
		case pbsubscribe.ConfigEntryUpdate_Delete:
			v.state = nil
		case pbsubscribe.ConfigEntryUpdate_Upsert:
			v.state = pbconfigentry.ConfigEntryToStructs(update.ConfigEntry)
		}
	}
	return nil
}

// configEntryListView implements a submatview.View for a list of config entries
// that are all of the same kind (name is treated as unique).
type configEntryListView struct {
	kind    string
	entMeta acl.EnterpriseMeta
	state   map[string]structs.ConfigEntry
}

func newConfigEntryListView(kind string, entMeta acl.EnterpriseMeta) *configEntryListView {
	view := &configEntryListView{kind: kind, entMeta: entMeta}
	view.Reset()
	return view
}

func (v *configEntryListView) Reset() {
	v.state = make(map[string]structs.ConfigEntry)
}

func (v *configEntryListView) Result(index uint64) any {
	entries := make([]structs.ConfigEntry, 0, len(v.state))
	for _, entry := range v.state {
		entries = append(entries, entry)
	}

	return &structs.IndexedConfigEntries{
		Kind:    v.kind,
		Entries: entries,
		QueryMeta: structs.QueryMeta{
			Index:   index,
			Backend: structs.QueryBackendStreaming,
		},
	}
}

func (v *configEntryListView) Update(events []*pbsubscribe.Event) error {
	for _, event := range filterByEnterpriseMeta(events, v.entMeta) {
		update := event.GetConfigEntry()
		configEntry := pbconfigentry.ConfigEntryToStructs(update.ConfigEntry)
		name := structs.NewServiceName(configEntry.GetName(), configEntry.GetEnterpriseMeta()).String()

		switch update.Op {
		case pbsubscribe.ConfigEntryUpdate_Delete:
			delete(v.state, name)
		case pbsubscribe.ConfigEntryUpdate_Upsert:
			v.state[name] = configEntry
		}
	}
	return nil
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
