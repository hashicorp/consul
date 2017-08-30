// Package ae provides tools to synchronize state between local and remote consul servers.
package ae

import (
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/lib"
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

type State interface {
	SyncChanges() error
	SyncFull() error
}

// StateSyncer manages background synchronization of the given state.
//
// The state is synchronized on a regular basis or on demand when either
// the state has changed or a new Consul server has joined the cluster.
//
// The regular state sychronization provides a self-healing mechanism
// for the cluster which is also called anti-entropy.
type StateSyncer struct {
	// State contains the data that needs to be synchronized.
	State State

	// Interval is the time between two regular sync runs.
	Interval time.Duration

	// ShutdownCh is closed when the application is shutting down.
	ShutdownCh chan struct{}

	// Logger is the logger.
	Logger *log.Logger

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
	paused *toggle
}

func NewStateSyner(state State, intv time.Duration, shutdownCh chan struct{}, logger *log.Logger) *StateSyncer {
	return &StateSyncer{
		State:       state,
		Interval:    intv,
		ShutdownCh:  shutdownCh,
		Logger:      logger,
		SyncFull:    NewTrigger(),
		SyncChanges: NewTrigger(),
		paused:      new(toggle),
	}
}

const (
	// serverUpIntv is the max time to wait before a sync is triggered
	// when a consul server has been added to the cluster.
	serverUpIntv = 3 * time.Second

	// retryFailIntv is the min time to wait before a failed sync is retried.
	retryFailIntv = 15 * time.Second
)

// Run is the long running method to perform state synchronization
// between local and remote servers.
func (s *StateSyncer) Run() {
	if s.ClusterSize == nil {
		panic("ClusterSize not set")
	}

	stagger := func(d time.Duration) time.Duration {
		f := scaleFactor(s.ClusterSize())
		return lib.RandomStagger(time.Duration(f) * d)
	}

FullSync:
	for {
		// attempt a full sync
		if err := s.State.SyncFull(); err != nil {
			s.Logger.Printf("[ERR] agent: failed to sync remote state: %v", err)

			// retry full sync after some time or when a consul
			// server was added.
			select {

			// trigger a full sync immediately.
			// this is usually called when a consul server was added to the cluster.
			// stagger the delay to avoid a thundering herd.
			case <-s.SyncFull.Notif():
				select {
				case <-time.After(stagger(serverUpIntv)):
				case <-s.ShutdownCh:
					return
				}

			// retry full sync after some time
			// todo(fs): why don't we use s.Interval here?
			case <-time.After(retryFailIntv + stagger(retryFailIntv)):

			case <-s.ShutdownCh:
				return
			}

			continue
		}

		// do partial syncs until it is time for a full sync again
		for {
			select {
			// trigger a full sync immediately
			// this is usually called when a consul server was added to the cluster.
			// stagger the delay to avoid a thundering herd.
			case <-s.SyncFull.Notif():
				select {
				case <-time.After(stagger(serverUpIntv)):
					continue FullSync
				case <-s.ShutdownCh:
					return
				}

			// time for a full sync again
			case <-time.After(s.Interval + stagger(s.Interval)):
				continue FullSync

			// do partial syncs on demand
			case <-s.SyncChanges.Notif():
				if s.Paused() {
					continue
				}
				if err := s.State.SyncChanges(); err != nil {
					s.Logger.Printf("[ERR] agent: failed to sync changes: %v", err)
				}

			case <-s.ShutdownCh:
				return
			}
		}
	}
}

// Pause temporarily disables sync runs.
func (s *StateSyncer) Pause() {
	s.paused.On()
}

// Paused returns whether sync runs are temporarily disabled.
func (s *StateSyncer) Paused() bool {
	return s.paused.IsOn()
}

// Resume re-enables sync runs.
func (s *StateSyncer) Resume() {
	s.paused.Off()
	s.SyncChanges.Trigger()
}

// toggle implements an on/off switch using methods from the atomic
// package. Since fields in structs that are accessed via
// atomic.Load/Add methods need to be aligned properly on some platforms
// we move that code into a separate struct.
//
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG for details
type toggle int32

func (p *toggle) On() {
	atomic.AddInt32((*int32)(p), 1)
}

func (p *toggle) Off() {
	if atomic.AddInt32((*int32)(p), -1) < 0 {
		panic("toggle not on")
	}
}

func (p *toggle) IsOn() bool {
	return atomic.LoadInt32((*int32)(p)) > 0
}
