// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/types"
)

func TestServiceHealthSnapshot(t *testing.T) {
	store := NewStateStore(nil)

	counter := newIndexCounter()
	err := store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "db"))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web"))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web", regNode2))
	require.NoError(t, err)

	buf := &snapshotAppender{}
	req := stream.SubscribeRequest{Topic: EventTopicServiceHealth, Subject: EventSubjectService{Key: "web"}}

	idx, err := store.ServiceHealthSnapshot(req, buf)
	require.NoError(t, err)
	require.Equal(t, counter.Last(), idx)

	expected := [][]stream.Event{
		{
			testServiceHealthEvent(t, "web", func(e *stream.Event) error {
				e.Index = counter.Last()
				csn := getPayloadCheckServiceNode(e.Payload)
				csn.Node.CreateIndex = 1
				csn.Node.ModifyIndex = 1
				csn.Service.CreateIndex = 2
				csn.Service.ModifyIndex = 2
				csn.Checks[0].CreateIndex = 1
				csn.Checks[0].ModifyIndex = 1
				csn.Checks[1].CreateIndex = 2
				csn.Checks[1].ModifyIndex = 2
				return nil
			}),
		},
		{
			testServiceHealthEvent(t, "web", evNode2, func(e *stream.Event) error {
				e.Index = counter.Last()
				csn := getPayloadCheckServiceNode(e.Payload)
				csn.Node.CreateIndex = 3
				csn.Node.ModifyIndex = 3
				csn.Service.CreateIndex = 3
				csn.Service.ModifyIndex = 3
				for i := range csn.Checks {
					csn.Checks[i].CreateIndex = 3
					csn.Checks[i].ModifyIndex = 3
				}
				return nil
			}),
		},
	}
	prototest.AssertDeepEqual(t, expected, buf.events, cmpEvents)
}

