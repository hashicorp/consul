package submatview

import "github.com/hashicorp/consul/proto/pbsubscribe"

type eventHandler func(events *pbsubscribe.Event) (eventHandler, error)

func (m *Materializer) initialHandler(index uint64) eventHandler {
	if index == 0 {
		return newSnapshotHandler(m)
	}
	return m.resumeStreamHandler
}

type snapshotHandler struct {
	material *Materializer
	events   []*pbsubscribe.Event
}

func newSnapshotHandler(m *Materializer) eventHandler {
	return (&snapshotHandler{material: m}).handle
}

func (h *snapshotHandler) handle(event *pbsubscribe.Event) (eventHandler, error) {
	if event.GetEndOfSnapshot() {
		err := h.material.updateView(h.events, event.Index)
		return h.material.eventStreamHandler, err
	}

	h.events = append(h.events, eventsFromEvent(event)...)
	return h.handle, nil
}

func (m *Materializer) eventStreamHandler(event *pbsubscribe.Event) (eventHandler, error) {
	err := m.updateView(eventsFromEvent(event), event.Index)
	return m.eventStreamHandler, err
}

func eventsFromEvent(event *pbsubscribe.Event) []*pbsubscribe.Event {
	if batch := event.GetEventBatch(); batch != nil {
		return batch.Events
	}
	return []*pbsubscribe.Event{event}
}

func (m *Materializer) resumeStreamHandler(event *pbsubscribe.Event) (eventHandler, error) {
	if event.GetNewSnapshotToFollow() {
		m.reset()
		return newSnapshotHandler(m), nil
	}
	return m.eventStreamHandler(event)
}
