// Package ae provides an anti-entropy mechanism for the local state.
package ae

import (
	"log"
	"math"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/lib"
)

const (
	// This scale factor means we will add a minute after we cross 128 nodes,
	// another at 256, another at 512, etc. By 8192 nodes, we will scale up
	// by a factor of 8.
	//
	// If you update this, you may need to adjust the tuning of
	// CoordinateUpdatePeriod and CoordinateUpdateMaxBatchSize.
	aeScaleThreshold = 128

	syncStaggerIntv = 3 * time.Second
	syncRetryIntv   = 15 * time.Second
)

// aeScale is used to scale the time interval at which anti-entropy updates take
// place. It is used to prevent saturation as the cluster size grows.
func aeScale(d time.Duration, n int) time.Duration {
	// Don't scale until we cross the threshold
	if n <= aeScaleThreshold {
		return d
	}

	mult := math.Ceil(math.Log2(float64(n))-math.Log2(aeScaleThreshold)) + 1.0
	return time.Duration(mult) * d
}

type StateSyncer struct {
	// paused is used to check if we are paused. Must be the first
	// element due to a go bug.
	// todo(fs): which bug? still relevant?
	paused int32

	// State contains the data that needs to be synchronized.
	State interface {
		UpdateSyncState() error
		SyncChanges() error
	}

	// Interval is the time between two sync runs.
	Interval time.Duration

	// ClusterSize returns the number of members in the cluster.
	// todo(fs): we use this for staggering but what about a random number?
	ClusterSize func() int

	// ShutdownCh is closed when the application is shutting down.
	ShutdownCh chan struct{}

	// ConsulCh contains data when a new consul server has been added to the cluster.
	ConsulCh chan struct{}

	// TriggerCh contains data when a sync should run immediately.
	TriggerCh chan struct{}

	Logger *log.Logger
}

// Pause is used to pause state synchronization, this can be
// used to make batch changes
func (ae *StateSyncer) Pause() {
	atomic.AddInt32(&ae.paused, 1)
}

// Resume is used to resume state synchronization
func (ae *StateSyncer) Resume() {
	paused := atomic.AddInt32(&ae.paused, -1)
	if paused < 0 {
		panic("unbalanced State.Resume() detected")
	}
	ae.changeMade()
}

// Paused is used to check if we are paused
func (ae *StateSyncer) Paused() bool {
	return atomic.LoadInt32(&ae.paused) > 0
}

func (ae *StateSyncer) changeMade() {
	select {
	case ae.TriggerCh <- struct{}{}:
	default:
	}
}

// antiEntropy is a long running method used to perform anti-entropy
// between local and remote state.
func (ae *StateSyncer) Run() {
SYNC:
	// Sync our state with the servers
	for {
		err := ae.State.UpdateSyncState()
		if err == nil {
			break
		}
		ae.Logger.Printf("[ERR] agent: failed to sync remote state: %v", err)
		select {
		case <-ae.ConsulCh:
			// Stagger the retry on leader election, avoid a thundering heard
			select {
			case <-time.After(lib.RandomStagger(aeScale(syncStaggerIntv, ae.ClusterSize()))):
			case <-ae.ShutdownCh:
				return
			}
		case <-time.After(syncRetryIntv + lib.RandomStagger(aeScale(syncRetryIntv, ae.ClusterSize()))):
		case <-ae.ShutdownCh:
			return
		}
	}

	// Force-trigger AE to pickup any changes
	ae.changeMade()

	// Schedule the next full sync, with a random stagger
	aeIntv := aeScale(ae.Interval, ae.ClusterSize())
	aeIntv = aeIntv + lib.RandomStagger(aeIntv)
	aeTimer := time.After(aeIntv)

	// Wait for sync events
	for {
		select {
		case <-aeTimer:
			goto SYNC
		case <-ae.TriggerCh:
			// Skip the sync if we are paused
			if ae.Paused() {
				continue
			}
			if err := ae.State.SyncChanges(); err != nil {
				ae.Logger.Printf("[ERR] agent: failed to sync changes: %v", err)
			}
		case <-ae.ShutdownCh:
			return
		}
	}
}