func TestServiceHealthSnapshot_ConnectTopic(t *testing.T) {
	store := NewStateStore(nil)

	setVirtualIPFlags(t, store)

	counter := newIndexCounter()
	err := store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "db"))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web"))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web", regSidecar))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web", regNode2))
	require.NoError(t, err)
	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "web", regNode2, regSidecar))
	require.NoError(t, err)

	configEntry := &structs.TerminatingGatewayConfigEntry{
		Kind: structs.TerminatingGateway,
		Name: "tgate1",
		Services: []structs.LinkedService{
			{
				Name:           "web",
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	err = store.EnsureConfigEntry(counter.Next(), configEntry)
	require.NoError(t, err)

	err = store.EnsureRegistration(counter.Next(), testServiceRegistration(t, "tgate1", regTerminatingGateway))
	require.NoError(t, err)

	buf := &snapshotAppender{}
	req := stream.SubscribeRequest{Subject: EventSubjectService{Key: "web"}, Topic: EventTopicServiceHealthConnect}

	idx, err := store.ServiceHealthSnapshot(req, buf)
	require.NoError(t, err)
	require.Equal(t, counter.Last(), idx)

	expected := [][]stream.Event{
		{
			testServiceHealthEvent(t, "web", evConnectTopic, evSidecar, func(e *stream.Event) error {
				e.Index = counter.Last()
				ep := e.Payload.(EventPayloadCheckServiceNode)
				e.Payload = ep
				csn := ep.Value
				csn.Node.CreateIndex = 1
				csn.Node.ModifyIndex = 1
				csn.Service.CreateIndex = 3
				csn.Service.ModifyIndex = 3
				csn.Checks[0].CreateIndex = 1
				csn.Checks[0].ModifyIndex = 1
				csn.Checks[1].CreateIndex = 3
				csn.Checks[1].ModifyIndex = 3
				return nil
			}),
		},
		{
			testServiceHealthEvent(t, "web", evConnectTopic, evNode2, evSidecar, func(e *stream.Event) error {
				e.Index = counter.Last()
				ep := e.Payload.(EventPayloadCheckServiceNode)
				e.Payload = ep
				csn := ep.Value
				csn.Node.CreateIndex = 4
				csn.Node.ModifyIndex = 4
				csn.Service.CreateIndex = 5
				csn.Service.ModifyIndex = 5
				csn.Checks[0].CreateIndex = 4
				csn.Checks[0].ModifyIndex = 4
				csn.Checks[1].CreateIndex = 5
				csn.Checks[1].ModifyIndex = 5
				return nil
			}),
		},
		{
			testServiceHealthEvent(t, "tgate1",
				evConnectTopic,
				evServiceTermingGateway("web"),
				func(e *stream.Event) error {
					e.Index = counter.Last()
					ep := e.Payload.(EventPayloadCheckServiceNode)
					e.Payload = ep
					csn := ep.Value
					csn.Node.CreateIndex = 1
					csn.Node.ModifyIndex = 1
					csn.Service.CreateIndex = 7
					csn.Service.ModifyIndex = 7
					csn.Checks[0].CreateIndex = 1
					csn.Checks[0].ModifyIndex = 1
					csn.Checks[1].CreateIndex = 7
					csn.Checks[1].ModifyIndex = 7
					return nil
				}),
		},
	}
	prototest.AssertDeepEqual(t, expected, buf.events, cmpEvents)
}

type snapshotAppender struct {
	events [][]stream.Event
}

func (s *snapshotAppender) Append(events []stream.Event) {
	s.events = append(s.events, events)
}

type indexCounter struct {
	value uint64
}

func (c *indexCounter) Next() uint64 {
	c.value++
	return c.value
}

func (c *indexCounter) Last() uint64 {
	return c.value
}

func newIndexCounter() *indexCounter {
	return &indexCounter{}
}

var _ stream.SnapshotAppender = (*snapshotAppender)(nil)

type eventsTestCase struct {
	Name       string
	Setup      func(s *Store, tx *txn) error
	Mutate     func(s *Store, tx *txn) error
	WantEvents []stream.Event
	WantErr    bool
}

func TestServiceHealthEventsFromChanges(t *testing.T) {
	setupIndex := uint64(10)

	run := func(t *testing.T, tc eventsTestCase) {
		t.Helper()
		runCase(t, tc.Name, tc.run)
	}

	run(t, eventsTestCase{
		Name: "irrelevant events",
		Mutate: func(s *Store, tx *txn) error {
			return kvsSetTxn(tx, tx.Index, &structs.DirEntry{
				Key:   "foo",
				Value: []byte("bar"),
			}, false)
		},
		WantEvents: nil,
		WantErr:    false,
	})
	run(t, eventsTestCase{
		Name: "service reg, new node",
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t, "web"),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "service reg, existing node",
		Setup: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false)
		},
		WantEvents: []stream.Event{
			// Should only publish new service
			testServiceHealthEvent(t, "web", evNodeUnchanged),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "service dereg, existing node",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.deleteServiceTxn(tx, tx.Index, "node1", "web", nil, "")
		},
		WantEvents: []stream.Event{
			// Should only publish deregistration for that service
			testServiceHealthDeregistrationEvent(t, "web"),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "node dereg",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.deleteNodeTxn(tx, tx.Index, "node1", nil, "")
		},
		WantEvents: []stream.Event{
			// Should publish deregistration events for all services
			testServiceHealthDeregistrationEvent(t, "db"),
			testServiceHealthDeregistrationEvent(t, "web"),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect native reg, new node",
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web", regConnectNative), false)
		},
		WantEvents: []stream.Event{
			// We should see both a regular service health event as well as a connect
			// one.
			testServiceHealthEvent(t, "web", evConnectNative),
			testServiceHealthEvent(t, "web", evConnectNative, evConnectTopic),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect native reg, existing node",
		Setup: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "db"), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web", regConnectNative), false)
		},
		WantEvents: []stream.Event{
			// We should see both a regular service health event as well as a connect
			// one.
			testServiceHealthEvent(t, "web",
				evNodeUnchanged,
				evConnectNative),
			testServiceHealthEvent(t, "web",
				evNodeUnchanged,
				evConnectNative,
				evConnectTopic),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect native dereg, existing node",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "db"), false); err != nil {
				return err
			}

			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web", regConnectNative), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.deleteServiceTxn(tx, tx.Index, "node1", "web", nil, "")
		},
		WantEvents: []stream.Event{
			// We should see both a regular service dereg event and a connect one
			testServiceHealthDeregistrationEvent(t, "web", evConnectNative),
			testServiceHealthDeregistrationEvent(t, "web", evConnectNative, evConnectTopic),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar reg, new node",
		Mutate: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "web", regSidecar), false)
		},
		WantEvents: []stream.Event{
			// We should see both a regular service health event for the web service
			// another for the sidecar service and a connect event for web.
			testServiceHealthEvent(t, "web"),
			testServiceHealthEvent(t, "web", evSidecar),
			testServiceHealthEvent(t, "web", evConnectTopic, evSidecar),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar reg, existing node",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		WantEvents: []stream.Event{
			// We should see both a regular service health event for the proxy
			// service and a connect one for the target service.
			testServiceHealthEvent(t, "web", evSidecar, evNodeUnchanged),
			testServiceHealthEvent(t, "web", evConnectTopic, evSidecar, evNodeUnchanged),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar dereg, existing node",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			// Delete only the sidecar
			return s.deleteServiceTxn(tx, tx.Index, "node1", "web_sidecar_proxy", nil, "")
		},
		WantEvents: []stream.Event{
			// We should see both a regular service dereg event and a connect one
			testServiceHealthDeregistrationEvent(t, "web", evSidecar),
			testServiceHealthDeregistrationEvent(t, "web", evConnectTopic, evSidecar),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar mutate svc",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change port of the target service instance
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regMutatePort), false)
		},
		WantEvents: []stream.Event{
			// We should see the service topic update but not connect since proxy
			// details didn't change.
			testServiceHealthEvent(t, "web",
				evMutatePort,
				evNodeUnchanged,
				evServiceMutated,
				evChecksUnchanged,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar mutate sidecar",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change port of the sidecar service instance
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regMutatePort), false)
		},
		WantEvents: []stream.Event{
			// We should see the proxy service topic update and a connect update
			testServiceHealthEvent(t, "web",
				evSidecar,
				evMutatePort,
				evNodeUnchanged,
				evServiceMutated,
				evChecksUnchanged),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeUnchanged,
				evMutatePort,
				evServiceMutated,
				evChecksUnchanged),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar rename service",
		Setup: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change service name but not ID, update proxy too
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regRenameService), false); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regRenameService), false)
		},
		WantEvents: []stream.Event{
			// We should see events to deregister the old service instance and the
			// old connect instance since we changed topic key for both. Then new
			// service and connect registrations. The proxy instance should also
			// change since it's not proxying a different service.
			testServiceHealthDeregistrationEvent(t, "web"),
			testServiceHealthEvent(t, "web",
				evRenameService,
				evServiceMutated,
				evNodeUnchanged,
				evServiceChecksMutated,
			),
			testServiceHealthDeregistrationEvent(t, "web",
				evConnectTopic,
				evSidecar,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evRenameService,
				evNodeUnchanged,
				evServiceMutated,
				evChecksUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeUnchanged,
				evRenameService,
				evServiceMutated,
				evChecksUnchanged,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "connect sidecar change destination service",
		Setup: func(s *Store, tx *txn) error {
			// Register a web_changed service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web_changed"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// And a sidecar initially for web, will be moved to target web_changed
			// in Mutate.
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change only the destination service of the proxy without a service
			// rename or deleting and recreating the proxy. This is far fetched but
			// still valid.
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regRenameService), false)
		},
		WantEvents: []stream.Event{
			// We should only see service health events for the sidecar service
			// since the actual target services didn't change. But also should see
			// Connect topic dereg for the old name to update existing subscribers
			// for Connect/web.
			testServiceHealthDeregistrationEvent(t, "web",
				evConnectTopic,
				evSidecar,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evRenameService,
				evNodeUnchanged,
				evServiceMutated,
				evChecksUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeUnchanged,
				evRenameService,
				evServiceMutated,
				evChecksUnchanged,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node update",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change only the node meta.
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testNodeRegistration(t, regNodeMeta), false)
		},
		WantEvents: []stream.Event{
			// We should see updates for all services and a connect update for the
			// sidecar's destination.
			testServiceHealthEvent(t, "db",
				evNodeMeta,
				evNodeMutated,
				evServiceUnchanged,
				evChecksUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evNodeMeta,
				evNodeMutated,
				evServiceUnchanged,
				evChecksUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evNodeMeta,
				evNodeMutated,
				evServiceUnchanged,
				evChecksUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeMeta,
				evNodeMutated,
				evServiceUnchanged,
				evChecksUnchanged,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node rename",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change only the node NAME but not it's ID. We do it for every service
			// though since this is effectively what client agent anti-entropy would
			// do on a node rename. If we only rename the node it will have no
			// services registered afterwards.
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db", regRenameNode), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regRenameNode), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regRenameNode), false); err != nil {
				return err
			}
			return nil
		},
		WantEvents: []stream.Event{
			// Node rename is implemented internally as a node delete and new node
			// insert after some renaming validation. So we should see full set of
			// new events for health, then the deletions of old services, then the
			// connect update and delete pair.
			testServiceHealthEvent(t, "db",
				evRenameNode,
				// Although we delete and re-insert, we do maintain the CreatedIndex
				// of the node record from the old one.
				evNodeMutated,
			),
			testServiceHealthEvent(t, "web",
				evRenameNode,
				evNodeMutated,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evRenameNode,
				evNodeMutated,
			),
			// dereg events for old node name services
			testServiceHealthDeregistrationEvent(t, "db"),
			testServiceHealthDeregistrationEvent(t, "web"),
			testServiceHealthDeregistrationEvent(t, "web", evSidecar),
			// Connect topic updates are last due to the way we add them
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evRenameNode,
				evNodeMutated,
			),
			testServiceHealthDeregistrationEvent(t, "web", evConnectTopic, evSidecar),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node check failure",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change only the node-level check status
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regNodeCheckFail), false); err != nil {
				return err
			}
			return nil
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t, "db",
				evNodeCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				// Only the node check changed. This needs to come after evNodeUnchanged
				evNodeChecksMutated,
			),
			testServiceHealthEvent(t, "web",
				evNodeCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evNodeChecksMutated,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evNodeCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evNodeChecksMutated,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evNodeChecksMutated,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node service check failure",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Change the service-level check status
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regServiceCheckFail), false); err != nil {
				return err
			}
			// Also change the service-level check status for the proxy. This is
			// analogous to what would happen with an alias check on the client side
			// - the proxies check would get updated at roughly the same time as the
			// target service check updates.
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regServiceCheckFail), false); err != nil {
				return err
			}
			return nil
		},
		WantEvents: []stream.Event{
			// Should only see the events for that one service change, the sidecar
			// service and hence the connect topic for that service.
			testServiceHealthEvent(t, "web",
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceChecksMutated,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceChecksMutated,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceChecksMutated,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node node-level check delete",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Delete only the node-level check
			if err := s.deleteCheckTxn(tx, tx.Index, "node1", "serf-health", nil, ""); err != nil {
				return err
			}
			return nil
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t, "db",
				evNodeCheckDelete,
				evNodeUnchanged,
				evServiceUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evNodeCheckDelete,
				evNodeUnchanged,
				evServiceUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evNodeCheckDelete,
				evNodeUnchanged,
				evServiceUnchanged,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evNodeCheckDelete,
				evNodeUnchanged,
				evServiceUnchanged,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "multi-service node service check delete",
		Setup: func(s *Store, tx *txn) error {
			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}
			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// Delete the service-level check for the main service
			if err := s.deleteCheckTxn(tx, tx.Index, "node1", "service:web", nil, ""); err != nil {
				return err
			}
			// Also delete for a proxy
			if err := s.deleteCheckTxn(tx, tx.Index, "node1", "service:web_sidecar_proxy", nil, ""); err != nil {
				return err
			}
			return nil
		},
		WantEvents: []stream.Event{
			// Should only see the events for that one service change, the sidecar
			// service and hence the connect topic for that service.
			testServiceHealthEvent(t, "web",
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceCheckDelete,
			),
			testServiceHealthEvent(t, "web",
				evSidecar,
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceCheckDelete,
			),
			testServiceHealthEvent(t, "web",
				evConnectTopic,
				evSidecar,
				evServiceCheckFail,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceCheckDelete,
			),
		},
		WantErr: false,
	})
	run(t, eventsTestCase{
		Name: "many services on many nodes in one TX",
		Setup: func(s *Store, tx *txn) error {
			// Node1

			// Register a db service
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "db"), false); err != nil {
				return err
			}

			// Node2
			// Also a web
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regNode2), false); err != nil {
				return err
			}
			// With a connect sidecar
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar, regNode2), false); err != nil {
				return err
			}

			return nil
		},
		Mutate: func(s *Store, tx *txn) error {
			// In one transaction the operator moves the web service and it's
			// sidecar from node2 back to node1 and deletes them from node2

			if err := s.deleteServiceTxn(tx, tx.Index, "node2", "web", nil, ""); err != nil {
				return err
			}
			if err := s.deleteServiceTxn(tx, tx.Index, "node2", "web_sidecar_proxy", nil, ""); err != nil {
				return err
			}

			// Register those on node1
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web"), false); err != nil {
				return err
			}
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "web", regSidecar), false); err != nil {
				return err
			}

			// And for good measure, add a new connect-native service to node2
			if err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "api", regConnectNative, regNode2), false); err != nil {
				return err
			}

			return nil
		},

		WantEvents: []stream.Event{
			// We should see:
			//  - service dereg for web and proxy on node2
			//  - connect dereg for web on node2
			//  - service reg for web and proxy on node1
			//  - connect reg for web on node1
			//  - service reg for api on node2
			//  - connect reg for api on node2
			testServiceHealthDeregistrationEvent(t, "web", evNode2),
			testServiceHealthDeregistrationEvent(t, "web", evNode2, evSidecar),
			testServiceHealthDeregistrationEvent(t, "web", evConnectTopic, evNode2, evSidecar),

			testServiceHealthEvent(t, "web", evNodeUnchanged),
			testServiceHealthEvent(t, "web", evSidecar, evNodeUnchanged),
			testServiceHealthEvent(t, "web", evConnectTopic, evSidecar, evNodeUnchanged),

			testServiceHealthEvent(t, "api", evNode2, evConnectNative, evNodeUnchanged, evVirtualIPChanged("240.0.0.2")),
			testServiceHealthEvent(t, "api", evNode2, evConnectTopic, evConnectNative, evNodeUnchanged, evVirtualIPChanged("240.0.0.2")),
		},
	})
	run(t, eventsTestCase{
		Name: "terminating gateway registered with no config entry",
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway("tgate1")),
		},
	})
	run(t, eventsTestCase{
		Name: "config entry created with no terminating gateway instance",
		Mutate: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		WantEvents: []stream.Event{},
	})
	run(t, eventsTestCase{
		Name: "terminating gateway registered after config entry exists",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		Mutate: func(s *Store, tx *txn) error {
			if err := s.ensureRegistrationTxn(
				tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false,
			); err != nil {
				return err
			}
			return s.ensureRegistrationTxn(
				tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway, regNode2), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway("tgate1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway("tgate1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNode2),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNode2),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNode2),
		},
	})
	run(t, eventsTestCase{
		Name: "terminating gateway updated after config entry exists",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(
				tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(
				tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway, regNodeCheckFail), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway("tgate1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNodeCheckFail,
				evNodeUnchanged,
				evNodeChecksMutated,
				evServiceUnchanged),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNodeCheckFail,
				evNodeUnchanged,
				evNodeChecksMutated,
				evServiceUnchanged),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evNodeCheckFail,
				evNodeUnchanged,
				evNodeChecksMutated,
				evServiceUnchanged),
		},
	})
	run(t, eventsTestCase{
		Name: "terminating gateway config entry created after gateway exists",
		Setup: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
		},
	})
	run(t, eventsTestCase{
		Name: "change the terminating gateway config entry to add a linked service",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
		},
	})
	run(t, eventsTestCase{
		Name: "change the terminating gateway config entry to remove a linked service",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evTerminatingGatewayVirtualIP("srv2", "240.0.0.2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIP("srv2", "240.0.0.2"),
				evServiceMutatedModifyIndex),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIP("srv2", "240.0.0.2"),
				evServiceIndex(setupIndex),
				evServiceMutatedModifyIndex),
		},
	})
	run(t, eventsTestCase{
		Name: "update a linked service within a terminating gateway config entry",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
			if err != nil {
				return err
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						CAFile:         "foo.crt",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1")),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evServiceIndex(setupIndex)),
		},
	})
	run(t, eventsTestCase{
		Name: "delete a terminating gateway config entry with a linked service",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			err = s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(
				tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway, regNode2), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return deleteConfigEntryTxn(tx, tx.Index, structs.TerminatingGateway, "tgate1", structs.DefaultEnterpriseMetaInDefaultPartition())
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1")),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evNode2),
		},
	})
	run(t, eventsTestCase{
		Name: "create an instance of a linked service in a terminating gateway",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "srv1"), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t, "srv1", evNodeUnchanged),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1"),
			),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceTermingGateway("srv1")),
		},
	})
	run(t, eventsTestCase{
		Name: "delete an instance of a linked service in a terminating gateway",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			err = s.ensureRegistrationTxn(tx, tx.Index, false, testServiceRegistration(t, "srv1"), false)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.deleteServiceTxn(tx, tx.Index, "node1", "srv1", nil, "")
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t, "srv1"),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1"),
			),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceTermingGateway("srv1")),
		},
	})
	run(t, eventsTestCase{
		Name: "rename a terminating gateway instance",
		Setup: func(s *Store, tx *txn) error {
			err := s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
			if err != nil {
				return err
			}

			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err = ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			configEntry = &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate2",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
		},
		Mutate: func(s *Store, tx *txn) error {
			rename := func(req *structs.RegisterRequest) error {
				req.Service.Service = "tgate2"
				req.Checks[1].ServiceName = "tgate2"
				return nil
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway, rename), false)
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evTerminatingGatewayVirtualIPs("srv1")),
			testServiceHealthEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evNodeUnchanged,
				evServiceMutated,
				evServiceChecksMutated,
				evTerminatingGatewayRenamed("tgate2"),
				evTerminatingGatewayVirtualIPs("srv1")),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1")),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evNodeUnchanged,
				evServiceMutated,
				evServiceChecksMutated,
				evTerminatingGatewayRenamed("tgate2")),
		},
	})
	run(t, eventsTestCase{
		Name: "delete a terminating gateway instance",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "srv1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Name:           "srv2",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			return s.deleteServiceTxn(tx, tx.Index, "node1", "tgate1", structs.DefaultEnterpriseMetaInDefaultPartition(), "")
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evServiceTermingGateway(""),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv1"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("srv2"),
				evTerminatingGatewayVirtualIPs("srv1", "srv2")),
		},
	})
	run(t, eventsTestCase{
		Name: "terminating gateway destination service-defaults",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "destination1",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntryDest := &structs.ServiceConfigEntry{
				Kind:        structs.ServiceDefaults,
				Name:        "destination1",
				Destination: &structs.DestinationConfig{Port: 9000, Addresses: []string{"kafka.test.com"}},
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntryDest)
		},
		WantEvents: []stream.Event{
			testServiceHealthDeregistrationEvent(t,
				"tgate1",
				evConnectTopic,
				evServiceTermingGateway("destination1"),
				evTerminatingGatewayVirtualIPs("destination1")),
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceTermingGateway("destination1"),
				evTerminatingGatewayVirtualIPs("destination1"),
			),
		},
	})

	run(t, eventsTestCase{
		Name: "terminating gateway destination service-defaults wildcard",
		Setup: func(s *Store, tx *txn) error {
			configEntry := &structs.TerminatingGatewayConfigEntry{
				Kind: structs.TerminatingGateway,
				Name: "tgate1",
				Services: []structs.LinkedService{
					{
						Name:           "*",
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}
			err := ensureConfigEntryTxn(tx, tx.Index, false, configEntry)
			if err != nil {
				return err
			}
			return s.ensureRegistrationTxn(tx, tx.Index, false,
				testServiceRegistration(t, "tgate1", regTerminatingGateway), false)
		},
		Mutate: func(s *Store, tx *txn) error {
			configEntryDest := &structs.ServiceConfigEntry{
				Kind:        structs.ServiceDefaults,
				Name:        "destination1",
				Destination: &structs.DestinationConfig{Port: 9000, Addresses: []string{"kafka.test.com"}},
			}
			return ensureConfigEntryTxn(tx, tx.Index, false, configEntryDest)
		},
		WantEvents: []stream.Event{
			testServiceHealthEvent(t,
				"tgate1",
				evConnectTopic,
				evNodeUnchanged,
				evServiceUnchanged,
				evServiceTermingGateway("destination1"),
				evTerminatingGatewayVirtualIPs("*"),
			),
		},
	})
}

func (tc eventsTestCase) run(t *testing.T) {
	s := NewStateStore(nil)
	require.NoError(t, s.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}))
	require.NoError(t, s.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataTermGatewayVirtualIPsEnabled,
		Value: "true",
	}))

	setupIndex := uint64(10)
	mutateIndex := uint64(100)

	if tc.Setup != nil {
		// Bypass the publish mechanism for this test or we get into odd
		// recursive stuff...
		setupTx := s.db.WriteTxn(setupIndex)
		require.NoError(t, tc.Setup(s, setupTx))
		// Commit the underlying transaction without using wrapped Commit so we
		// avoid the whole event publishing system for setup here. It _should_
		// work but it makes debugging test hard as it will call the function
		// under test for the setup data...
		setupTx.Txn.Commit()
	}

	tx := s.db.WriteTxn(mutateIndex)
	require.NoError(t, tc.Mutate(s, tx))

	// Note we call the func under test directly rather than publishChanges so
	// we can test this in isolation.
	got, err := ServiceHealthEventsFromChanges(tx, Changes{Changes: tx.Changes(), Index: 100})
	if tc.WantErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	prototest.AssertDeepEqual(t, tc.WantEvents, got, cmpPartialOrderEvents, cmpopts.EquateEmpty())
}

