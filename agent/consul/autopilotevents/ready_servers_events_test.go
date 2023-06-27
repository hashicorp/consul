// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package autopilotevents

import (
	"testing"
	time "time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
	types "github.com/hashicorp/consul/types"
)

var testTime = time.Date(2022, 4, 14, 10, 56, 00, 0, time.UTC)

var exampleState = &autopilot.State{
	Servers: map[raft.ServerID]*autopilot.ServerState{
		"792ae13c-d765-470b-852c-e073fdb6e849": {
			Health: autopilot.ServerHealth{
				Healthy: true,
			},
			State: autopilot.RaftLeader,
			Server: autopilot.Server{
				ID:         "792ae13c-d765-470b-852c-e073fdb6e849",
				Address:    "198.18.0.2:8300",
				Version:    "v1.12.0",
				NodeStatus: autopilot.NodeAlive,
			},
		},
		"65e79ff4-bbce-467b-a9d6-725c709fa985": {
			Health: autopilot.ServerHealth{
				Healthy: true,
			},
			State: autopilot.RaftVoter,
			Server: autopilot.Server{
				ID:         "65e79ff4-bbce-467b-a9d6-725c709fa985",
				Address:    "198.18.0.3:8300",
				Version:    "v1.12.0",
				NodeStatus: autopilot.NodeAlive,
			},
		},
		// this server is up according to Serf but is unhealthy
		// due to having an index that is behind
		"db11f0ac-0cbe-4215-80cc-b4e843f4df1e": {
			Health: autopilot.ServerHealth{
				Healthy: false,
			},
			State: autopilot.RaftVoter,
			Server: autopilot.Server{
				ID:         "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
				Address:    "198.18.0.4:8300",
				Version:    "v1.12.0",
				NodeStatus: autopilot.NodeAlive,
			},
		},
		// this server is up according to Serf but is unhealthy
		// due to having an index that is behind. It is a non-voter
		// and thus will be filtered out
		"4c48a154-8176-4e14-ba5d-20bf1f784a7e": {
			Health: autopilot.ServerHealth{
				Healthy: false,
			},
			State: autopilot.RaftNonVoter,
			Server: autopilot.Server{
				ID:         "4c48a154-8176-4e14-ba5d-20bf1f784a7e",
				Address:    "198.18.0.5:8300",
				Version:    "v1.12.0",
				NodeStatus: autopilot.NodeAlive,
			},
		},
		// this is a voter that has died
		"7a22eec8-de85-43a6-a76e-00b427ef6627": {
			Health: autopilot.ServerHealth{
				Healthy: false,
			},
			State: autopilot.RaftVoter,
			Server: autopilot.Server{
				ID:         "7a22eec8-de85-43a6-a76e-00b427ef6627",
				Address:    "198.18.0.6",
				Version:    "v1.12.0",
				NodeStatus: autopilot.NodeFailed,
			},
		},
	},
}

func TestEventPayloadReadyServers_HasReadPermission(t *testing.T) {
	t.Run("no service:write", func(t *testing.T) {
		hasRead := EventPayloadReadyServers{}.HasReadPermission(acl.DenyAll())
		require.False(t, hasRead)
	})

	t.Run("has service:write", func(t *testing.T) {
		policy, err := acl.NewPolicyFromSource(`
 			service "foo" {
 				policy = "write"
 			}
 		`, nil, nil)
		require.NoError(t, err)

		authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)

		hasRead := EventPayloadReadyServers{}.HasReadPermission(authz)
		require.True(t, hasRead)
	})
}

func TestAutopilotStateToReadyServers(t *testing.T) {
	expected := EventPayloadReadyServers{
		{
			ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
			Address: "198.18.0.2",
			Version: "v1.12.0",
		},
		{
			ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
			Address: "198.18.0.3",
			Version: "v1.12.0",
		},
		{
			ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
			Address: "198.18.0.4",
			Version: "v1.12.0",
		},
	}

	r := ReadyServersEventPublisher{}

	actual := r.autopilotStateToReadyServers(exampleState)
	require.ElementsMatch(t, expected, actual)
}

