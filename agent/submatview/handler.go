// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package submatview

import (
	"github.com/hashicorp/consul/proto/private/pbsubscribe"
)

// eventHandler is a function which performs some operation on the received
// events, then returns the eventHandler that should be used for the next set
// of events.
// If eventHandler fails to handle the events it may return an error. If an
// error is returned the next eventHandler will be ignored.
// eventHandler is used to implement a very simple finite-state machine.
type eventHandler func(state viewState, events *pbsubscribe.Event) (next eventHandler, err error)

type viewState interface {
	updateView(events []*pbsubscribe.Event, index uint64) error
	reset()
}

func initialHandler(index uint64) eventHandler {
	if index == 0 {
		return newSnapshotHandler()
	}
	return resumeStreamHandler
}

// snapshotHandler accumulates events. When it receives an EndOfSnapshot event
// it updates the view, and then returns eventStreamHandler to handle new events.
type snapshotHandler struct {
	events []*pbsubscribe.Event
}

func newSnapshotHandler() eventHandler {
	return (&snapshotHandler{}).handle
}

func (h *snapshotHandler) handle(state viewState, event *pbsubscribe.Event) (eventHandler, error) {
	if event.GetEndOfSnapshot() {
		err := state.updateView(h.events, event.Index)
		return eventStreamHandler, err
	}

	h.events = append(h.events, eventsFromEvent(event)...)
	return h.handle, nil
}

// eventStreamHandler handles events by updating the view. It always returns
// itself as the next handler.
func eventStreamHandler(state viewState, event *pbsubscribe.Event) (eventHandler, error) {
	err := state.updateView(eventsFromEvent(event), event.Index)
	return eventStreamHandler, err
}

func eventsFromEvent(event *pbsubscribe.Event) []*pbsubscribe.Event {
	if batch := event.GetEventBatch(); batch != nil {
		return batch.Events
	}
	return []*pbsubscribe.Event{event}
}

// resumeStreamHandler checks if the event is a NewSnapshotToFollow event. If it
// is it resets the view and returns a snapshotHandler to handle the next event.
// Otherwise it uses eventStreamHandler to handle events.
func resumeStreamHandler(state viewState, event *pbsubscribe.Event) (eventHandler, error) {
	if event.GetNewSnapshotToFollow() {
		state.reset()
		return newSnapshotHandler(), nil
	}
	return eventStreamHandler(state, event)
}
