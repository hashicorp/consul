package stream

// eventSnapshot represents the state of memdb for a given topic and key at some
// point in time. It is modelled as a buffer of events so that snapshots can be
// streamed to possibly multiple subscribers concurrently, and can be trivially
// cached by retaining a reference to a Snapshot. Once the reference to eventSnapshot
// is dropped from memory, any subscribers still reading from it may do so by following
// their pointers. When the last subscriber unsubscribes, the snapshot is garbage
// collected automatically by Go's runtime. This simplifies snapshot and buffer
// management dramatically.
type eventSnapshot struct {
	// Head is the first item in the buffer containing the snapshot. Once the
	// snapshot is complete, subsequent BufferItems are appended to snapBuffer,
	// so that subscribers receive all the events from the same buffer.
	Head *bufferItem

	// snapBuffer is the Head of the snapshot buffer the fn should write to.
	snapBuffer *eventBuffer
}

type snapFunc func(req *SubscribeRequest, buf SnapshotAppender) (uint64, error)

// newEventSnapshot creates a snapshot buffer based on the subscription request.
// The current buffer head for the topic requested is passed so that once the
// snapshot is complete and has been delivered into the buffer, any events
// published during snapshotting can be immediately appended and won't be
// missed. Once the snapshot is delivered the topic buffer is spliced onto the
// snapshot buffer so that subscribers will naturally follow from the snapshot
// to wait for any subsequent updates.
func newEventSnapshot(req *SubscribeRequest, topicBufferHead *bufferItem, fn snapFunc) *eventSnapshot {
	buf := newEventBuffer()
	s := &eventSnapshot{
		Head:       buf.Head(),
		snapBuffer: buf,
	}

	go func() {
		idx, err := fn(req, s.snapBuffer)
		if err != nil {
			s.snapBuffer.AppendItem(&bufferItem{Err: err})
			return
		}
		// We wrote the snapshot events to the buffer, send the "end of snapshot" event
		s.snapBuffer.Append([]Event{{
			Topic:   req.Topic,
			Key:     req.Key,
			Index:   idx,
			Payload: endOfSnapshot{},
		}})
		s.spliceFromTopicBuffer(topicBufferHead, idx)
	}()
	return s
}

func (s *eventSnapshot) spliceFromTopicBuffer(topicBufferHead *bufferItem, idx uint64) {
	// Now splice on the topic buffer. We need to iterate through the buffer to
	// find the first event after the current snapshot.
	item := topicBufferHead
	for {
		next := item.NextNoBlock()
		switch {
		case next == nil:
			// This is the head of the topic buffer (or was just now which is after
			// the snapshot completed). We don't want any of the events (if any) in
			// the snapshot buffer as they came before the snapshot but we do need to
			// wait for the next update.
			s.snapBuffer.AppendItem(item.NextLink())
			return

		case next.Err != nil:
			// This case is not currently possible because errors can only come
			// from a snapshot func, and this is consuming events from a topic
			// buffer which does not contain a snapshot.
			// Handle this case anyway in case errors can come from other places
			// in the future.
			s.snapBuffer.AppendItem(next)
			return

		case len(next.Events) > 0 && next.Events[0].Index > idx:
			// We've found an update in the topic buffer that happened after our
			// snapshot was taken, splice it into the snapshot buffer so subscribers
			// can continue to read this and others after it.
			s.snapBuffer.AppendItem(next)
			return
		}

		// We don't need this item, continue to next
		item = next
	}
}

// err returns an error if the snapshot func has failed with an error or nil
// otherwise. Nil doesn't necessarily mean there won't be an error but there
// hasn't been one yet.
func (s *eventSnapshot) err() error {
	// Fetch the head of the buffer, this is atomic. If the snapshot func errored
	// then the last event will be an error.
	head := s.snapBuffer.Head()
	return head.Err
}