func TestAutopilotStateToReadyServersWithTaggedAddresses(t *testing.T) {
	expected := EventPayloadReadyServers{
		{
			ID:              "792ae13c-d765-470b-852c-e073fdb6e849",
			Address:         "198.18.0.2",
			TaggedAddresses: map[string]string{"wan": "5.4.3.2"},
			Version:         "v1.12.0",
		},
		{
			ID:              "65e79ff4-bbce-467b-a9d6-725c709fa985",
			Address:         "198.18.0.3",
			TaggedAddresses: map[string]string{"wan": "1.2.3.4"},
			Version:         "v1.12.0",
		},
		{
			ID:              "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
			Address:         "198.18.0.4",
			TaggedAddresses: map[string]string{"wan": "9.8.7.6"},
			Version:         "v1.12.0",
		},
	}

	store := &MockStateStore{}
	t.Cleanup(func() { store.AssertExpectations(t) })
	store.On("GetNodeID",
		types.NodeID("792ae13c-d765-470b-852c-e073fdb6e849"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-1", TaggedAddresses: map[string]string{"wan": "5.4.3.2"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-1",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("65e79ff4-bbce-467b-a9d6-725c709fa985"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-2", TaggedAddresses: map[string]string{"wan": "1.2.3.4"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-2",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("db11f0ac-0cbe-4215-80cc-b4e843f4df1e"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-3", TaggedAddresses: map[string]string{"wan": "9.8.7.6"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-3",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	r := NewReadyServersEventPublisher(Config{
		GetStore: func() StateStore { return store },
	})

	actual := r.autopilotStateToReadyServers(exampleState)
	require.ElementsMatch(t, expected, actual)
}

func TestAutopilotStateToReadyServersWithExtGRPCPort(t *testing.T) {
	expected := EventPayloadReadyServers{
		{
			ID:          "792ae13c-d765-470b-852c-e073fdb6e849",
			Address:     "198.18.0.2",
			ExtGRPCPort: 1234,
			Version:     "v1.12.0",
		},
		{
			ID:          "65e79ff4-bbce-467b-a9d6-725c709fa985",
			Address:     "198.18.0.3",
			ExtGRPCPort: 2345,
			Version:     "v1.12.0",
		},
		{
			ID:          "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
			Address:     "198.18.0.4",
			ExtGRPCPort: 3456,
			Version:     "v1.12.0",
		},
	}

	store := &MockStateStore{}
	t.Cleanup(func() { store.AssertExpectations(t) })
	store.On("GetNodeID",
		types.NodeID("792ae13c-d765-470b-852c-e073fdb6e849"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-1"},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-1",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		&structs.NodeService{Meta: map[string]string{"grpc_port": "1234"}},
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("65e79ff4-bbce-467b-a9d6-725c709fa985"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-2"},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-2",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		&structs.NodeService{Meta: map[string]string{"grpc_port": "2345"}},
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("db11f0ac-0cbe-4215-80cc-b4e843f4df1e"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-3"},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-3",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		&structs.NodeService{Meta: map[string]string{"grpc_port": "3456"}},
		nil,
	)

	r := NewReadyServersEventPublisher(Config{
		GetStore: func() StateStore { return store },
	})

	actual := r.autopilotStateToReadyServers(exampleState)
	require.ElementsMatch(t, expected, actual)
}

func TestAutopilotReadyServersEvents(t *testing.T) {
	// we have already tested the ReadyServerInfo extraction within the
	// TestAutopilotStateToReadyServers test. Therefore this test is going
	// to focus only on the change detection.
	//
	// * - added server
	// * - removed server
	// * - server with address changed
	// * - upgraded server with version change

	expectedServers := EventPayloadReadyServers{
		{
			ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
			Address: "198.18.0.3",
			Version: "v1.12.0",
		},
		{
			ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
			Address: "198.18.0.2",
			Version: "v1.12.0",
		},
		{
			ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
			Address: "198.18.0.4",
			Version: "v1.12.0",
		},
	}

	type testCase struct {
		// The elements of this slice must already be sorted
		previous       EventPayloadReadyServers
		changeDetected bool
	}

	cases := map[string]testCase{
		"no-change": {
			previous: EventPayloadReadyServers{
				{
					ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
					Address: "198.18.0.3",
					Version: "v1.12.0",
				},
				{
					ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
					Address: "198.18.0.2",
					Version: "v1.12.0",
				},
				{
					ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
					Address: "198.18.0.4",
					Version: "v1.12.0",
				},
			},
			changeDetected: false,
		},
		"server-added": {
			previous: EventPayloadReadyServers{
				{
					ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
					Address: "198.18.0.3",
					Version: "v1.12.0",
				},
				{
					ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
					Address: "198.18.0.2",
					Version: "v1.12.0",
				},
				// server with id db11f0ac-0cbe-4215-80cc-b4e843f4df1e will be added.
			},
			changeDetected: true,
		},
		"server-removed": {
			previous: EventPayloadReadyServers{
				{
					ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
					Address: "198.18.0.3",
					Version: "v1.12.0",
				},
				{
					ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
					Address: "198.18.0.2",
					Version: "v1.12.0",
				},
				// this server isn't present in the state and will be removed
				{
					ID:      "7e3235de-8a75-4c8d-9ec3-847ca87d07e8",
					Address: "198.18.0.5",
					Version: "v1.12.0",
				},
				{
					ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
					Address: "198.18.0.4",
					Version: "v1.12.0",
				},
			},
			changeDetected: true,
		},
		"address-change": {
			previous: EventPayloadReadyServers{
				{
					ID: "65e79ff4-bbce-467b-a9d6-725c709fa985",
					// this value is different from the state and should
					// cause an event to be generated
					Address: "198.18.0.9",
					Version: "v1.12.0",
				},
				{
					ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
					Address: "198.18.0.2",
					Version: "v1.12.0",
				},
				{
					ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
					Address: "198.18.0.4",
					Version: "v1.12.0",
				},
			},
			changeDetected: true,
		},
		"upgraded-version": {
			previous: EventPayloadReadyServers{
				{
					ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
					Address: "198.18.0.3",
					// This is v1.12.0 in the state and therefore an
					// event should be generated
					Version: "v1.11.4",
				},
				{
					ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
					Address: "198.18.0.2",
					Version: "v1.12.0",
				},
				{
					ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
					Address: "198.18.0.4",
					Version: "v1.12.0",
				},
			},
			changeDetected: true,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			r := ReadyServersEventPublisher{
				previous: tcase.previous,
			}
			events, changeDetected := r.readyServersEvents(exampleState)
			require.Equal(t, tcase.changeDetected, changeDetected, "servers: %+v", events)
			if tcase.changeDetected {
				require.Len(t, events, 1)
				require.Equal(t, EventTopicReadyServers, events[0].Topic)
				payload, ok := events[0].Payload.(EventPayloadReadyServers)
				require.True(t, ok)
				require.ElementsMatch(t, expectedServers, payload)
			} else {
				require.Empty(t, events)
			}
		})
	}
}

func TestAutopilotPublishReadyServersEvents(t *testing.T) {
	t.Run("publish", func(t *testing.T) {
		pub := &MockPublisher{}
		pub.On("Publish", []stream.Event{
			{
				Topic: EventTopicReadyServers,
				Index: uint64(testTime.UnixMicro()),
				Payload: EventPayloadReadyServers{
					{
						ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
						Address: "198.18.0.3",
						Version: "v1.12.0",
					},
					{
						ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
						Address: "198.18.0.2",
						Version: "v1.12.0",
					},
					{
						ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
						Address: "198.18.0.4",
						Version: "v1.12.0",
					},
				},
			},
		})

		mtime := &mockTimeProvider{}
		mtime.On("Now").Return(testTime).Once()

		t.Cleanup(func() {
			mtime.AssertExpectations(t)
			pub.AssertExpectations(t)
		})

		r := NewReadyServersEventPublisher(Config{
			Publisher:    pub,
			timeProvider: mtime,
		})

		r.PublishReadyServersEvents(exampleState)
	})

	t.Run("suppress", func(t *testing.T) {
		pub := &MockPublisher{}
		mtime := &mockTimeProvider{}

		t.Cleanup(func() {
			mtime.AssertExpectations(t)
			pub.AssertExpectations(t)
		})

		r := NewReadyServersEventPublisher(Config{
			Publisher:    pub,
			timeProvider: mtime,
		})

		r.previous = EventPayloadReadyServers{
			{
				ID:      "65e79ff4-bbce-467b-a9d6-725c709fa985",
				Address: "198.18.0.3",
				Version: "v1.12.0",
			},
			{
				ID:      "792ae13c-d765-470b-852c-e073fdb6e849",
				Address: "198.18.0.2",
				Version: "v1.12.0",
			},
			{
				ID:      "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
				Address: "198.18.0.4",
				Version: "v1.12.0",
			},
		}

		r.PublishReadyServersEvents(exampleState)
	})
}

type MockAppender struct {
	mock.Mock
}

func (m *MockAppender) Append(events []stream.Event) {
	m.Called(events)
}

func TestReadyServerEventsSnapshotHandler(t *testing.T) {
	buf := MockAppender{}
	buf.On("Append", []stream.Event{
		{
			Topic:   EventTopicReadyServers,
			Index:   0,
			Payload: EventPayloadReadyServers{},
		},
	})
	buf.On("Append", []stream.Event{
		{
			Topic: EventTopicReadyServers,
			Index: 1649933760000000,
			Payload: EventPayloadReadyServers{
				{
					ID:              "65e79ff4-bbce-467b-a9d6-725c709fa985",
					Address:         "198.18.0.3",
					TaggedAddresses: map[string]string{"wan": "1.2.3.4"},
					Version:         "v1.12.0",
				},
				{
					ID:              "792ae13c-d765-470b-852c-e073fdb6e849",
					Address:         "198.18.0.2",
					TaggedAddresses: map[string]string{"wan": "5.4.3.2"},
					Version:         "v1.12.0",
				},
				{
					ID:              "db11f0ac-0cbe-4215-80cc-b4e843f4df1e",
					Address:         "198.18.0.4",
					TaggedAddresses: map[string]string{"wan": "9.8.7.6"},
					Version:         "v1.12.0",
				},
			},
		},
	}).Once()

	mtime := mockTimeProvider{}
	mtime.On("Now").Return(testTime).Once()

	store := &MockStateStore{}
	t.Cleanup(func() { store.AssertExpectations(t) })
	store.On("GetNodeID",
		types.NodeID("792ae13c-d765-470b-852c-e073fdb6e849"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-1", TaggedAddresses: map[string]string{"wan": "5.4.3.2"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-1",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("65e79ff4-bbce-467b-a9d6-725c709fa985"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-2", TaggedAddresses: map[string]string{"wan": "1.2.3.4"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-2",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	store.On("GetNodeID",
		types.NodeID("db11f0ac-0cbe-4215-80cc-b4e843f4df1e"),
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Times(2).Return(
		uint64(0),
		&structs.Node{Node: "node-3", TaggedAddresses: map[string]string{"wan": "9.8.7.6"}},
		nil,
	)

	store.On("NodeService",
		memdb.WatchSet(nil),
		"node-3",
		structs.ConsulServiceID,
		structs.NodeEnterpriseMetaInDefaultPartition(),
		structs.DefaultPeerKeyword,
	).Once().Return(
		uint64(0),
		nil,
		nil,
	)

	t.Cleanup(func() {
		buf.AssertExpectations(t)
		store.AssertExpectations(t)
		mtime.AssertExpectations(t)
	})

	r := NewReadyServersEventPublisher(Config{
		GetStore:     func() StateStore { return store },
		timeProvider: &mtime,
	})

	req := stream.SubscribeRequest{
		Topic:   EventTopicReadyServers,
		Subject: stream.SubjectNone,
	}

	// get the first snapshot that should have the zero value event
	_, err := r.HandleSnapshot(req, &buf)
	require.NoError(t, err)

	// setup the value to be returned by the snapshot handler
	r.snapshot, _ = r.readyServersEvents(exampleState)

	// now get the second snapshot which has actual servers
	_, err = r.HandleSnapshot(req, &buf)
	require.NoError(t, err)
}

type fakePayload struct{}

func (e fakePayload) Subject() stream.Subject { return stream.SubjectNone }

func (e fakePayload) HasReadPermission(authz acl.Authorizer) bool {
	return false
}

func (e fakePayload) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	panic("fakePayload does not implement ToSubscriptionEvent")
}

func TestExtractEventPayload(t *testing.T) {
	t.Run("wrong-topic", func(t *testing.T) {
		payload, err := ExtractEventPayload(stream.NewCloseSubscriptionEvent([]string{"foo"}))
		require.Nil(t, payload)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected topic")
	})

	t.Run("unexpected-payload", func(t *testing.T) {
		payload, err := ExtractEventPayload(stream.Event{
			Topic:   EventTopicReadyServers,
			Payload: fakePayload{},
		})
		require.Nil(t, payload)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected payload type")
	})

	t.Run("success", func(t *testing.T) {
		expected := EventPayloadReadyServers{
			{
				ID:      "a7c340ae-ce17-47da-895c-af2509767b3d",
				Address: "198.18.0.1",
				Version: "1.2.3",
			},
		}
		actual, err := ExtractEventPayload(stream.Event{
			Topic:   EventTopicReadyServers,
			Payload: expected,
		})

		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestReadyServerInfo_Equal(t *testing.T) {
	base := func() *ReadyServerInfo {
		return &ReadyServerInfo{
			ID:      "0356e5ae-ed6b-4024-b953-e1b6a8f0f81b",
			Version: "1.12.0",
			Address: "198.18.0.1",
			TaggedAddresses: map[string]string{
				"wan": "1.2.3.4",
			},
		}
	}
	type testCase struct {
		modify func(i *ReadyServerInfo)
		equal  bool
	}

	cases := map[string]testCase{
		"unmodified": {
			equal: true,
		},
		"id-mod": {
			modify: func(i *ReadyServerInfo) { i.ID = "30f8f451-e54b-4c7e-a533-b55dddb51be6" },
		},
		"version-mod": {
			modify: func(i *ReadyServerInfo) { i.Version = "1.12.1" },
		},
		"address-mod": {
			modify: func(i *ReadyServerInfo) { i.Address = "198.18.0.2" },
		},
		"tagged-addresses-added": {
			modify: func(i *ReadyServerInfo) { i.TaggedAddresses["wan_ipv4"] = "1.2.3.4" },
		},
		"tagged-addresses-mod": {
			modify: func(i *ReadyServerInfo) { i.TaggedAddresses["wan"] = "4.3.2.1" },
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			original := base()
			modified := base()
			if tcase.modify != nil {
				tcase.modify(modified)
			}

			require.Equal(t, tcase.equal, original.Equal(modified))

		})
	}
}
