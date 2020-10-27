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
	// First item in the buffer. Used as the Head of a subscription, or to
	// splice this snapshot onto another one.
	First *bufferItem

	// buffer is the Head of the snapshot buffer the fn should write to.
	buffer *eventBuffer
}

// newEventSnapshot creates an empty snapshot buffer.
func newEventSnapshot() *eventSnapshot {
	snapBuffer := newEventBuffer()
	return &eventSnapshot{
		First:  snapBuffer.Head(),
		buffer: snapBuffer,
	}
}

// appendAndSlice populates the snapshot buffer by calling the SnapshotFunc,
// then adding an endOfSnapshot framing event, and finally by splicing in
// events from the topicBuffer.
func (s *eventSnapshot) appendAndSplice(req SubscribeRequest, fn SnapshotFunc, topicBufferHead *bufferItem) {
	idx, err := fn(req, s.buffer)
	if err != nil {
		s.buffer.AppendItem(&bufferItem{Err: err})
		return
	}
	s.buffer.Append([]Event{{
		Topic:   req.Topic,
		Key:     req.Key,
		Index:   idx,
		Payload: endOfSnapshot{},
	}})
	s.spliceFromTopicBuffer(topicBufferHead, idx)
}

// spliceFromTopicBuffer traverses the topicBuffer looking for the last item
// in the buffer, or the first item where the index is greater than idx. Once
// the item is found it is appended to the snapshot buffer.
func (s *eventSnapshot) spliceFromTopicBuffer(topicBufferHead *bufferItem, idx uint64) {
	item := topicBufferHead
	for {
		next := item.NextNoBlock()
		switch {
		case next == nil:
			// This is the head of the topic buffer (or was just now which is after
			// the snapshot completed). We don't want any of the events (if any) in
			// the snapshot buffer as they came before the snapshot but we do need to
			// wait for the next update.
			s.buffer.AppendItem(item.NextLink())
			return

		case next.Err != nil:
			// This case is not currently possible because errors can only come
			// from a snapshot func, and this is consuming events from a topic
			// buffer which does not contain a snapshot.
			// Handle this case anyway in case errors can come from other places
			// in the future.
			s.buffer.AppendItem(next)
			return

		case len(next.Events) > 0 && next.Events[0].Index > idx:
			// We've found an update in the topic buffer that happened after our
			// snapshot was taken, splice it into the snapshot buffer so subscribers
			// can continue to read this and others after it.
			s.buffer.AppendItem(next)
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
	head := s.buffer.Head()
	return head.Err
}
