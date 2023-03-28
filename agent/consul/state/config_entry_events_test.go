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
				return ensureConfigEntryTxn(tx, 0, false, &structs.MeshConfigEntry{
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
				return ensureConfigEntryTxn(tx, 0, false, &structs.MeshConfigEntry{})
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
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceResolverConfigEntry{
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
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceResolverConfigEntry{
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
				return ensureConfigEntryTxn(tx, 0, false, &structs.IngressGatewayConfigEntry{
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
				return ensureConfigEntryTxn(tx, 0, false, &structs.IngressGatewayConfigEntry{
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
		"upsert service intentions": {
			mutate: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceIntentionsConfigEntry{
					Name: "web",
				})
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceIntentions,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: &structs.ServiceIntentionsConfigEntry{
							Name: "web",
						},
					},
				},
			},
		},
		"delete service intentions": {
			setup: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceIntentionsConfigEntry{
					Name: "web",
				})
			},
			mutate: func(tx *txn) error {
				return deleteConfigEntryTxn(tx, 0, structs.ServiceIntentions, "web", nil)
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceIntentions,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						Value: &structs.ServiceIntentionsConfigEntry{
							Name: "web",
						},
					},
				},
			},
		},
		"upsert service defaults": {
			mutate: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceConfigEntry{
					Name: "web",
				})
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceDefaults,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: &structs.ServiceConfigEntry{
							Name: "web",
						},
					},
				},
			},
		},
		"delete service defaults": {
			setup: func(tx *txn) error {
				return ensureConfigEntryTxn(tx, 0, false, &structs.ServiceConfigEntry{
					Name: "web",
				})
			},
			mutate: func(tx *txn) error {
				return deleteConfigEntryTxn(tx, 0, structs.ServiceDefaults, "web", nil)
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceDefaults,
					Index: changeIndex,
					Payload: EventPayloadConfigEntry{
						Op: pbsubscribe.ConfigEntryUpdate_Delete,
						Value: &structs.ServiceConfigEntry{
							Name: "web",
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

func TestServiceIntentionsSnapshot(t *testing.T) {
	const index uint64 = 123

	ixn1 := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "svc1",
	}
	ixn2 := &structs.ServiceIntentionsConfigEntry{
		Kind: structs.ServiceIntentions,
		Name: "svc2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, ixn1))
	require.NoError(t, store.EnsureConfigEntry(index, ixn2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: ixn1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicServiceIntentions,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicServiceIntentions,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn1,
					},
				},
				{
					Topic: EventTopicServiceIntentions,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn2,
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.ServiceIntentionsSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestServiceDefaultsSnapshot(t *testing.T) {
	const index uint64 = 123

	ixn1 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "svc1",
	}
	ixn2 := &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "svc2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, ixn1))
	require.NoError(t, store.EnsureConfigEntry(index, ixn2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: ixn1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicServiceDefaults,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicServiceDefaults,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn1,
					},
				},
				{
					Topic: EventTopicServiceDefaults,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: ixn2,
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.ServiceDefaultsSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestAPIGatewaySnapshot(t *testing.T) {
	const index uint64 = 123

	gw1 := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "agw1",
	}
	gw2 := &structs.APIGatewayConfigEntry{
		Kind: structs.APIGateway,
		Name: "agw2",
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
					Topic: EventTopicAPIGateway,
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
					Topic: EventTopicAPIGateway,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: gw1,
					},
				},
				{
					Topic: EventTopicAPIGateway,
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

			idx, err := store.APIGatewaySnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestTCPRouteSnapshot(t *testing.T) {
	const index uint64 = 123

	rt1 := &structs.TCPRouteConfigEntry{
		Kind: structs.TCPRoute,
		Name: "tcprt1",
	}
	rt2 := &structs.TCPRouteConfigEntry{
		Kind: structs.TCPRoute,
		Name: "tcprt2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, rt1))
	require.NoError(t, store.EnsureConfigEntry(index, rt2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: rt1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicTCPRoute,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: rt1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicTCPRoute,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: rt1,
					},
				},
				{
					Topic: EventTopicTCPRoute,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: rt2,
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.TCPRouteSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestHTTPRouteSnapshot(t *testing.T) {
	const index uint64 = 123

	rt1 := &structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "httprt1",
	}
	gw2 := &structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "httprt2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, rt1))
	require.NoError(t, store.EnsureConfigEntry(index, gw2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: rt1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicHTTPRoute,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: rt1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicHTTPRoute,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: rt1,
					},
				},
				{
					Topic: EventTopicHTTPRoute,
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

			idx, err := store.HTTPRouteSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestInlineCertificateSnapshot(t *testing.T) {
	const index uint64 = 123

	crt1 := &structs.InlineCertificateConfigEntry{
		Kind: structs.InlineCertificate,
		Name: "inlinecert1",
	}
	crt2 := &structs.InlineCertificateConfigEntry{
		Kind: structs.InlineCertificate,
		Name: "inlinecert2",
	}

	store := testStateStore(t)
	require.NoError(t, store.EnsureConfigEntry(index, crt1))
	require.NoError(t, store.EnsureConfigEntry(index, crt2))

	testCases := map[string]struct {
		subject stream.Subject
		events  []stream.Event
	}{
		"named entry": {
			subject: EventSubjectConfigEntry{Name: crt1.Name},
			events: []stream.Event{
				{
					Topic: EventTopicInlineCertificate,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: crt1,
					},
				},
			},
		},
		"wildcard": {
			subject: stream.SubjectWildcard,
			events: []stream.Event{
				{
					Topic: EventTopicInlineCertificate,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: crt1,
					},
				},
				{
					Topic: EventTopicInlineCertificate,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: crt2,
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			buf := &snapshotAppender{}

			idx, err := store.InlineCertificateSnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}

func TestBoundAPIGatewaySnapshot(t *testing.T) {
	const index uint64 = 123

	gw1 := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "boundapigw1",
	}
	gw2 := &structs.BoundAPIGatewayConfigEntry{
		Kind: structs.BoundAPIGateway,
		Name: "boundapigw2",
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
					Topic: EventTopicBoundAPIGateway,
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
					Topic: EventTopicBoundAPIGateway,
					Index: index,
					Payload: EventPayloadConfigEntry{
						Op:    pbsubscribe.ConfigEntryUpdate_Upsert,
						Value: gw1,
					},
				},
				{
					Topic: EventTopicBoundAPIGateway,
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

			idx, err := store.BoundAPIGatewaySnapshot(stream.SubscribeRequest{Subject: tc.subject}, buf)
			require.NoError(t, err)
			require.Equal(t, index, idx)
			require.Len(t, buf.events, 1)
			require.ElementsMatch(t, tc.events, buf.events[0])
		})
	}
}
