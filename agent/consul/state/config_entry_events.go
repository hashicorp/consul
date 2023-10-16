// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// Adding events for a new config entry kind? Remember to update ConfigEntryFromStructs and ConfigEntryToStructs.
var configEntryKindToTopic = map[string]stream.Topic{
	structs.MeshConfig:        EventTopicMeshConfig,
	structs.ServiceResolver:   EventTopicServiceResolver,
	structs.IngressGateway:    EventTopicIngressGateway,
	structs.ServiceIntentions: EventTopicServiceIntentions,
	structs.ServiceDefaults:   EventTopicServiceDefaults,
	structs.APIGateway:        EventTopicAPIGateway,
	structs.TCPRoute:          EventTopicTCPRoute,
	structs.HTTPRoute:         EventTopicHTTPRoute,
	structs.InlineCertificate: EventTopicInlineCertificate,
	structs.BoundAPIGateway:   EventTopicBoundAPIGateway,
	structs.RateLimitIPConfig: EventTopicIPRateLimit,
	structs.SamenessGroup:     EventTopicSamenessGroup,
	structs.JWTProvider:       EventTopicJWTProvider,
}

// EventSubjectConfigEntry is a stream.Subject used to route and receive events
// for a specific config entry (kind is encoded in the topic).
type EventSubjectConfigEntry struct {
	Name           string
	EnterpriseMeta *acl.EnterpriseMeta
}

func (s EventSubjectConfigEntry) String() string {
	return fmt.Sprintf(
		"%s/%s/%s",
		s.EnterpriseMeta.PartitionOrDefault(),
		s.EnterpriseMeta.NamespaceOrDefault(),
		s.Name,
	)
}

type EventPayloadConfigEntry struct {
	Op    pbsubscribe.ConfigEntryUpdate_UpdateOp
	Value structs.ConfigEntry
}

func (e EventPayloadConfigEntry) Subject() stream.Subject {
	return EventSubjectConfigEntry{
		Name:           e.Value.GetName(),
		EnterpriseMeta: e.Value.GetEnterpriseMeta(),
	}
}

func (e EventPayloadConfigEntry) HasReadPermission(authz acl.Authorizer) bool {
	return e.Value.CanRead(authz) == nil
}

func (e EventPayloadConfigEntry) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index: idx,
		Payload: &pbsubscribe.Event_ConfigEntry{
			ConfigEntry: &pbsubscribe.ConfigEntryUpdate{
				Op:          e.Op,
				ConfigEntry: pbconfigentry.ConfigEntryFromStructs(e.Value),
			},
		},
	}
}

// ConfigEntryEventsFromChanges returns events that will be emitted when config
// entries change in the state store.
func ConfigEntryEventsFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	for _, c := range changes.Changes {
		if c.Table != tableConfigEntries {
			continue
		}

		configEntry := changeObject(c).(structs.ConfigEntry)
		topic, ok := configEntryKindToTopic[configEntry.GetKind()]
		if !ok {
			continue
		}

		op := pbsubscribe.ConfigEntryUpdate_Upsert
		if c.Deleted() {
			op = pbsubscribe.ConfigEntryUpdate_Delete
		}
		events = append(events, configEntryEvent(topic, changes.Index, op, configEntry))
	}
	return events, nil
}

// MeshConfigSnapshot is a stream.SnapshotFunc that returns a snapshot of mesh
// config entries.
func (s *Store) MeshConfigSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.MeshConfig, req, buf)
}

// ServiceResolverSnapshot is a stream.SnapshotFunc that returns a snapshot of
// service-resolver config entries.
func (s *Store) ServiceResolverSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.ServiceResolver, req, buf)
}

// IngressGatewaySnapshot is a stream.SnapshotFunc that returns a snapshot of
// ingress-gateway config entries.
func (s *Store) IngressGatewaySnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.IngressGateway, req, buf)
}

// ServiceIntentionsSnapshot is a stream.SnapshotFunc that returns a snapshot of
// service-intentions config entries.
func (s *Store) ServiceIntentionsSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.ServiceIntentions, req, buf)
}

// ServiceDefaultsSnapshot is a stream.SnapshotFunc that returns a snapshot of
// service-defaults config entries.
func (s *Store) ServiceDefaultsSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.ServiceDefaults, req, buf)
}

// APIGatewaySnapshot is a stream.SnapshotFunc that returns a snapshot of
// api-gateway config entries.
func (s *Store) APIGatewaySnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.APIGateway, req, buf)
}

// TCPRouteSnapshot is a stream.SnapshotFunc that returns a snapshot of
// tcp-route config entries.
func (s *Store) TCPRouteSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.TCPRoute, req, buf)
}

// HTTPRouteSnapshot is a stream.SnapshotFunc that retuns a snapshot of
// http-route config entries.
func (s *Store) HTTPRouteSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.HTTPRoute, req, buf)
}

// InlineCertificateSnapshot is a stream.SnapshotFunc that returns a snapshot of
// inline-certificate config entries.
func (s *Store) InlineCertificateSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.InlineCertificate, req, buf)
}

// BoundAPIGatewaySnapshot is a stream.SnapshotFunc that returns a snapshot of
// bound-api-gateway config entries.
func (s *Store) BoundAPIGatewaySnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.BoundAPIGateway, req, buf)
}

// IPRateLimiterSnapshot is a stream.SnapshotFunc that returns a snapshot of
// "control-plane-request-limit" config entries.
func (s *Store) IPRateLimiterSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.RateLimitIPConfig, req, buf)
}

// SamenessGroupSnapshot is a stream.SnapshotFunc that returns a snapshot of
// "sameness-group" config entries.
func (s *Store) SamenessGroupSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.SamenessGroup, req, buf)
}

// JWTProviderSnapshot is a stream.SnapshotFunc that returns a snapshot of
// jwt-provider config entries.
func (s *Store) JWTProviderSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	return s.configEntrySnapshot(structs.JWTProvider, req, buf)
}

func (s *Store) configEntrySnapshot(kind string, req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	var (
		idx     uint64
		err     error
		entries []structs.ConfigEntry
	)
	if subject, ok := req.Subject.(EventSubjectConfigEntry); ok {
		var entry structs.ConfigEntry
		idx, entry, err = s.ConfigEntry(nil, kind, subject.Name, subject.EnterpriseMeta)
		if entry != nil {
			entries = []structs.ConfigEntry{entry}
		}
	} else if req.Subject == stream.SubjectWildcard {
		entMeta := structs.WildcardEnterpriseMetaInPartition(structs.WildcardSpecifier)
		idx, entries, err = s.ConfigEntriesByKind(nil, kind, entMeta)
	} else {
		return 0, fmt.Errorf("subject must be of type EventSubjectConfigEntry or be SubjectWildcard, was: %T", req.Subject)
	}

	if err != nil {
		return 0, err
	}

	if l := len(entries); l != 0 {
		topic := configEntryKindToTopic[kind]
		events := make([]stream.Event, l)
		for i, e := range entries {
			events[i] = configEntryEvent(topic, idx, pbsubscribe.ConfigEntryUpdate_Upsert, e)
		}
		buf.Append(events)
	}

	return idx, nil
}

func configEntryEvent(topic stream.Topic, idx uint64, op pbsubscribe.ConfigEntryUpdate_UpdateOp, configEntry structs.ConfigEntry) stream.Event {
	return stream.Event{
		Topic: topic,
		Index: idx,
		Payload: EventPayloadConfigEntry{
			Op:    op,
			Value: configEntry,
		},
	}
}
