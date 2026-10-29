// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

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

			st, _, err := tracker.Connected(aPeerID)
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
		gen       uint64
		err       error
	)

	testutil.RunStep(t, "new stream", func(t *testing.T) {
		statusPtr, gen, err = tracker.Connected(peerID)
		require.NoError(t, err)

		expect := Status{
			Connected: true,
		}

		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "duplicate gets rejected", func(t *testing.T) {
		_, _, err := tracker.Connected(peerID)
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
		tracker.DisconnectedGracefully(peerID, gen)
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
		_, _, err := tracker.Connected(peerID)
		require.NoError(t, err)

		expect := Status{
			Connected:      true,
			LastAck:        lastSuccess,
			DisconnectTime: nil,
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
				_, _, err := s.Connected("foo")
				require.NoError(t, err)

				_, _, err = s.Connected("bar")
				require.NoError(t, err)
			},
			expect: []string{"bar", "foo"},
		},
		{
			name: "mixed active and inactive",
			setup: func(t *testing.T, s *Tracker) {
				status, _, err := s.Connected("foo")
				require.NoError(t, err)

				// Mark foo as disconnected to avoid showing it as an active stream
				status.TrackDisconnectedGracefully()

				_, _, err = s.Connected("bar")
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
	require.Nil(t, s.DisconnectTime)
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

func TestMutableStatus_TrackDisconnectedIfCurrent_StaleGen(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	s := MutableStatus{
		timeNow: it.Now,
		Status: Status{
			Connected: true,
		},
		streamGeneration: 5,
	}

	// Stale generation should be a no-op
	applied := s.TrackDisconnectedGracefullyIfCurrent(3)
	require.False(t, applied, "stale generation should not apply disconnect")
	require.True(t, s.IsConnected(), "stream should still be connected after stale disconnect")

	applied = s.TrackDisconnectedDueToErrorIfCurrent(3, "stale error")
	require.False(t, applied, "stale generation should not apply error disconnect")
	require.True(t, s.IsConnected(), "stream should still be connected after stale error disconnect")
	require.Empty(t, s.DisconnectErrorMessage)
}

func TestMutableStatus_TrackDisconnectedIfCurrent_MatchingGen(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	t.Run("graceful", func(t *testing.T) {
		s := MutableStatus{
			timeNow: it.Now,
			Status: Status{
				Connected: true,
			},
			streamGeneration: 5,
		}

		applied := s.TrackDisconnectedGracefullyIfCurrent(5)
		require.True(t, applied, "matching generation should apply disconnect")
		require.False(t, s.IsConnected(), "stream should be disconnected")
		require.NotNil(t, s.DisconnectTime)
		require.Empty(t, s.DisconnectErrorMessage)
	})

	t.Run("due to error", func(t *testing.T) {
		s := MutableStatus{
			timeNow: it.Now,
			Status: Status{
				Connected: true,
			},
			streamGeneration: 5,
		}

		applied := s.TrackDisconnectedDueToErrorIfCurrent(5, "some error")
		require.True(t, applied, "matching generation should apply disconnect")
		require.False(t, s.IsConnected(), "stream should be disconnected")
		require.NotNil(t, s.DisconnectTime)
		require.Equal(t, "some error", s.DisconnectErrorMessage)
	})
}

func TestTracker_DisconnectedWithGeneration(t *testing.T) {
	tracker := NewTracker(defaultIncomingHeartbeatTimeout)
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	// Connect and get generation
	_, gen1, err := tracker.Connected(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), gen1)

	// Disconnect with matching generation should work
	tracker.DisconnectedGracefully(peerID, gen1)
	status, ok := tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.False(t, status.Connected, "should be disconnected")

	// Reconnect - gen should increment
	_, gen2, err := tracker.Connected(peerID)
	require.NoError(t, err)
	require.Equal(t, uint64(2), gen2)

	// Disconnect with stale gen1 should be a no-op
	tracker.DisconnectedDueToError(peerID, gen1, "stale error")
	status, ok = tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.True(t, status.Connected, "stale disconnect should not affect current stream")
	require.Nil(t, status.DisconnectTime)

	// Disconnect with current gen2 should work
	tracker.DisconnectedDueToError(peerID, gen2, "real error")
	status, ok = tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.False(t, status.Connected, "current gen disconnect should work")
	require.Equal(t, "real error", status.DisconnectErrorMessage)
}

func TestTracker_IsHealthy_RecentACKOverridesDisconnected(t *testing.T) {
	tracker := NewTracker(defaultIncomingHeartbeatTimeout)

	now := time.Now()
	tracker.setClock(func() time.Time { return now })

	// Create a status that appears disconnected but has a recent ACK
	s := Status{
		Connected:      false,
		DisconnectTime: ptr(now.Add(-1 * time.Hour)),   // disconnected a long time ago
		LastAck:        ptr(now.Add(-1 * time.Second)), // but ACK was just 1 second ago
	}

	require.True(t, tracker.IsHealthy(s),
		"recent ACK should override disconnected state")

	// Verify that old ACK does NOT override
	sOld := Status{
		Connected:      false,
		DisconnectTime: ptr(now.Add(-1 * time.Hour)),
		LastAck:        ptr(now.Add(-1 * time.Hour)), // ACK is also old
	}

	require.False(t, tracker.IsHealthy(sOld),
		"old ACK should not override disconnected state")
}
