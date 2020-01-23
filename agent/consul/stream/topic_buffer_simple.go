package stream

/*
type simpleTopicBuffer struct {
	l      sync.Mutex
	ch     chan struct{}
	events [][]Event
}

func (b *simpleTopicBuffer) Append(evts []Event) error {
	b.l.Lock()
	defer b.l.Unlock()

	if len(evts) < 1 {
		return nil
	}

	b.events = append(b.events, evts)

	// If anyone was watching, signal them
	if b.ch != nil {
		close(b.ch)
		b.ch = nil
	}

	return nil
}

func (b *simpleTopicBuffer) NextEvent(ctx context.Context, cursor uint64) ([]Event, error) {
	b.l.Lock()
	defer b.l.Unlock()

RESTART:
	// Search for the first event after cursor
	i := b.nextEventIndexLocked(cursor)

	if i == len(b.events) {
		// Not found any events with larger index, wait for one!
		if b.ch == nil {
			b.ch = make(chan struct{})
		}
		b.l.Unlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-b.ch:
		}

		// Something was published, should be after cursor but start again...
		b.l.Lock()
		goto RESTART
	}

	if i == 0 {
		// If the first event is larger then we don't know if there were missing
		// events.
		return nil, ErrCursorNotInBuffer
	}

	return b.events[i], nil
}

// nextEventIndexLocked returns the index into events that contains the next
// events after events at cursor. If cursor events are no longer present it will
// return 0. If cursor events are the most recent it will return len(events).
func (b *simpleTopicBuffer) nextEventIndexLocked(cursor uint64) int {
	// First assume that most of the time the clients are waiting for a new event
	// so the newest event will be pretty near the end of the buffer. Linear scan
	// the last k events to find it.
	k := 100
	len := len(b.events)
	for i := 0; i < k; i++ {
		// Look back i events from the end
		idx := len - 1 - i
		if idx < 0 {
			return 0
		}
		if b.events[idx][0].Index == cursor {
			return idx + 1
		}
	}

	// We looked at the most recent k but didn't find it. In case the buffer is
	// huge and this subscriber is way behind, binary search to it from here.
	return sort.Search(len, func(i int) bool {
		return b.events[i][0].Index > cursor
	})
}

func (b *simpleTopicBuffer) Truncate(cursor uint64) error {
	b.l.Lock()
	defer b.l.Unlock()

	i := b.nextEventIndexLocked(cursor)
	if i == 0 {
		return nil
	}

	if i == len(b.events) {
		b.events = nil
		return nil
	}

	// Make a new slice with only events from i onwards. We make a whole new slice
	// instead of reslicing so that the old array can be deallocated and garbage
	// collected over time.
	old := b.events
	b.events = make([][]Event, 0, len(old)-i)
	copy(b.events[:], old[i:])

	return nil
}
*/