func runCase(t *testing.T, name string, fn func(t *testing.T)) bool {
	t.Helper()
	return t.Run(name, func(t *testing.T) {
		t.Helper()
		t.Log("case:", name)
		fn(t)
	})
}

func regTerminatingGateway(req *structs.RegisterRequest) error {
	req.Service.Kind = structs.ServiceKindTerminatingGateway
	req.Service.Port = 22000
	return nil
}

func evServiceTermingGateway(name string) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		csn := getPayloadCheckServiceNode(e.Payload)

		csn.Service.Kind = structs.ServiceKindTerminatingGateway
		csn.Service.Port = 22000

		sn := structs.NewServiceName(name, &csn.Service.EnterpriseMeta)
		key := structs.ServiceGatewayVirtualIPTag(sn)
		if name != "" && name != csn.Service.Service {
			csn.Service.TaggedAddresses = map[string]structs.ServiceAddress{
				key: {Address: "240.0.0.1"},
			}
		}

		if e.Topic == EventTopicServiceHealthConnect {
			payload := e.Payload.(EventPayloadCheckServiceNode)
			payload.overrideKey = name
			e.Payload = payload
		}
		return nil
	}
}

func evTerminatingGatewayVirtualIP(name, addr string) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		csn := getPayloadCheckServiceNode(e.Payload)

		sn := structs.NewServiceName(name, &csn.Service.EnterpriseMeta)
		key := structs.ServiceGatewayVirtualIPTag(sn)
		csn.Service.TaggedAddresses = map[string]structs.ServiceAddress{
			key: {Address: addr},
		}

		return nil
	}
}

