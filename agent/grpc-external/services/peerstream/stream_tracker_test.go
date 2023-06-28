// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peerstream

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

const (
	aPeerID = "63b60245-c475-426b-b314-4588d210859d"
)

func TestTracker_IsHealthy(t *testing.T) {
	type testcase struct {
		name         string
		tracker      *Tracker
		modifierFunc func(status *MutableStatus)
		expectedVal  bool
	}

	tcs := []testcase{
		{
			name:        "disconnect time within timeout",
			tracker:     NewTracker(defaultIncomingHeartbeatTimeout),
			expectedVal: true,
			modifierFunc: func(status *MutableStatus) {
				status.DisconnectTime = ptr(time.Now())
			},
		},
		{
			name:        "disconnect time past timeout",
			tracker:     NewTracker(1 * time.Millisecond),
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				status.DisconnectTime = ptr(time.Now().Add(-1 * time.Minute))
			},
		},
		{
			name:        "receive error before receive success within timeout",
			tracker:     NewTracker(defaultIncomingHeartbeatTimeout),
			expectedVal: true,
			modifierFunc: func(status *MutableStatus) {
				now := time.Now()
				status.LastRecvResourceSuccess = &now
				status.LastRecvError = ptr(now.Add(1 * time.Second))
			},
		},
		{
			name:        "receive error before receive success within timeout",
			tracker:     NewTracker(defaultIncomingHeartbeatTimeout),
			expectedVal: true,
			modifierFunc: func(status *MutableStatus) {
				now := time.Now()
				status.LastRecvResourceSuccess = &now
				status.LastRecvError = ptr(now.Add(1 * time.Second))
			},
		},
		{
			name:        "receive error before receive success past timeout",
			tracker:     NewTracker(1 * time.Millisecond),
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				now := time.Now().Add(-2 * time.Second)
				status.LastRecvResourceSuccess = &now
				status.LastRecvError = ptr(now.Add(1 * time.Second))
			},
		},
		{
			name:        "nack before ack within timeout",
			tracker:     NewTracker(defaultIncomingHeartbeatTimeout),
			expectedVal: true,
			modifierFunc: func(status *MutableStatus) {
				now := time.Now()
				status.LastAck = &now
				status.LastNack = ptr(now.Add(1 * time.Second))
			},
		},
		{
			name:        "nack before ack past timeout",
			tracker:     NewTracker(1 * time.Millisecond),
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				now := time.Now().Add(-2 * time.Second)
				status.LastAck = &now
				status.LastNack = ptr(now.Add(1 * time.Second))
			},
		},
		{
			name:        "healthy",
			tracker:     NewTracker(defaultIncomingHeartbeatTimeout),
			expectedVal: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tracker := tc.tracker

			st, err := tracker.Connected(aPeerID)
			require.NoError(t, err)
			require.True(t, st.Connected)

			if tc.modifierFunc != nil {
				tc.modifierFunc(st)
			}

			assert.Equal(t, tc.expectedVal, tracker.IsHealthy(st.GetStatus()))
		})
	}
}

