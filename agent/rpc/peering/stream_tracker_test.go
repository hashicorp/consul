package peering

import (
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestStreamTracker_EnsureConnectedDisconnected(t *testing.T) {
	tracker := newStreamTracker()
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	it := incrementalTime{
		base: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	tracker.timeNow = it.Now

	var (
		statusPtr *lockableStreamStatus
		err       error
	)

	testutil.RunStep(t, "new stream", func(t *testing.T) {
		statusPtr, err = tracker.connected(peerID)
		require.NoError(t, err)

		expect := StreamStatus{
			Connected: true,
		}

		status, ok := tracker.streamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "duplicate gets rejected", func(t *testing.T) {
		_, err := tracker.connected(peerID)
		require.Error(t, err)
		require.Contains(t, err.Error(), `there is an active stream for the given PeerID "63b60245-c475-426b-b314-4588d210859d"`)
	})

	var sequence uint64
	var lastSuccess time.Time

	testutil.RunStep(t, "stream updated", func(t *testing.T) {
		statusPtr.trackAck()
		sequence++

		status, ok := tracker.streamStatus(peerID)
		require.True(t, ok)

		lastSuccess = it.base.Add(time.Duration(sequence) * time.Second).UTC()
		expect := StreamStatus{
			Connected: true,
			LastAck:   lastSuccess,
		}
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "disconnect", func(t *testing.T) {
		tracker.disconnected(peerID)
		sequence++

		expect := StreamStatus{
			Connected:      false,
			DisconnectTime: it.base.Add(time.Duration(sequence) * time.Second).UTC(),
			LastAck:        lastSuccess,
		}
		status, ok := tracker.streamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "re-connect", func(t *testing.T) {
		_, err := tracker.connected(peerID)
		require.NoError(t, err)

		expect := StreamStatus{
			Connected: true,
			LastAck:   lastSuccess,

			// DisconnectTime gets cleared on re-connect.
		}

		status, ok := tracker.streamStatus(peerID)
		require.True(t, ok)
		require.Equal(t, expect, status)
	})

	testutil.RunStep(t, "delete", func(t *testing.T) {
		tracker.deleteStatus(peerID)

		status, ok := tracker.streamStatus(peerID)
		require.False(t, ok)
		require.Zero(t, status)
	})
}

func TestStreamTracker_connectedStreams(t *testing.T) {
	type testCase struct {
		name   string
		setup  func(t *testing.T, s *streamTracker)
		expect []string
	}

	run := func(t *testing.T, tc testCase) {
		tracker := newStreamTracker()
		if tc.setup != nil {
			tc.setup(t, tracker)
		}

		streams := tracker.connectedStreams()

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
			setup: func(t *testing.T, s *streamTracker) {
				_, err := s.connected("foo")
				require.NoError(t, err)

				_, err = s.connected("bar")
				require.NoError(t, err)
			},
			expect: []string{"bar", "foo"},
		},
		{
			name: "mixed active and inactive",
			setup: func(t *testing.T, s *streamTracker) {
				status, err := s.connected("foo")
				require.NoError(t, err)

				// Mark foo as disconnected to avoid showing it as an active stream
				status.trackDisconnected()

				_, err = s.connected("bar")
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