func evTerminatingGatewayVirtualIPs(names ...string) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		csn := getPayloadCheckServiceNode(e.Payload)

		if len(names) > 0 {
			csn.Service.TaggedAddresses = make(map[string]structs.ServiceAddress)
		}
		for i, name := range names {
			sn := structs.NewServiceName(name, &csn.Service.EnterpriseMeta)
			key := structs.ServiceGatewayVirtualIPTag(sn)

			csn.Service.TaggedAddresses[key] = structs.ServiceAddress{
				Address: fmt.Sprintf("240.0.0.%d", i+1),
			}
		}

		return nil
	}
}

func evServiceIndex(idx uint64) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		payload := e.Payload.(EventPayloadCheckServiceNode)
		payload.Value.Node.CreateIndex = idx
		payload.Value.Node.ModifyIndex = idx
		payload.Value.Service.CreateIndex = idx
		payload.Value.Service.ModifyIndex = idx
		for _, check := range payload.Value.Checks {
			check.CreateIndex = idx
			check.ModifyIndex = idx
		}
		e.Payload = payload

		return nil
	}
}

// cmpPartialOrderEvents returns a compare option which sorts events so that
// all events for a particular topic are grouped together. The sort is
// stable so events with the same key retain their relative order.
//
// This sort should match the logic in EventPayloadCheckServiceNode.Subject
// to avoid masking bugs.
var cmpPartialOrderEvents = cmp.Options{
	cmpopts.SortSlices(func(i, j stream.Event) bool {
		key := func(e stream.Event) string {
			payload := e.Payload.(EventPayloadCheckServiceNode)
			csn := payload.Value

			name := csn.Service.Service
			if payload.overrideKey != "" {
				name = payload.overrideKey
			}
			ns := csn.Service.EnterpriseMeta.NamespaceOrDefault()
			if payload.overrideNamespace != "" {
				ns = payload.overrideNamespace
			}
			ap := csn.Service.EnterpriseMeta.PartitionOrDefault()
			if payload.overridePartition != "" {
				ap = payload.overridePartition
			}
			return fmt.Sprintf("%s/%s/%s/%s/%s", e.Topic, ap, csn.Node.Node, ns, name)
		}
		return key(i) < key(j)
	}),
	cmpEvents,
}

