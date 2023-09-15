// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

// Tracker contains a map of (PeerID -> MutableStatus).
// As streams are opened and closed we track details about their status.
type Tracker struct {
	mu      sync.RWMutex
	streams map[string]*MutableStatus

	// heartbeatTimeout is the max duration a connection is allowed to be
	// disconnected before the stream health is reported as non-healthy
	heartbeatTimeout time.Duration

	// timeNow is a shim for testing.
	timeNow func() time.Time
}

func NewTracker(heartbeatTimeout time.Duration) *Tracker {
	if heartbeatTimeout == 0 {
		heartbeatTimeout = defaultIncomingHeartbeatTimeout
	}
	return &Tracker{
		streams:          make(map[string]*MutableStatus),
		timeNow:          time.Now,
		heartbeatTimeout: heartbeatTimeout,
	}
}

// setClock is used for debugging purposes only.
func (t *Tracker) setClock(clock func() time.Time) {
	if clock == nil {
		t.timeNow = time.Now
	} else {
		t.timeNow = clock
	}
}

// Register a stream for a given peer but do not mark it as connected.
func (t *Tracker) Register(id string) (*MutableStatus, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	status, _, err := t.registerLocked(id, false)
	return status, err
}

func (t *Tracker) registerLocked(id string, initAsConnected bool) (*MutableStatus, bool, error) {
	status, ok := t.streams[id]
	if !ok {
		status = newMutableStatus(t.timeNow, initAsConnected)
		t.streams[id] = status
		return status, true, nil
	}
	return status, false, nil
}

// Connected registers a stream for a given peer, and marks it as connected.
// It also enforces that there is only one active stream for a peer.
func (t *Tracker) Connected(id string) (*MutableStatus, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connectedLocked(id)
}

func (t *Tracker) connectedLocked(id string) (*MutableStatus, error) {
	status, newlyRegistered, err := t.registerLocked(id, true)
	if err != nil {
		return nil, err
	} else if newlyRegistered {
		return status, nil
	}

	if status.IsConnected() {
		return nil, fmt.Errorf("there is an active stream for the given PeerID %q", id)
	}
	status.TrackConnected()

	return status, nil
}

// DisconnectedGracefully marks the peer id's stream status as disconnected gracefully.
func (t *Tracker) DisconnectedGracefully(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if status, ok := t.streams[id]; ok {
		status.TrackDisconnectedGracefully()
	}
}

// DisconnectedDueToError marks the peer id's stream status as disconnected due to an error.
func (t *Tracker) DisconnectedDueToError(id string, error string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if status, ok := t.streams[id]; ok {
		status.TrackDisconnectedDueToError(error)
	}
}

