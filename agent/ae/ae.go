// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ae provides tools to synchronize state between local and remote consul servers.
package ae

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
)

// scaleThreshold is the number of nodes after which regular sync runs are
// spread out farther apart. The value should be a power of 2 since the
// scale function uses log2.
//
// When set to 128 nodes the delay between regular runs is doubled when the
// cluster is larger than 128 nodes. It doubles again when it passes 256
// nodes, and again at 512 nodes and so forth. At 8192 nodes, the delay
// factor is 8.
//
// If you update this, you may need to adjust the tuning of
// CoordinateUpdatePeriod and CoordinateUpdateMaxBatchSize.
const scaleThreshold = 128

// scaleFactor returns a factor by which the next sync run should be delayed to
// avoid saturation of the cluster. The larger the cluster grows the farther
// the sync runs should be spread apart.
//
// The current implementation uses a log2 scale which doubles the delay between
// runs every time the cluster doubles in size.
func scaleFactor(nodes int) int {
	if nodes <= scaleThreshold {
		return 1.0
	}
	return int(math.Ceil(math.Log2(float64(nodes))-math.Log2(float64(scaleThreshold))) + 1.0)
}

type SyncState interface {
	SyncChanges() error
	SyncFull() error
}

// StateSyncer manages background synchronization of the given state.
//
// The state is synchronized on a regular basis or on demand when either
// the state has changed or a new Consul server has joined the cluster.
//
// The regular state synchronization provides a self-healing mechanism
// for the cluster which is also called anti-entropy.
type StateSyncer struct {
	// State contains the data that needs to be synchronized.
	State SyncState

	// Interval is the time between two full sync runs.
	Interval time.Duration

	// ShutdownCh is closed when the application is shutting down.
	ShutdownCh chan struct{}

	// Logger is the logger.
	Logger hclog.Logger

	// ClusterSize returns the number of members in the cluster to
	// allow staggering the sync runs based on cluster size.
	// This needs to be set before Run() is called.
	ClusterSize func() int

	// SyncFull allows triggering an immediate but staggered full sync
	// in a non-blocking way.
	SyncFull *Trigger

	// SyncChanges allows triggering an immediate partial sync
	// in a non-blocking way.
	SyncChanges *Trigger

	// paused stores whether sync runs are temporarily disabled.
	pauseLock sync.Mutex
	paused    int

	// serverUpInterval is the max time after which a full sync is
	// performed when a server has been added to the cluster.
	serverUpInterval time.Duration

	// retryFailInterval is the time after which a failed full sync is retried.
	retryFailInterval time.Duration

	// stagger randomly picks a duration between 0s and the given duration.
	stagger func(time.Duration) time.Duration

	// retrySyncFullEvent generates an event based on multiple conditions
	// when the state machine is trying to retry a full state sync.
	retrySyncFullEvent func() event

	// syncChangesEvent generates an event based on multiple conditions
	// when the state machine is performing partial state syncs.
	syncChangesEvent func() event

	// nextFullSyncCh is a chan that receives a time.Time when the next
	// full sync should occur.
	nextFullSyncCh <-chan time.Time
}

const (
	// serverUpIntv is the max time to wait before a sync is triggered
	// when a consul server has been added to the cluster.
	serverUpIntv = 3 * time.Second

	// retryFailIntv is the min time to wait before a failed sync is retried.
	retryFailIntv = 15 * time.Second
)

func NewStateSyncer(state SyncState, intv time.Duration, shutdownCh chan struct{}, logger hclog.Logger) *StateSyncer {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{})
	}

	s := &StateSyncer{
		State:             state,
		Interval:          intv,
		ShutdownCh:        shutdownCh,
		Logger:            logger.Named(logging.AntiEntropy),
		SyncFull:          NewTrigger(),
		SyncChanges:       NewTrigger(),
		serverUpInterval:  serverUpIntv,
		retryFailInterval: retryFailIntv,
	}

	// retain these methods as member variables so that
	// we can mock them for testing.
	s.retrySyncFullEvent = s.retrySyncFullEventFn
	s.syncChangesEvent = s.syncChangesEventFn
	s.stagger = s.staggerFn

	return s
}

// fsmState defines states for the state machine.
type fsmState string

const (
	doneState          fsmState = "done"
	fullSyncState      fsmState = "fullSync"
	partialSyncState   fsmState = "partialSync"
	retryFullSyncState fsmState = "retryFullSync"
)

// Run is the long running method to perform state synchronization
// between local and remote servers.
func (s *StateSyncer) Run() {
	if s.ClusterSize == nil {
		panic("ClusterSize not set")
	}
	s.resetNextFullSyncCh()
	s.runFSM(fullSyncState, s.nextFSMState)
}

// runFSM runs the state machine.
func (s *StateSyncer) runFSM(fs fsmState, next func(fsmState) fsmState) {
	for {
		if fs = next(fs); fs == doneState {
			return
		}
	}
}