var cmpEvents = cmp.Options{
	cmp.AllowUnexported(EventPayloadCheckServiceNode{}),
}

type regOption func(req *structs.RegisterRequest) error

func testNodeRegistration(t *testing.T, opts ...regOption) *structs.RegisterRequest {
	r := &structs.RegisterRequest{
		Datacenter:     "dc1",
		ID:             "11111111-2222-3333-4444-555555555555",
		Node:           "node1",
		Address:        "10.10.10.10",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		Checks: structs.HealthChecks{
			&structs.HealthCheck{
				CheckID: "serf-health",
				Name:    "serf-health",
				Node:    "node1",
				Status:  api.HealthPassing,
			},
		},
	}
	for _, opt := range opts {
		err := opt(r)
		require.NoError(t, err)
	}
	return r
}

func testServiceRegistration(t *testing.T, svc string, opts ...regOption) *structs.RegisterRequest {
	// note: don't pass opts or they might get applied twice!
	r := testNodeRegistration(t)
	r.Service = &structs.NodeService{
		ID:             svc,
		Service:        svc,
		Port:           8080,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	r.Checks = append(r.Checks,
		&structs.HealthCheck{
			CheckID:        types.CheckID("service:" + svc),
			Name:           "service:" + svc,
			Node:           "node1",
			ServiceID:      svc,
			ServiceName:    svc,
			Type:           "ttl",
			Status:         api.HealthPassing,
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		})
	for _, opt := range opts {
		err := opt(r)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}
	return r
}

type eventOption func(e *stream.Event) error

func testServiceHealthEvent(t *testing.T, svc string, opts ...eventOption) stream.Event {
	e := newTestEventServiceHealthRegister(100, 1, svc)

	// Normalize a few things that are different in the generic event which was
	// based on original code here but made more general. This means we don't have
	// to change all the test loads...
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Node.ID = "11111111-2222-3333-4444-555555555555"
	csn.Node.Address = "10.10.10.10"
	csn.Node.Partition = structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty()

	for _, opt := range opts {
		if err := opt(&e); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}
	return e
}

func testServiceHealthDeregistrationEvent(t *testing.T, svc string, opts ...eventOption) stream.Event {
	e := newTestEventServiceHealthDeregister(100, 1, svc)
	for _, opt := range opts {
		if err := opt(&e); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	}
	return e
}

// regConnectNative option converts the base registration into a Connect-native
// one.
func regConnectNative(req *structs.RegisterRequest) error {
	if req.Service == nil {
		return nil
	}
	req.Service.Connect.Native = true
	return nil
}

// regSidecar option converts the base registration request
// into the registration for it's sidecar service.
func regSidecar(req *structs.RegisterRequest) error {
	if req.Service == nil {
		return nil
	}
	svc := req.Service.Service

	req.Service.Kind = structs.ServiceKindConnectProxy
	req.Service.ID = svc + "_sidecar_proxy"
	req.Service.Service = svc + "_sidecar_proxy"
	req.Service.Port = 20000 + req.Service.Port

	req.Service.Proxy.DestinationServiceName = svc
	req.Service.Proxy.DestinationServiceID = svc

	// Convert the check to point to the right ID now. This isn't totally
	// realistic - sidecars should have alias checks etc but this is good enough
	// to test this code path.
	if len(req.Checks) >= 2 {
		req.Checks[1].CheckID = types.CheckID("service:" + svc + "_sidecar_proxy")
		req.Checks[1].ServiceID = svc + "_sidecar_proxy"
	}

	return nil
}

// regNodeCheckFail option converts the base registration request
// into a registration with the node-level health check failing
func regNodeCheckFail(req *structs.RegisterRequest) error {
	req.Checks[0].Status = api.HealthCritical
	return nil
}

// regServiceCheckFail option converts the base registration request
// into a registration with the service-level health check failing
func regServiceCheckFail(req *structs.RegisterRequest) error {
	req.Checks[1].Status = api.HealthCritical
	return nil
}

// regMutatePort option alters the base registration service port by a relative
// amount to simulate a service change. Can be used with regSidecar since it's a
// relative change (+10).
func regMutatePort(req *structs.RegisterRequest) error {
	if req.Service == nil {
		return nil
	}
	req.Service.Port += 10
	return nil
}

// regRenameService option alters the base registration service name but not
// it's ID simulating a service being renamed while it's ID is maintained
// separately e.g. by a scheduler. This is an edge case but an important one as
// it changes which topic key events propagate.
func regRenameService(req *structs.RegisterRequest) error {
	if req.Service == nil {
		return nil
	}
	isSidecar := req.Service.Kind == structs.ServiceKindConnectProxy

	if !isSidecar {
		req.Service.Service += "_changed"
		// Update service checks
		if len(req.Checks) >= 2 {
			req.Checks[1].ServiceName += "_changed"
		}
		return nil
	}
	// This is a sidecar, it's not really realistic but lets only update the
	// fields necessary to make it work again with the new service name to be sure
	// we get the right result. This is certainly possible if not likely so a
	// valid case.

	// We don't need to update out own details, only the name of the destination
	req.Service.Proxy.DestinationServiceName += "_changed"

	return nil
}

// regRenameNode option alters the base registration node name by adding the
// _changed suffix.
func regRenameNode(req *structs.RegisterRequest) error {
	req.Node += "_changed"
	for i := range req.Checks {
		req.Checks[i].Node = req.Node
	}
	return nil
}

// regNode2 option alters the base registration to be on a different node.
func regNode2(req *structs.RegisterRequest) error {
	req.Node = "node2"
	req.ID = "22222222-2222-3333-4444-555555555555"
	for i := range req.Checks {
		req.Checks[i].Node = req.Node
	}
	return nil
}

// regNodeMeta option alters the base registration node to add some meta data.
func regNodeMeta(req *structs.RegisterRequest) error {
	req.NodeMeta = map[string]string{"foo": "bar"}
	return nil
}

// evNodeUnchanged option converts the event to reset the node and node check
// raft indexes to the original value where we expect the node not to have been
// changed in the mutation.
func evNodeUnchanged(e *stream.Event) error {
	// If the node wasn't touched, its modified index and check's modified
	// indexes should be the original ones.
	csn := getPayloadCheckServiceNode(e.Payload)

	// Check this isn't a dereg event with made up/placeholder node info
	if csn.Node.CreateIndex == 0 {
		return nil
	}
	csn.Node.CreateIndex = 10
	csn.Node.ModifyIndex = 10
	csn.Checks[0].CreateIndex = 10
	csn.Checks[0].ModifyIndex = 10
	return nil
}

// evServiceUnchanged option converts the event to reset the service and service
// check raft indexes to the original value where we expect the service record
// not to have been changed in the mutation.
func evServiceUnchanged(e *stream.Event) error {
	// If the node wasn't touched, its modified index and check's modified
	// indexes should be the original ones.
	csn := getPayloadCheckServiceNode(e.Payload)

	csn.Service.CreateIndex = 10
	csn.Service.ModifyIndex = 10
	if len(csn.Checks) > 1 {
		csn.Checks[1].CreateIndex = 10
		csn.Checks[1].ModifyIndex = 10
	}
	return nil
}

// evConnectNative option converts the base event to represent a connect-native
// service instance.
func evConnectNative(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Service.Connect.Native = true
	csn.Service.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: "240.0.0.1",
			Port:    csn.Service.Port,
		},
	}
	return nil
}