func (t *Tracker) StreamStatus(id string) (resp Status, found bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.streams[id]
	if !ok {
		return Status{
			NeverConnected: true,
		}, false
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

// IsHealthy is a calculates the health of a peering status.
// We define a peering as unhealthy if its status has been in the following
// states for longer than the configured incomingHeartbeatTimeout.
//   - If it is disconnected
//   - If the last received Nack is newer than last received Ack
//   - If the last received error is newer than last received success
//
// If none of these conditions apply, we call the peering healthy.
func (t *Tracker) IsHealthy(s Status) bool {
	// If stream is in a disconnected state for longer than the configured
	// heartbeat timeout, report as unhealthy.
	if s.DisconnectTime != nil &&
		t.timeNow().Sub(*s.DisconnectTime) > t.heartbeatTimeout {
		return false
	}

	// If last Nack is after last Ack, it means the peer is unable to
	// handle our replication message
	if s.LastAck == nil {
		s.LastAck = &time.Time{}
	}

	if s.LastNack != nil &&
		s.LastNack.After(*s.LastAck) &&
		t.timeNow().Sub(*s.LastAck) > t.heartbeatTimeout {
		return false
	}

	// If last recv error is newer than last recv success, we were unable
	// to handle the peer's replication message.
	if s.LastRecvResourceSuccess == nil {
		s.LastRecvResourceSuccess = &time.Time{}
	}

	if s.LastRecvError != nil &&
		s.LastRecvError.After(*s.LastRecvResourceSuccess) &&
		t.timeNow().Sub(*s.LastRecvError) > t.heartbeatTimeout {
		return false
	}

	return true
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

	// NeverConnected is true for peerings that have never connected, false otherwise.
	NeverConnected bool

	// DisconnectErrorMessage tracks the error that caused the stream to disconnect non-gracefully.
	// If the stream is connected or it disconnected gracefully it will be empty.
	DisconnectErrorMessage string

	// If the status is not connected, DisconnectTime tracks when the stream was closed. Else it's zero.
	DisconnectTime *time.Time

	// LastAck tracks the time we received the last ACK for a resource replicated TO the peer.
	LastAck *time.Time

	// LastNack tracks the time we received the last NACK for a resource replicated to the peer.
	LastNack *time.Time

	// LastNackMessage tracks the reported error message associated with the last NACK from a peer.
	LastNackMessage string

	// LastSendError tracks the time of the last error sending into the stream.
	LastSendError *time.Time

	// LastSendErrorMessage tracks the last error message when sending into the stream.
	LastSendErrorMessage string

	// LastSendSuccess tracks the time we last successfully sent a resource TO the peer.
	LastSendSuccess *time.Time

	// LastRecvHeartbeat tracks when we last received a heartbeat from our peer.
	LastRecvHeartbeat *time.Time

	// LastRecvResourceSuccess tracks the time we last successfully stored a resource replicated FROM the peer.
	LastRecvResourceSuccess *time.Time

	// LastRecvError tracks either:
	// - The time we failed to store a resource replicated FROM the peer.
	// - The time of the last error when receiving from the stream.
	LastRecvError *time.Time

	// LastRecvErrorMessage tracks the last error message when receiving from the stream.
	LastRecvErrorMessage string

	// TODO(peering): consider keeping track of imported and exported services thru raft
	// ImportedServices keeps track of which service names are imported for the peer
	ImportedServices []string
	// ExportedServices keeps track of which service names a peer asks to export
	ExportedServices []string
}

func (s *Status) GetImportedServicesCount() uint64 {
	return uint64(len(s.ImportedServices))
}

func (s *Status) GetExportedServicesCount() uint64 {
	return uint64(len(s.ExportedServices))
}

func newMutableStatus(now func() time.Time, connected bool) *MutableStatus {
	return &MutableStatus{
		Status: Status{
			Connected:      connected,
			NeverConnected: !connected,
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
	s.LastAck = ptr(s.timeNow().UTC())
	s.mu.Unlock()
}

func (s *MutableStatus) TrackSendError(error string) {
	s.mu.Lock()
	s.LastSendError = ptr(s.timeNow().UTC())
	s.LastSendErrorMessage = error
	s.mu.Unlock()
}

func (s *MutableStatus) TrackSendSuccess() {
	s.mu.Lock()
	s.LastSendSuccess = ptr(s.timeNow().UTC())
	s.mu.Unlock()
}

// TrackRecvResourceSuccess tracks receiving a replicated resource.
func (s *MutableStatus) TrackRecvResourceSuccess() {
	s.mu.Lock()
	s.LastRecvResourceSuccess = ptr(s.timeNow().UTC())
	s.mu.Unlock()
}

// TrackRecvHeartbeat tracks receiving a heartbeat from our peer.
func (s *MutableStatus) TrackRecvHeartbeat() {
	s.mu.Lock()
	s.LastRecvHeartbeat = ptr(s.timeNow().UTC())
	s.mu.Unlock()
}

func (s *MutableStatus) TrackRecvError(error string) {
	s.mu.Lock()
	s.LastRecvError = ptr(s.timeNow().UTC())
	s.LastRecvErrorMessage = error
	s.mu.Unlock()
}

func (s *MutableStatus) TrackNack(msg string) {
	s.mu.Lock()
	s.LastNack = ptr(s.timeNow().UTC())
	s.LastNackMessage = msg
	s.mu.Unlock()
}

func (s *MutableStatus) TrackConnected() {
	s.mu.Lock()
	s.Connected = true
	s.DisconnectTime = &time.Time{}
	s.DisconnectErrorMessage = ""
	s.mu.Unlock()
}

// TrackDisconnectedGracefully tracks when the stream was disconnected in a way we expected.
// For example, we got a terminated message, or we terminated the stream ourselves.
func (s *MutableStatus) TrackDisconnectedGracefully() {
	s.mu.Lock()
	s.Connected = false
	s.DisconnectTime = ptr(s.timeNow().UTC())
	s.DisconnectErrorMessage = ""
	s.mu.Unlock()
}

// TrackDisconnectedDueToError tracks when the stream was disconnected due to an error.
// For example the heartbeat timed out, or we couldn't send into the stream.
func (s *MutableStatus) TrackDisconnectedDueToError(error string) {
	s.mu.Lock()
	s.Connected = false
	s.DisconnectTime = ptr(s.timeNow().UTC())
	s.DisconnectErrorMessage = error
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

func (s *MutableStatus) SetImportedServices(serviceNames []structs.ServiceName) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ImportedServices = make([]string, len(serviceNames))
	for i, sn := range serviceNames {
		s.ImportedServices[i] = sn.String()
	}
}

func (s *MutableStatus) GetImportedServicesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.ImportedServices)
}

func (s *MutableStatus) SetExportedServices(serviceNames []structs.ServiceName) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ExportedServices = make([]string, len(serviceNames))

	for i, sn := range serviceNames {
		s.ExportedServices[i] = sn.String()
	}
}

func (s *MutableStatus) GetExportedServicesCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.ExportedServices)
}

func ptr[T any](x T) *T {
	return &x
}
