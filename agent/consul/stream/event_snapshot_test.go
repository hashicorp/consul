// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package stream

import (
	"context"
	fmt "fmt"
	"testing"
	time "time"

	"github.com/stretchr/testify/require"
)

func TestEventSnapshot(t *testing.T) {
	// Setup a dummy state that we can manipulate easily. The properties we care
	// about are that we publish some sequence of events as a snapshot and then
	// follow them up with "live updates". We control the interleaving. Our state
	// consists of health events (only type fully defined so far) for service
	// instances with consecutive ID numbers starting from 0 (e.g. test-000,
	// test-001). The snapshot is delivered at index 1000. updatesBeforeSnap
	// controls how many updates are delivered _before_ the snapshot is complete
	// (with an index < 1000). updatesBeforeSnap controls the number of updates
	// delivered after (index > 1000).
	//
	// In all cases the invariant should be that we end up with all of the
	// instances in the snapshot, plus any delivered _after_ the snapshot index,
	// but none delivered _before_ the snapshot index otherwise we may have an
	// inconsistent snapshot.
	cases := []struct {
		name              string
		snapshotSize      int
		updatesBeforeSnap int
		updatesAfterSnap  int
	}{
		{
			name:              "snapshot with subsequent mutations",
			snapshotSize:      10,
			updatesBeforeSnap: 0,
			updatesAfterSnap:  10,
		},
		{
			name:              "snapshot with concurrent mutations",
			snapshotSize:      10,
			updatesBeforeSnap: 5,
			updatesAfterSnap:  5,
		},
		{
			name:              "empty snapshot with subsequent mutations",
			snapshotSize:      0,
			updatesBeforeSnap: 0,
			updatesAfterSnap:  10,
		},
		{
			name:              "empty snapshot with concurrent mutations",
			snapshotSize:      0,
			updatesBeforeSnap: 5,
			updatesAfterSnap:  5,
		},
	}

	snapIndex := uint64(1000)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.updatesBeforeSnap < 999,
				"bad test param updatesBeforeSnap must be less than the snapshot"+
					" index (%d) minus one (%d), got: %d", snapIndex, snapIndex-1,
				tc.updatesBeforeSnap)

			// Create a snapshot func that will deliver registration events.
			snFn := testHealthConsecutiveSnapshotFn(tc.snapshotSize, snapIndex)

			// Create a topic buffer for updates
			tb := newEventBuffer()

			// Capture the topic buffer head now so updatesBeforeSnap are "concurrent"
			// and are seen by the eventSnapshot once it completes the snap.
			tbHead := tb.Head()

			// Deliver any pre-snapshot events simulating updates that occur after the
			// topic buffer is captured during a Subscribe call, but before the
			// snapshot is made of the FSM.
			for i := tc.updatesBeforeSnap; i > 0; i-- {
				index := snapIndex - uint64(i)
				// Use an instance index that's unique and should never appear in the
				// output so we can be sure these were not included as they came before
				// the snapshot.
				tb.Append([]Event{newDefaultHealthEvent(index, 10000+i)})
			}

			es := newEventSnapshot()
			es.appendAndSplice(SubscribeRequest{}, snFn, tbHead)

			// Deliver any post-snapshot events simulating updates that occur
			// logically after snapshot. It doesn't matter that these might actually
			// be appended before the snapshot fn executes in another goroutine since
			// it's operating an a possible stale "snapshot". This is the same as
			// reality with the state store where updates that occur after the
			// snapshot is taken but while the SnapFn is still running must be captured
			// correctly.
			for i := 0; i < tc.updatesAfterSnap; i++ {
				index := snapIndex + 1 + uint64(i)
				// Use an instance index that's unique.
				tb.Append([]Event{newDefaultHealthEvent(index, 20000+i)})
			}

			// Now read the snapshot buffer until we've received everything we expect.
			// Don't wait too long in case we get stuck.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			snapIDs := make([]string, 0, tc.snapshotSize)
			updateIDs := make([]string, 0, tc.updatesAfterSnap)
			snapDone := false
			curItem := es.First
			var err error
		RECV:
			for {
				curItem, err = curItem.Next(ctx, nil)
				// This error is typically timeout so dump the state to aid debugging.
				require.NoError(t, err,
					"current state: snapDone=%v snapIDs=%s updateIDs=%s", snapDone,
					snapIDs, updateIDs)
				if len(curItem.Events) == 0 {
					// An item without an error or events is a bufferItem.NextLink event.
					// A subscription handles this by proceeding to the next item,
					// so we do the same here.
					continue
				}
				e := curItem.Events[0]
				switch {
				case snapDone:
					payload, ok := e.Payload.(simplePayload)
					require.True(t, ok, "want health event got: %#v", e.Payload)
					updateIDs = append(updateIDs, payload.value)
					if len(updateIDs) == tc.updatesAfterSnap {
						// We're done!
						break RECV
					}
				case e.IsEndOfSnapshot():
					snapDone = true
				default:
					payload, ok := e.Payload.(simplePayload)
					require.True(t, ok, "want health event got: %#v", e.Payload)
					snapIDs = append(snapIDs, payload.value)
				}
			}

			// Validate the event IDs we got delivered.
			require.Equal(t, genSequentialIDs(0, tc.snapshotSize), snapIDs)
			require.Equal(t, genSequentialIDs(20000, 20000+tc.updatesAfterSnap), updateIDs)
		})
	}
}

func genSequentialIDs(start, end int) []string {
	ids := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		ids = append(ids, fmt.Sprintf("test-event-%03d", i))
	}
	return ids
}

func testHealthConsecutiveSnapshotFn(size int, index uint64) SnapshotFunc {
	return func(req SubscribeRequest, buf SnapshotAppender) (uint64, error) {
		for i := 0; i < size; i++ {
			// Event content is arbitrary we are just using Health because it's the
			// first type defined. We just want a set of things with consecutive
			// names.
			buf.Append([]Event{newDefaultHealthEvent(index, i)})
		}
		return index, nil
	}
}

func newDefaultHealthEvent(index uint64, n int) Event {
	return Event{
		Index:   index,
		Topic:   testTopic,
		Payload: simplePayload{value: fmt.Sprintf("test-event-%03d", n)},
	}
}