// evConnectTopic option converts the base event to the equivalent event that
// should be published to the connect topic. When needed it should be applied
// first as several other options (notable evSidecar) change behavior subtly
// depending on which topic they are published to and they determine this from
// the event.
func evConnectTopic(e *stream.Event) error {
	e.Topic = EventTopicServiceHealthConnect
	return nil
}

// evSidecar option converts the base event to the health (not connect) event
// expected from the sidecar proxy registration for that service instead. When
// needed it should be applied after any option that changes topic (e.g.
// evConnectTopic) but before other options that might change behavior subtly
// depending on whether it's a sidecar or regular service event (e.g.
// evRenameService).
func evSidecar(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)

	svc := csn.Service.Service

	csn.Service.Kind = structs.ServiceKindConnectProxy
	csn.Service.ID = svc + "_sidecar_proxy"
	csn.Service.Service = svc + "_sidecar_proxy"
	csn.Service.Port = 20000 + csn.Service.Port

	csn.Service.Proxy.DestinationServiceName = svc
	csn.Service.Proxy.DestinationServiceID = svc

	csn.Service.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: "240.0.0.1",
			Port:    csn.Service.Port,
		},
	}

	// Convert the check to point to the right ID now. This isn't totally
	// realistic - sidecars should have alias checks etc but this is good enough
	// to test this code path.
	if len(csn.Checks) >= 2 {
		csn.Checks[1].CheckID = types.CheckID("service:" + svc + "_sidecar_proxy")
		csn.Checks[1].ServiceID = svc + "_sidecar_proxy"
		csn.Checks[1].ServiceName = svc + "_sidecar_proxy"
	}

	if e.Topic == EventTopicServiceHealthConnect {
		payload := e.Payload.(EventPayloadCheckServiceNode)
		payload.overrideKey = svc
		e.Payload = payload
	}
	return nil
}

