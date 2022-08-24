package peerstream

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

const (
	aPeerID = "63b60245-c475-426b-b314-4588d210859d"
)

func TestStatus_IsHealthy(t *testing.T) {
	type testcase struct {
		name             string
		dontConnect      bool
		modifierFunc     func(status *MutableStatus)
		expectedVal      bool
		heartbeatTimeout time.Duration
	}

	tcs := []testcase{
		{
			name:        "never connected, unhealthy",
			expectedVal: false,
			dontConnect: true,
		},
		{
			name:        "no heartbeat, unhealthy",
			expectedVal: false,
		},
		{
			name:        "heartbeat is not received, unhealthy",
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				// set heartbeat
				status.LastRecvHeartbeat = time.Now().Add(-1 * time.Second)
			},
			heartbeatTimeout: 1 * time.Second,
		},
		{
			name:        "send error before send success",
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				// set heartbeat
				status.LastRecvHeartbeat = time.Now()

				status.LastSendSuccess = time.Now()
				status.LastSendError = time.Now()
			},
		},
		{
			name:        "received error before received success",
			expectedVal: false,
			modifierFunc: func(status *MutableStatus) {
				// set heartbeat
				status.LastRecvHeartbeat = time.Now()

				status.LastRecvResourceSuccess = time.Now()
				status.LastRecvError = time.Now()
			},
		},
		{
			name:        "healthy",
			expectedVal: true,
			modifierFunc: func(status *MutableStatus) {
				// set heartbeat
				status.LastRecvHeartbeat = time.Now()
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tracker := NewTracker()
			if tc.heartbeatTimeout.Microseconds() != 0 {
				tracker.SetHeartbeatTimeout(tc.heartbeatTimeout)
			}

			if !tc.dontConnect {
				st, err := tracker.Connected(aPeerID)
				require.NoError(t, err)
				require.True(t, st.Connected)

				if tc.modifierFunc != nil {
					tc.modifierFunc(st)
				}

				require.Equal(t, tc.expectedVal, st.IsHealthy())

			} else {
				st, found := tracker.StreamStatus(aPeerID)
				require.False(t, found)
				require.Equal(t, tc.expectedVal, st.IsHealthy())
			}
		})
	}
}

func TestTracker_EnsureConnectedDisconnected(t *testing.T) {
	tracker := NewTracker()
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
			Connected:        true,
			heartbeatTimeout: defaultIncomingHeartbeatTimeout,
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
	var lastSuccess time.Time

	testutil.RunStep(t, "stream updated", func(t *testing.T) {
		statusPtr.TrackAck()
		sequence++

		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)

		lastSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()
		expect := Status{
			Connected:        true,
			LastAck:          lastSuccess,
			heartbeatTimeout: defaultIncomingHeartbeatTimeout,
		}
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "disconnect", func(t *testing.T) {
		tracker.DisconnectedGracefully(peerID)
		sequence++

		expect := Status{
			Connected:        false,
			DisconnectTime:   it.base.Add(time.Duration(sequence) * time.Second).UTC(),
			LastAck:          lastSuccess,
			heartbeatTimeout: defaultIncomingHeartbeatTimeout,
		}
		status, ok := tracker.StreamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "re-connect", func(t *testing.T) {
		_, err := tracker.Connected(peerID)
		require.NoError(t, err)

		expect := Status{
			Connected:        true,
			LastAck:          lastSuccess,
			heartbeatTimeout: defaultIncomingHeartbeatTimeout,

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
		tracker := NewTracker()
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
			DisconnectTime:         time.Now(),
			DisconnectErrorMessage: "disconnected",
		},
	}
	s.TrackConnected()

	require.True(t, s.IsConnected())
	require.True(t, s.Connected)
	require.Equal(t, time.Time{}, s.DisconnectTime)
	require.Empty(t, s.DisconnectErrorMessage)
}

func TestMutableStatus_TrackDisconnectedGracefully(t *testing.T) {
	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	disconnectTime := it.FutureNow(1)

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
	disconnectTime := it.FutureNow(1)

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