// nextFSMState determines the next state based on the current state.
func (s *StateSyncer) nextFSMState(fs fsmState) fsmState {
	switch fs {
	case fullSyncState:
		if s.Paused() {
			return retryFullSyncState
		}

		if err := s.State.SyncFull(); err != nil {
			s.Logger.Error("failed to sync remote state", "error", err)
			return retryFullSyncState
		}

		return partialSyncState

	case retryFullSyncState:
		e := s.retrySyncFullEvent()
		switch e {
		case syncFullNotifEvent, syncFullTimerEvent:
			return fullSyncState
		case shutdownEvent:
			return doneState
		default:
			panic(fmt.Sprintf("invalid event: %s", e))
		}

	case partialSyncState:
		e := s.syncChangesEvent()
		switch e {
		case syncFullNotifEvent, syncFullTimerEvent:
			return fullSyncState

		case syncChangesNotifEvent:
			if s.Paused() {
				return partialSyncState
			}

			err := s.State.SyncChanges()
			if err != nil {
				s.Logger.Error("failed to sync changes", "error", err)
			}
			return partialSyncState

		case shutdownEvent:
			return doneState

		default:
			panic(fmt.Sprintf("invalid event: %s", e))
		}

	default:
		panic(fmt.Sprintf("invalid state: %s", fs))
	}
}

// event defines a timing or notification event from multiple timers and
// channels.
type event string

const (
	shutdownEvent         event = "shutdown"
	syncFullNotifEvent    event = "syncFullNotif"
	syncFullTimerEvent    event = "syncFullTimer"
	syncChangesNotifEvent event = "syncChangesNotif"
)

// retrySyncFullEventFn waits for an event which triggers a retry
// of a full sync or a termination signal. This function should not be
// called directly but through s.retryFullSyncState to allow mocking for
// testing.
func (s *StateSyncer) retrySyncFullEventFn() event {
	select {
	// trigger a full sync immediately.
	// this is usually called when a consul server was added to the cluster.
	// stagger the delay to avoid a thundering herd.
	case <-s.SyncFull.Notif():
		select {
		case <-time.After(s.stagger(s.serverUpInterval)):
			return syncFullNotifEvent
		case <-s.ShutdownCh:
			return shutdownEvent
		}

	// retry full sync after some time
	// it is using retryFailInterval because it is retrying the sync
	case <-time.After(s.retryFailInterval + s.stagger(s.retryFailInterval)):
		s.resetNextFullSyncCh()
		return syncFullTimerEvent

	case <-s.ShutdownCh:
		return shutdownEvent
	}
}

// syncChangesEventFn waits for a event which either triggers a full
// or a partial sync or a termination signal. This function should not
// be called directly but through s.syncChangesEvent to allow mocking
// for testing.
func (s *StateSyncer) syncChangesEventFn() event {
	select {
	// trigger a full sync immediately
	// this is usually called when a consul server was added to the cluster.
	// stagger the delay to avoid a thundering herd.
	case <-s.SyncFull.Notif():
		select {
		case <-time.After(s.stagger(s.serverUpInterval)):
			s.resetNextFullSyncCh()
			return syncFullNotifEvent
		case <-s.ShutdownCh:
			return shutdownEvent
		}

	// time for a full sync again
	case <-s.nextFullSyncCh:
		s.resetNextFullSyncCh()
		return syncFullTimerEvent

	// do partial syncs on demand
	case <-s.SyncChanges.Notif():
		return syncChangesNotifEvent

	case <-s.ShutdownCh:
		return shutdownEvent
	}
}

// resetNextFullSyncCh resets nextFullSyncCh and sets it to interval+stagger.
// Call this function everytime a full sync is performed.
func (s *StateSyncer) resetNextFullSyncCh() {
	if s.stagger != nil {
		s.nextFullSyncCh = time.After(s.Interval + s.stagger(s.Interval))
	} else {
		s.nextFullSyncCh = time.After(s.Interval)
	}
}

// stubbed out for testing
var libRandomStagger = lib.RandomStagger

// staggerFn returns a random duration which depends on the cluster size
// and a random factor which should provide some timely distribution of
// cluster wide events. This function should not be called directly
// but through s.stagger to allow mocking for testing.
func (s *StateSyncer) staggerFn(d time.Duration) time.Duration {
	f := scaleFactor(s.ClusterSize())
	return libRandomStagger(time.Duration(f) * d)
}

// Pause temporarily disables sync runs.
func (s *StateSyncer) Pause() {
	s.pauseLock.Lock()
	s.paused++
	s.pauseLock.Unlock()
}

// Paused returns whether sync runs are temporarily disabled.
func (s *StateSyncer) Paused() bool {
	s.pauseLock.Lock()
	defer s.pauseLock.Unlock()
	return s.paused != 0
}

// Resume re-enables sync runs. It returns true if it was the last pause/resume
// pair on the stack and so actually caused the state syncer to resume.
func (s *StateSyncer) Resume() bool {
	s.pauseLock.Lock()
	s.paused--
	if s.paused < 0 {
		panic("unbalanced pause/resume")
	}
	trigger := s.paused == 0
	s.pauseLock.Unlock()
	if trigger {
		s.SyncChanges.Trigger()
	}
	return trigger
}