// evMutatePort option alters the base event service port by a relative
// amount to simulate a service change. Can be used with evSidecar since it's a
// relative change (+10).
func evMutatePort(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Service.Port += 10
	if addr, ok := csn.Service.TaggedAddresses[structs.TaggedAddressVirtualIP]; ok {
		addr.Port = csn.Service.Port
		csn.Service.TaggedAddresses[structs.TaggedAddressVirtualIP] = addr
	}
	return nil
}

// evNodeMutated option alters the base event node to set it's CreateIndex
// (but not modify index) to the setup index. This expresses that we expect the
// node record originally created in setup to have been mutated during the
// update.
func evNodeMutated(e *stream.Event) error {
	getPayloadCheckServiceNode(e.Payload).Node.CreateIndex = 10
	return nil
}

// evServiceMutated option alters the base event service to set it's CreateIndex
// (but not modify index) to the setup index. This expresses that we expect the
// service record originally created in setup to have been mutated during the
// update.
func evServiceMutated(e *stream.Event) error {
	getPayloadCheckServiceNode(e.Payload).Service.CreateIndex = 10
	return nil
}

func evServiceMutatedModifyIndex(e *stream.Event) error {
	getPayloadCheckServiceNode(e.Payload).Service.ModifyIndex = 100
	return nil
}

// evServiceChecksMutated option alters the base event service check to set it's
// CreateIndex (but not modify index) to the setup index. This expresses that we
// expect the service check records originally created in setup to have been
// mutated during the update. NOTE: this must be sequenced after
// evServiceUnchanged if both are used.
func evServiceChecksMutated(e *stream.Event) error {
	getPayloadCheckServiceNode(e.Payload).Checks[1].CreateIndex = 10
	getPayloadCheckServiceNode(e.Payload).Checks[1].ModifyIndex = 100
	return nil
}

// evNodeChecksMutated option alters the base event node check to set it's
// CreateIndex (but not modify index) to the setup index. This expresses that we
// expect the node check records originally created in setup to have been
// mutated during the update. NOTE: this must be sequenced after evNodeUnchanged
// if both are used.
func evNodeChecksMutated(e *stream.Event) error {
	getPayloadCheckServiceNode(e.Payload).Checks[0].CreateIndex = 10
	getPayloadCheckServiceNode(e.Payload).Checks[0].ModifyIndex = 100
	return nil
}

// evChecksUnchanged option alters the base event service to set all check raft
// indexes to the setup index. This expresses that we expect none of the checks
// to have changed in the update.
func evChecksUnchanged(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	for i := range csn.Checks {
		csn.Checks[i].CreateIndex = 10
		csn.Checks[i].ModifyIndex = 10
	}
	return nil
}

// evRenameService option alters the base event service to change the service
// name but not ID simulating an in-place service rename.
func evRenameService(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)

	if csn.Service.Kind != structs.ServiceKindConnectProxy {
		csn.Service.Service += "_changed"
		// Update service checks
		if len(csn.Checks) >= 2 {
			csn.Checks[1].ServiceName += "_changed"
		}
		return nil
	}
	// This is a sidecar, it's not really realistic but lets only update the
	// fields necessary to make it work again with the new service name to be sure
	// we get the right result. This is certainly possible if not likely so a
	// valid case.

	// We don't need to update our own details, only the name of the destination
	csn.Service.Proxy.DestinationServiceName += "_changed"

	taggedAddr := csn.Service.TaggedAddresses[structs.TaggedAddressVirtualIP]
	taggedAddr.Address = "240.0.0.2"
	csn.Service.TaggedAddresses[structs.TaggedAddressVirtualIP] = taggedAddr

	if e.Topic == EventTopicServiceHealthConnect {
		payload := e.Payload.(EventPayloadCheckServiceNode)
		payload.overrideKey = csn.Service.Proxy.DestinationServiceName
		e.Payload = payload
	}
	return nil
}

func evTerminatingGatewayRenamed(newName string) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		csn := getPayloadCheckServiceNode(e.Payload)
		csn.Service.Service = newName
		csn.Checks[1].ServiceName = newName
		return nil
	}
}

func evVirtualIPChanged(newIP string) func(e *stream.Event) error {
	return func(e *stream.Event) error {
		csn := getPayloadCheckServiceNode(e.Payload)
		csn.Service.TaggedAddresses = map[string]structs.ServiceAddress{
			structs.TaggedAddressVirtualIP: {
				Address: newIP,
				Port:    csn.Service.Port,
			},
		}
		return nil
	}
}

// evNodeMeta option alters the base event node to add some meta data.
func evNodeMeta(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Node.Meta = map[string]string{"foo": "bar"}
	return nil
}

// evRenameNode option alters the base event node name.
func evRenameNode(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Node.Node += "_changed"
	for i := range csn.Checks {
		csn.Checks[i].Node = csn.Node.Node
	}
	return nil
}

// evNode2 option alters the base event to refer to a different node
func evNode2(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Node.Node = "node2"
	// Only change ID if it's set (e.g. it's not in a deregistration event)
	if csn.Node.ID != "" {
		csn.Node.ID = "22222222-2222-3333-4444-555555555555"
	}
	for i := range csn.Checks {
		csn.Checks[i].Node = csn.Node.Node
	}
	return nil
}

// evNodeCheckFail option alters the base event to set the node-level health
// check to be failing
func evNodeCheckFail(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Checks[0].Status = api.HealthCritical
	return nil
}

// evNodeCheckDelete option alters the base event to remove the node-level
// health check
func evNodeCheckDelete(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	// Ensure this is idempotent as we sometimes get called multiple times..
	if len(csn.Checks) > 0 && csn.Checks[0].ServiceID == "" {
		csn.Checks = csn.Checks[1:]
	}
	return nil
}

// evServiceCheckFail option alters the base event to set the service-level health
// check to be failing
func evServiceCheckFail(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	csn.Checks[1].Status = api.HealthCritical
	return nil
}

// evServiceCheckDelete option alters the base event to remove the service-level
// health check
func evServiceCheckDelete(e *stream.Event) error {
	csn := getPayloadCheckServiceNode(e.Payload)
	// Ensure this is idempotent as we sometimes get called multiple times..
	if len(csn.Checks) > 1 && csn.Checks[1].ServiceID != "" {
		csn.Checks = csn.Checks[0:1]
	}
	return nil
}