func TestTracker_EnsureConnectedDisconnected(t *testing.T) {
	tracker := NewTracker(defaultIncomingHeartbeatTimeout)
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	tracker.timeNow = it.Now

	var (
		statusPtr *MutableStatus
		err       error
	)

	testutil.RunStep(t, "new stream", func(t *testing.T) {
		statusPtr, err = tracker.Connected(peerID)
		require.NoError(t, err)

		expect := Status{
			Connected: true,
		}

		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "duplicate gets rejected", func(t *testing.T) {
		_, err := tracker.Connected(peerID)
		require.Error(t, err)
		require.Contains(t, err.Error(), `there is an active stream for the given PeerID "63b60245-c475-426b-b314-4588d210859d"`)
	})

	var sequence uint64
	var lastSuccess *time.Time

	testutil.RunStep(t, "stream updated", func(t *testing.T) {
		statusPtr.TrackAck()
		sequence++

		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)

		lastSuccess = ptr(it.base.Add(time.Duration(sequence) * time.Second).UTC())
		expect := Status{
			Connected: true,
			LastAck:   lastSuccess,
		}
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "disconnect", func(t *testing.T) {
		tracker.DisconnectedGracefully(peerID)
		sequence++

		expect := Status{
			Connected:      false,
			DisconnectTime: ptr(it.base.Add(time.Duration(sequence) * time.Second).UTC()),
			LastAck:        lastSuccess,
		}
		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "re-connect", func(t *testing.T) {
		_, err := tracker.Connected(peerID)
		require.NoError(t, err)

		expect := Status{
			Connected:      true,
			LastAck:        lastSuccess,
			DisconnectTime: &time.Time{},
			// DisconnectTime gets cleared on re-connect.
		}

		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "delete", func(t *testing.T) {
		tracker.DeleteStatus(peerID)

		status, ok := tracker.StreamStatus(peerID)
		require.False(t, ok)
		require.Equal(t, Status{NeverConnected: true}, status)
	})
}

func TestTracker_connectedStreams(t *testing.T) {
	type testCase struct {
		name   string
		setup  func(t *testing.T, s *Tracker)
		expect []string
	}

	run := func(t *testing.T, tc testCase) {
		tracker := NewTracker(defaultIncomingHeartbeatTimeout)
		if tc.setup != nil {
			tc.setup(t, tracker)
		}

		streams := tracker.ConnectedStreams()

		var keys []string
		for key := range streams {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		require.Equal(t, tc.expect, keys)
	}

	tt := []testCase{
		{
			name:   "no streams",
			expect: nil,
		},
		{
			name: "all streams active",
			setup: func(t *testing.T, s *Tracker) {
				_, err := s.Connected("foo")
				require.NoError(t, err)

				_, err = s.Connected("bar")
				require.NoError(t, err)
			},
			expect: []string{"bar", "foo"},
		},
		{
			name: "mixed active and inactive",
			setup: func(t *testing.T, s *Tracker) {
				status, err := s.Connected("foo")
				require.NoError(t, err)

				// Mark foo as disconnected to avoid showing it as an active stream
				status.TrackDisconnectedGracefully()

				_, err = s.Connected("bar")
				require.NoError(t, err)
			},
			expect: []string{"bar"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestMutableStatus_TrackConnected(t *testing.T) {
	s := MutableStatus{
		Status: Status{
			Connected:              false,
			DisconnectTime:         ptr(time.Now()),
			DisconnectErrorMessage: "disconnected",
		},
	}
	s.TrackConnected()

	require.True(t, s.IsConnected())
	require.True(t, s.Connected)
	require.Equal(t, &time.Time{}, s.DisconnectTime)
	require.Empty(t, s.DisconnectErrorMessage)
}

func TestMutableStatus_TrackDisconnectedGracefully(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	disconnectTime := ptr(it.FutureNow(1))

	s := MutableStatus{
		timeNow: it.Now,
		Status: Status{
			Connected: true,
		},
	}

	s.TrackDisconnectedGracefully()

	require.False(t, s.IsConnected())
	require.False(t, s.Connected)
	require.Equal(t, disconnectTime, s.DisconnectTime)
	require.Empty(t, s.DisconnectErrorMessage)
}

func TestMutableStatus_TrackDisconnectedDueToError(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	disconnectTime := ptr(it.FutureNow(1))

	s := MutableStatus{
		timeNow: it.Now,
		Status: Status{
			Connected: true,
		},
	}

	s.TrackDisconnectedDueToError("disconnect err")

	require.False(t, s.IsConnected())
	require.False(t, s.Connected)
	require.Equal(t, disconnectTime, s.DisconnectTime)
	require.Equal(t, "disconnect err", s.DisconnectErrorMessage)
}
