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

// StateSyncer manages background synchronization of the given state.
//
// The state is synchronized on a regular basis or on demand when either
// the state has changed or a new Consul server has joined the cluster.
//
// The regular state sychronization provides a self-healing mechanism
// for the cluster which is also called anti-entropy.
type StateSyncer struct {
	// paused flags whether sync runs are temporarily disabled.
	// Must be the first element due to a go bug.
	// todo(fs): which bug? Is this still relevant?
	paused int32

	// State contains the data that needs to be synchronized.
	State interface {
		UpdateSyncState() error
		SyncChanges() error
	}

	// Interval is the time between two regular sync runs.
	Interval time.Duration

	// ClusterSize returns the number of members in the cluster to
	// allow staggering the sync runs based on cluster size.
	ClusterSize func() int

	// ShutdownCh is closed when the application is shutting down.
	ShutdownCh chan struct{}

	// ServerUpCh contains data when a new consul server has been added to the cluster.
	ServerUpCh chan struct{}

	// TriggerCh contains data when a sync should run immediately.
	TriggerCh chan struct{}

	Logger *log.Logger
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
	stagger := func(d time.Duration) time.Duration {
		f := scaleFactor(s.ClusterSize())
		return lib.RandomStagger(time.Duration(f) * d)
	}

Sync:
	for {
		switch err := s.State.UpdateSyncState(); {

		// update sync status failed
		case err != nil:
			s.Logger.Printf("[ERR] agent: failed to sync remote state: %v", err)

			// retry updating sync status after some time or when a consul
			// server was added.
			select {

			// consul server added to cluster.
			// retry sooner than retryFailIntv to converge cluster sooner
			// but stagger delay to avoid thundering herd
			case <-s.ServerUpCh:
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

		// update sync status OK
		default:
			// force-trigger sync to pickup any changes
			s.triggerSync()

			// do partial syncs until it is time for a full sync again
			for {
				select {
				// todo(fs): why don't we honor the ServerUpCh here as well?
				// todo(fs): by default, s.Interval is 60s which is >> 3s (serverUpIntv)
				// case <-s.ServerUpCh:
				// 	select {
				// 	case <-time.After(stagger(serverUpIntv)):
				// 		continue Sync
				// 	case <-s.ShutdownCh:
				// 		return
				// 	}

				case <-time.After(s.Interval + stagger(s.Interval)):
					continue Sync

				case <-s.TriggerCh:
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
}

// Pause temporarily disables sync runs.
func (s *StateSyncer) Pause() {
	atomic.AddInt32(&s.paused, 1)
}

// Paused returns whether sync runs are temporarily disabled.
func (s *StateSyncer) Paused() bool {
	return atomic.LoadInt32(&s.paused) > 0
}

// Resume re-enables sync runs.
func (s *StateSyncer) Resume() {
	paused := atomic.AddInt32(&s.paused, -1)
	if paused < 0 {
		panic("unbalanced StateSyncer.Resume() detected")
	}
	s.triggerSync()
}

// triggerSync queues a sync run if one has not been triggered already.
func (s *StateSyncer) triggerSync() {
	select {
	case s.TriggerCh <- struct{}{}:
	default:
	}
}