// newTestEventServiceHealthRegister returns a realistically populated service
// health registration event. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func newTestEventServiceHealthRegister(index uint64, nodeNum int, svc string) stream.Event {
	node := fmt.Sprintf("node%d", nodeNum)
	nodeID := types.NodeID(fmt.Sprintf("11111111-2222-3333-4444-%012d", nodeNum))
	addr := fmt.Sprintf("10.10.%d.%d", nodeNum/256, nodeNum%256)

	return stream.Event{
		Topic: EventTopicServiceHealth,
		Index: index,
		Payload: EventPayloadCheckServiceNode{
			Op: pbsubscribe.CatalogOp_Register,
			Value: &structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         nodeID,
					Node:       node,
					Address:    addr,
					Datacenter: "dc1",
					Partition:  structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
				},
				Service: &structs.NodeService{
					ID:      svc,
					Service: svc,
					Port:    8080,
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
					RaftIndex: structs.RaftIndex{
						CreateIndex: index,
						ModifyIndex: index,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
				Checks: []*structs.HealthCheck{
					{
						Node:    node,
						CheckID: "serf-health",
						Name:    "serf-health",
						Status:  "passing",
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
					{
						Node:        node,
						CheckID:     types.CheckID("service:" + svc),
						Name:        "service:" + svc,
						ServiceID:   svc,
						ServiceName: svc,
						Type:        "ttl",
						Status:      "passing",
						RaftIndex: structs.RaftIndex{
							CreateIndex: index,
							ModifyIndex: index,
						},
						EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
					},
				},
			},
		},
	}
}

// TestEventServiceHealthDeregister returns a realistically populated service
// health deregistration event. The nodeNum is a
// logical node and is used to create the node name ("node%d") but also change
// the node ID and IP address to make it a little more realistic for cases that
// need that. nodeNum should be less than 64k to make the IP address look
// realistic. Any other changes can be made on the returned event to avoid
// adding too many options to callers.
func newTestEventServiceHealthDeregister(index uint64, nodeNum int, svc string) stream.Event {
	return stream.Event{
		Topic: EventTopicServiceHealth,
		Index: index,
		Payload: EventPayloadCheckServiceNode{
			Op: pbsubscribe.CatalogOp_Deregister,
			Value: &structs.CheckServiceNode{
				Node: &structs.Node{
					Node:      fmt.Sprintf("node%d", nodeNum),
					Partition: structs.NodeEnterpriseMetaInDefaultPartition().PartitionOrEmpty(),
				},
				Service: &structs.NodeService{
					ID:      svc,
					Service: svc,
					Port:    8080,
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
					RaftIndex: structs.RaftIndex{
						// The original insertion index since a delete doesn't update
						// this. This magic value came from state store tests where we
						// setup at index 10 and then mutate at index 100. It can be
						// modified by the caller later and makes it easier than having
						// yet another argument in the common case.
						CreateIndex: 10,
						ModifyIndex: 10,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
				},
			},
		},
	}
}

func newPayloadCheckServiceNode(service, namespace string) EventPayloadCheckServiceNode {
	return EventPayloadCheckServiceNode{
		Value: &structs.CheckServiceNode{
			Service: &structs.NodeService{
				Service:        service,
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
			},
		},
	}
}

func newPayloadCheckServiceNodeWithOverride(
	service, namespace, overrideKey, overrideNamespace string) EventPayloadCheckServiceNode {
	return EventPayloadCheckServiceNode{
		Value: &structs.CheckServiceNode{
			Service: &structs.NodeService{
				Service:        service,
				EnterpriseMeta: structs.NewEnterpriseMetaInDefaultPartition(namespace),
			},
		},
		overrideKey:       overrideKey,
		overrideNamespace: overrideNamespace,
	}
}

func TestServiceListUpdateSnapshot(t *testing.T) {
	const index uint64 = 123

	store := testStateStore(t)
	require.NoError(t, store.EnsureRegistration(index, testServiceRegistration(t, "db")))

	buf := &snapshotAppender{}
	idx, err := store.ServiceListSnapshot(stream.SubscribeRequest{Subject: stream.SubjectNone}, buf)
	require.NoError(t, err)
	require.NotZero(t, idx)

	require.Len(t, buf.events, 1)
	require.Len(t, buf.events[0], 1)

	payload := buf.events[0][0].Payload.(*EventPayloadServiceListUpdate)
	require.Equal(t, pbsubscribe.CatalogOp_Register, payload.Op)
	require.Equal(t, "db", payload.Name)
}

func TestServiceListUpdateEventsFromChanges(t *testing.T) {
	const changeIndex = 123

	testCases := map[string]struct {
		setup  func(*Store, *txn) error
		mutate func(*Store, *txn) error
		events []stream.Event
	}{
		"register new service": {
			mutate: func(store *Store, tx *txn) error {
				return store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db"), false)
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceList,
					Index: changeIndex,
					Payload: &EventPayloadServiceListUpdate{
						Op:             pbsubscribe.CatalogOp_Register,
						Name:           "db",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
				},
			},
		},
		"service already registered": {
			setup: func(store *Store, tx *txn) error {
				return store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db"), false)
			},
			mutate: func(store *Store, tx *txn) error {
				return store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db"), false)
			},
			events: nil,
		},
		"deregister last instance of service": {
			setup: func(store *Store, tx *txn) error {
				return store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db"), false)
			},
			mutate: func(store *Store, tx *txn) error {
				return store.deleteServiceTxn(tx, tx.Index, "node1", "db", nil, "")
			},
			events: []stream.Event{
				{
					Topic: EventTopicServiceList,
					Index: changeIndex,
					Payload: &EventPayloadServiceListUpdate{
						Op:             pbsubscribe.CatalogOp_Deregister,
						Name:           "db",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
				},
			},
		},
		"deregister (not the last) instance of service": {
			setup: func(store *Store, tx *txn) error {
				if err := store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db"), false); err != nil {
					return err
				}
				if err := store.ensureRegistrationTxn(tx, changeIndex, false, testServiceRegistration(t, "db", regNode2), false); err != nil {
					return err
				}
				return nil
			},
			mutate: func(store *Store, tx *txn) error {
				return store.deleteServiceTxn(tx, tx.Index, "node1", "db", nil, "")
			},
			events: nil,
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			store := testStateStore(t)

			if tc.setup != nil {
				tx := store.db.WriteTxn(0)
				require.NoError(t, tc.setup(store, tx))
				require.NoError(t, tx.Commit())
			}

			tx := store.db.WriteTxn(0)
			t.Cleanup(tx.Abort)

			if tc.mutate != nil {
				require.NoError(t, tc.mutate(store, tx))
			}

			events, err := ServiceListUpdateEventsFromChanges(tx, Changes{Index: changeIndex, Changes: tx.Changes()})
			require.NoError(t, err)
			require.Equal(t, tc.events, events)
		})
	}
}
