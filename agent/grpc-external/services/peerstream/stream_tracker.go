package peerstream

import (
	"fmt"
	"sync"
	"time"
)

// Tracker contains a map of (PeerID -> Status).
// As streams are opened and closed we track details about their status.
type Tracker struct {
	mu      sync.RWMutex
	streams map[string]*MutableStatus

	// timeNow is a shim for testing.
	timeNow func() time.Time
}

func NewTracker() *Tracker {
	return &Tracker{
		streams: make(map[string]*MutableStatus),
		timeNow: time.Now,
	}
}

func (t *Tracker) SetClock(clock func() time.Time) {
	if clock == nil {
		t.timeNow = time.Now
	} else {
		t.timeNow = clock
	}
}

// Connected registers a stream for a given peer, and marks it as connected.
// It also enforces that there is only one active stream for a peer.
func (t *Tracker) Connected(id string) (*MutableStatus, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	status, ok := t.streams[id]
	if !ok {
		status = newMutableStatus(t.timeNow)
		t.streams[id] = status
		return status, nil
	}

	if status.IsConnected() {
		return nil, fmt.Errorf("there is an active stream for the given PeerID %q", id)
	}
	status.TrackConnected()

	return status, nil
}

// Disconnected ensures that if a peer id's stream status is tracked, it is marked as disconnected.
func (t *Tracker) Disconnected(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if status, ok := t.streams[id]; ok {
		status.TrackDisconnected()
	}
}

func (t *Tracker) StreamStatus(id string) (resp Status, found bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.streams[id]
	if !ok {
		return Status{}, false
	}
	return s.GetStatus(), true
}

func (t *Tracker) ConnectedStreams() map[string]chan struct{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	resp := make(map[string]chan struct{})
	for peer, status := range t.streams {
		if status.IsConnected() {
			resp[peer] = status.doneCh
		}
	}
	return resp
}

func (t *Tracker) DeleteStatus(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.streams, id)
}

type MutableStatus struct {
	mu sync.RWMutex

	// timeNow is a shim for testing.
	timeNow func() time.Time

	// doneCh allows for shutting down a stream gracefully by sending a termination message
	// to the peer before the stream's context is cancelled.
	doneCh chan struct{}

	Status
}

// Status contains information about the replication stream to a peer cluster.
// TODO(peering): There's a lot of fields here...
type Status struct {
	// Connected is true when there is an open stream for the peer.
	Connected bool

	// If the status is not connected, DisconnectTime tracks when the stream was closed. Else it's zero.
	DisconnectTime time.Time

	// LastAck tracks the time we received the last ACK for a resource replicated TO the peer.
	LastAck time.Time

	// LastNack tracks the time we received the last NACK for a resource replicated to the peer.
	LastNack time.Time

	// LastNackMessage tracks the reported error message associated with the last NACK from a peer.
	LastNackMessage string

	// LastSendError tracks the time of the last error sending into the stream.
	LastSendError time.Time

	// LastSendErrorMessage tracks the last error message when sending into the stream.
	LastSendErrorMessage string

	// LastReceiveSuccess tracks the time we last successfully stored a resource replicated FROM the peer.
	LastReceiveSuccess time.Time

	// LastReceiveError tracks either:
	// - The time we failed to store a resource replicated FROM the peer.
	// - The time of the last error when receiving from the stream.
	LastReceiveError time.Time

	// LastReceiveError tracks either:
	// - The error message when we failed to store a resource replicated FROM the peer.
	// - The last error message when receiving from the stream.
	LastReceiveErrorMessage string
}

func newMutableStatus(now func() time.Time) *MutableStatus {
	return &MutableStatus{
		Status: Status{
			Connected: true,
		},
		timeNow: now,
		doneCh:  make(chan struct{}),
	}
}

func (s *MutableStatus) Done() <-chan struct{} {
	return s.doneCh
}

func (s *MutableStatus) TrackAck() {
	s.mu.Lock()
	s.LastAck = s.timeNow().UTC()
	s.mu.Unlock()
}

func (s *MutableStatus) TrackSendError(error string) {
	s.mu.Lock()
	s.LastSendError = s.timeNow().UTC()
	s.LastSendErrorMessage = error
	s.mu.Unlock()
}

func (s *MutableStatus) TrackReceiveSuccess() {
	s.mu.Lock()
	s.LastReceiveSuccess = s.timeNow().UTC()
	s.mu.Unlock()
}

func (s *MutableStatus) TrackReceiveError(error string) {
	s.mu.Lock()
	s.LastReceiveError = s.timeNow().UTC()
	s.LastReceiveErrorMessage = error
	s.mu.Unlock()
}

func (s *MutableStatus) TrackNack(msg string) {
	s.mu.Lock()
	s.LastNack = s.timeNow().UTC()
	s.LastNackMessage = msg
	s.mu.Unlock()
}

func (s *MutableStatus) TrackConnected() {
	s.mu.Lock()
	s.Connected = true
	s.DisconnectTime = time.Time{}
	s.mu.Unlock()
}

func (s *MutableStatus) TrackDisconnected() {
	s.mu.Lock()
	s.Connected = false
	s.DisconnectTime = s.timeNow().UTC()
	s.mu.Unlock()
}

func (s *MutableStatus) IsConnected() bool {
	var resp bool

	s.mu.RLock()
	resp = s.Connected
	s.mu.RUnlock()

	return resp
}

func (s *MutableStatus) GetStatus() Status {
	s.mu.RLock()
	copy := s.Status
	s.mu.RUnlock()

	return copy
}
