package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

func TestConfigEntryEventsFromChanges(t *testing.T) {
	const changeIndex uint64 = 123

	testCases := map[string]struct {
		setup  func(tx *txn) error
		mutate func(tx *txn) error
		events []stream.Event
	}{
		"upsert mesh config": {
			mutate: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.MeshConfigEntry{
					Meta: map[string]string{"foo": "bar"},
				})
			},
			events: []stream.Event{
				{
					Topic: EventTopicMeshConfig,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: &structs.MeshConfigEntry{
							Meta: map[string]string{"foo": "bar"},
						},
					},
				},
			},
		},
		"delete mesh config": {
			setup: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.MeshConfigEntry{})
			},
			mutate: func(tx *txn) error {
				return deleteConfigEntryTxn(tx, 0, structs.MeshConfig, structs.MeshConfigMesh, nil)
			},
			events: []stream.Event{
				{
					Topic: EventTopicMeshConfig,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Delete,
						Value: &structs.MeshConfigEntry{},
					},
				},
			},
		},
		"upsert service resolver": {
			mutate: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.ServiceResolverConfigEntry{
					Name: "web",
				})
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceResolver,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: &structs.ServiceResolverConfigEntry{
							Name: "web",
						},
					},
				},
			},
		},
		"delete service resolver": {
			setup: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.ServiceResolverConfigEntry{
					Name: "web",
				})
			},
			mutate: func(tx *txn) error {
				return deleteConfigEntryTxn(tx, 0, structs.ServiceResolver, "web", nil)
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceResolver,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						Value: &structs.ServiceResolverConfigEntry{
							Name: "web",
						},
					},
				},
			},
		},
		"upsert ingress gateway": {
			mutate: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.IngressGatewayConfigEntry{
					Name: "gw1",
				})
			},
			events: []stream.Event{
				{
					Topic: EventTopicIngressGateway,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: &structs.IngressGatewayConfigEntry{
							Name: "gw1",
						},
					},
				},
			},
		},
		"delete ingress gateway": {
			setup: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, &structs.IngressGatewayConfigEntry{
					Name: "gw1",
				})
			},
			mutate: func(tx *txn) error {
				return deleteConfigEntryTxn(tx, 0, structs.IngressGateway, "gw1", nil)
			},
			events: []stream.Event{
				{
					Topic: EventTopicIngressGateway,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						Value: &structs.IngressGatewayConfigEntry{
							Name: "gw1",
						},
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			store := testStateStore(t)

			if tc.setup != nil {
				tx := store.db.WriteTxn(0)
				require.NoError(t, tc.setup(tx))
				require.NoError(t, tx.Commit())
			}

			tx := store.db.WriteTxn(0)
			t.Cleanup(tx.Abort)

			if tc.mutate != nil {
				require.NoError(t, tc.mutate(tx))
			}

			events, err := ConfigEntryEventsFromChanges(tx, Changes{Index: changeIndex, Changes: tx.Changes()})
			require.NoError(t, err)
			require.Equal(t, tc.events, events)
		})
	}
}

func TestMeshConfigSnapshot(t *testing.T) {
	const index uint64 = 123

	entry := &structs.MeshConfigEntry{
		Meta: map[string]string{"foo": "bar"},
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, entry))

	testCases := map[string]stream.Subject{
		"named entry": EventSubjectConfigEntry{Name: structs.MeshConfigMesh},
		"wildcard":    stream.SubjectWildcard,
	}
	for desc, subject := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.MeshConfigSnapshot(stream.SubscribeRequest{Subject: subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)

			require.Len(t, buf.events, 1)
			require.Len(t, buf.events[0], 1)

			payload := buf.events[0][0].Payload.(EventPayloadConfigEntry)
			require.Equal(t, pbsubscribe.ConfigEntryUpdate_Upsert, payload.Op)
			require.Equal(t, entry, payload.Value)
		})
	}
}

func TestServiceResolverSnapshot(t *testing.T) {
	const index uint64 = 123

	webResolver := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "web",
	}
	dbResolver := &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "db",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, webResolver))
	require.NoError(t, store.EnsureConfigEntry(index, dbResolver))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: "web"},
			events: []stream.Event{
				{
					Topic: EventTopicServiceResolver,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: webResolver,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicServiceResolver,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: webResolver,
					},
				},
				{
					Topic: EventTopicServiceResolver,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: dbResolver,
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.ServiceResolverSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestIngressGatewaySnapshot(t *testing.T) {
	const index uint64 = 123

	gw1 := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gw1",
	}
	gw2 := &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "gw2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, gw1))
	require.NoError(t, store.EnsureConfigEntry(index, gw2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: gw1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicIngressGateway,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: gw1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicIngressGateway,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: gw1,
					},
				},
				{
					Topic: EventTopicIngressGateway,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: gw2,
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.IngressGatewaySnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}
