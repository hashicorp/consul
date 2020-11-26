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

	// TODO: accept this value from the constructor instead of setting the field
	Delayer Delayer

	// SyncFull allows triggering an immediate but staggered full sync
	// in a non-blocking way.
	SyncFull *Trigger

	// SyncChanges allows triggering an immediate partial sync
	// in a non-blocking way.
	SyncChanges *Trigger

	// paused stores whether sync runs are temporarily disabled.
	pauseLock sync.Mutex
	paused    int
	chPaused  chan struct{}

	// serverUpInterval is the max time after which a full sync is
	// performed when a server has been added to the cluster.
	serverUpInterval time.Duration

	// retryFailInterval is the time after which a failed full sync is retried.
	retryFailInterval time.Duration

	// waitNextFullSync is a chan that receives a time.Time when the next
	// full sync should occur.
	waitNextFullSync <-chan time.Time
}

// Delayer calculates a duration used to delay the next sync operation after a sync
// is performed.
type Delayer interface {
	Jitter(time.Duration) time.Duration
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

	return s
}

// fsmState defines states for the state machine.
type fsmState string

const (
	doneState        fsmState = "done"
	fullSyncState    fsmState = "fullSync"
	partialSyncState fsmState = "partialSync"
)

// Run is the long running method to perform state synchronization
// between local and remote servers.
func (s *StateSyncer) Run() {
	state := fullSyncState
	for state != doneState {
		state = s.nextFSMState(state)
	}
}

// nextFSMState determines the next state based on the current state.
func (s *StateSyncer) nextFSMState(fs fsmState) fsmState {
	switch fs {
	case fullSyncState:
		s.waitNextFullSync = time.After(s.Interval + s.Delayer.Jitter(s.Interval))
		if s.isPaused() {
			return s.retryFullSync()
		}

		if err := s.State.SyncFull(); err != nil {
			s.Logger.Error("failed to sync remote state", "error", err)
			return s.retryFullSync()
		}

		return partialSyncState

	case partialSyncState:
		select {
		case <-s.SyncFull.wait():
			return s.waitFullSyncDelay()

		case <-s.waitNextFullSync:
			return fullSyncState

		case <-s.SyncChanges.wait():
			if s.isPaused() {
				return partialSyncState
			}

			if err := s.State.SyncChanges(); err != nil {
				s.Logger.Error("failed to sync changes", "error", err)
			}
			return partialSyncState

		case <-s.ShutdownCh:
			return doneState
		}

	default:
		panic(fmt.Sprintf("invalid state: %s", fs))
	}
}

func (s *StateSyncer) retryFullSync() fsmState {
	// FIXME: We enter this state if StateSyncer.isPaused, but Resume does
	// not SyncFull.Trigger. It only calls SyncChanges.Trigger. This seems
	// like an oversight. Entering this loop will block until a server is
	// added (rare), an acl token is changed (rare), or after waiting
	// for the retryFailInterval.
	select {
	case <-s.SyncFull.Wait():
		return s.waitFullSyncDelay()

	// retry full sync after some time
	// it is using retryFailInterval because it is retrying the sync
	case <-time.After(s.retryFailInterval + s.Delayer.Jitter(s.retryFailInterval)):
		return fullSyncState

	case <-s.ShutdownCh:
		return doneState
	}
}

func (s *StateSyncer) waitFullSyncDelay() fsmState {
	select {
	case <-time.After(s.Delayer.Jitter(s.serverUpInterval)):
		return fullSyncState
	case <-s.ShutdownCh:
		return doneState
	}
}

// shim for testing
var libRandomStagger = lib.RandomStagger

func NewClusterSizeDelayer(size func() int) Delayer {
	return delayer{fn: size}
}

type delayer struct {
	fn func() int
}

// Jitter returns a random duration which depends on the cluster size
// and a random factor which should provide some timely distribution of
// cluster wide events.
func (d delayer) Jitter(delay time.Duration) time.Duration {
	return libRandomStagger(time.Duration(scaleFactor(d.fn())) * delay)
}

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

// Pause temporarily disables sync runs.
func (s *StateSyncer) Pause() {
	s.pauseLock.Lock()
	s.paused++

	if s.chPaused == nil {
		s.chPaused = make(chan struct{})
	}
	s.pauseLock.Unlock()
}

// Paused returns whether sync runs are temporarily disabled.
func (s *StateSyncer) isPaused() bool {
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
	resumed := s.paused == 0

	if resumed {
		close(s.chPaused)
		s.chPaused = nil
	}
	s.pauseLock.Unlock()

	if resumed {
		s.SyncChanges.Trigger()
	}
	return resumed
}

// WaitResume returns a channel which blocks until the StateSyncer has been
// resumed.
// If StateSyncer is not paused, WaitResume returns nil.
func (s *StateSyncer) WaitResume() <-chan struct{} {
	s.pauseLock.Lock()
	defer s.pauseLock.Unlock()
	return s.chPaused
}
