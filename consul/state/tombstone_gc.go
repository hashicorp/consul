package state

import (
	"fmt"
	"sync"
	"time"
)

// TombstoneGC is used to track creation of tombstones so that they can be
// garbage collected after their TTL expires. The tombstones allow queries to
// provide monotonic index values within the TTL window. The GC is used to
// prevent monotonic growth in storage usage. This is a trade off between the
// length of the TTL and the storage overhead.
//
// In practice, this is required to fix the issue of delete visibility. When
// data is deleted from the KV store, the "latest" row can go backwards if the
// newest row is removed. The tombstones provide a way to ensure time doesn't
// move backwards within some interval.
//
type TombstoneGC struct {
	// ttl sets the TTL for tombstones.
	ttl time.Duration

	// granularity determines how we bin TTLs into timers.
	granularity time.Duration

	// enabled controls if we actually setup any timers.
	enabled bool

	// expires maps the time of expiration to the highest tombstone value
	// that should be expired.
	expires map[time.Time]*expireInterval

	// expireCh is used to stream expiration to the leader for processing.
	expireCh chan uint64

	sync.Mutex
}

// expireInterval is used to track the maximum index to expire in a given
// interval with a timer.
type expireInterval struct {
	// maxIndex has the highest tombstone index that should be GC-d.
	maxIndex uint64

	// timer is the timer tracking this bin.
	timer *time.Timer
}

// NewTombstoneGC is used to construct a new TombstoneGC given a TTL for
// tombstones and a tracking granularity. Longer TTLs ensure correct behavior
// for more time, but use more storage. A shorter granularity increases the
// number of Raft transactions and reduce how far past the TTL we perform GC.
func NewTombstoneGC(ttl, granularity time.Duration) (*TombstoneGC, error) {
	// Sanity check the inputs
	if ttl <= 0 || granularity <= 0 {
		return nil, fmt.Errorf("Tombstone TTL and granularity must be positive")
	}

	t := &TombstoneGC{
		ttl:         ttl,
		granularity: granularity,
		expires:     make(map[time.Time]*expireInterval),
		expireCh:    make(chan uint64, 1),
	}
	return t, nil
}

// ExpireCh is used to return a channel that streams the next index that should
// be expired.
func (t *TombstoneGC) ExpireCh() <-chan uint64 {
	return t.expireCh
}

// SetEnabled is used to control if the tombstone GC is
// enabled. Should only be enabled by the leader node.
func (t *TombstoneGC) SetEnabled(enabled bool) {
	t.Lock()
	defer t.Unlock()
	if enabled == t.enabled {
		return
	}

	// Stop all the timers and clear
	if !enabled {
		for _, exp := range t.expires {
			exp.timer.Stop()
		}
		t.expires = make(map[time.Time]*expireInterval)
	}

	// Update the status
	t.enabled = enabled
}

// Hint is used to indicate that keys at the given index have been
// deleted, and that their GC should be scheduled.
func (t *TombstoneGC) Hint(index uint64) {
	expires := t.nextExpires()

	t.Lock()
	defer t.Unlock()
	if !t.enabled {
		return
	}

	// Check for an existing expiration timer and bump its index if we
	// find one.
	exp, ok := t.expires[expires]
	if ok {
		if index > exp.maxIndex {
			exp.maxIndex = index
		}
		return
	}

	// Create a new expiration timer.
	t.expires[expires] = &expireInterval{
		maxIndex: index,
		timer: time.AfterFunc(expires.Sub(time.Now()), func() {
			t.expireTime(expires)
		}),
	}
}

// PendingExpiration is used to check if any expirations are pending.
func (t *TombstoneGC) PendingExpiration() bool {
	t.Lock()
	defer t.Unlock()

	return len(t.expires) > 0
}

// nextExpires is used to calculate the next expiration time, based on the
// granularity that is set. This allows us to bin expirations and avoid a ton
// of timers.
func (t *TombstoneGC) nextExpires() time.Time {
	expires := time.Now().Add(t.ttl)
	remain := expires.UnixNano() % int64(t.granularity)
	adj := expires.Add(t.granularity - time.Duration(remain))
	return adj
}

// expireTime is used to expire the entries at the given time.
func (t *TombstoneGC) expireTime(expires time.Time) {
	t.Lock()
	defer t.Unlock()

	// Get the maximum index and clear the entry. It's possible that the GC
	// has been shut down while this timer fired and got blocked on the lock,
	// so if there's nothing in the map for us we just exit out since there
	// is no work to do.
	exp, ok := t.expires[expires]
	if !ok {
		return
	}
	delete(t.expires, expires)
	t.expireCh <- exp.maxIndex
}
