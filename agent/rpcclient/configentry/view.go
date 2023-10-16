// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

var _ submatview.View = (*ConfigEntryView)(nil)

// ConfigEntryView implements a submatview.View for a single config entry.
type ConfigEntryView struct {
	state structs.ConfigEntry
}

// Reset resets the state to nil for the ConfigEntryView
func (v *ConfigEntryView) Reset() {
	v.state = nil
}

// Result returns the structs.ConfigEntryResponse stored by this view.
func (v *ConfigEntryView) Result(index uint64) any {
	return &structs.ConfigEntryResponse{
		QueryMeta: structs.QueryMeta{
			Index:   index,
			Backend: structs.QueryBackendStreaming,
		},
		Entry: v.state,
	}
}

// Update updates the state containing a config entry based on events
func (v *ConfigEntryView) Update(events []*pbsubscribe.Event) error {
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

var _ submatview.View = (*ConfigEntryListView)(nil)

// ConfigEntryListView implements a submatview.View for a list of config entries
// that are all of the same kind (name is treated as unique).
type ConfigEntryListView struct {
	kind    string
	entMeta acl.EnterpriseMeta
	state   map[string]structs.ConfigEntry
}

// NewConfigEntryListView contructs a ConfigEntryListView based on the enterprise meta data and the kind
func NewConfigEntryListView(kind string, entMeta acl.EnterpriseMeta) *ConfigEntryListView {
	view := &ConfigEntryListView{kind: kind, entMeta: entMeta}
	view.Reset()
	return view
}

// Reset resets the states of the list view to an empty map of Config Entries
func (v *ConfigEntryListView) Reset() {
	v.state = make(map[string]structs.ConfigEntry)
}

// Result returns the structs.IndexedConfigEntries stored by this view.
func (v *ConfigEntryListView) Result(index uint64) any {
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

// Update updates the states containing a config entry based on events
func (v *ConfigEntryListView) Update(events []*pbsubscribe.Event) error {
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
