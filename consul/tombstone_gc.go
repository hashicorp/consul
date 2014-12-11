package consul

import (
	"fmt"
	"sync"
	"time"
)

// TombstoneGC is used to track creation of tombstones
// so that they can be garbage collected after their TTL
// expires. The tombstones allow queries to provide monotonic
// index values within the TTL window. The GC is used to
// prevent monotonic growth in storage usage. This is a trade off
// between the length of the TTL and the storage overhead.
//
// In practice, this is required to fix the issue of delete
// visibility. When data is deleted from the KV store, the
// "latest" row can go backwards if the newest row is removed.
// The tombstones provide a way to ensure time doesn't move
// backwards within some interval.
//
type TombstoneGC struct {
	ttl         time.Duration
	granularity time.Duration

	// expires maps the time of expiration to the highest
	// tombstone value that should be expired.
	expires     map[time.Time]uint64
	expiresLock sync.Mutex

	// expireCh is used to stream expiration
	expireCh chan uint64
}

// NewTombstoneGC is used to construct a new TombstoneGC given
// a TTL for tombstones and a tracking granularity. Longer TTLs
// ensure correct behavior for more time, but use more storage.
// A shorter granularity increases the number of Raft transactions
// and reduce how far past the TTL we perform GC.
func NewTombstoneGC(ttl, granularity time.Duration) (*TombstoneGC, error) {
	// Sanity check the inputs
	if ttl <= 0 || granularity <= 0 {
		return nil, fmt.Errorf("Tombstone TTL and granularity must be positive")
	}

	t := &TombstoneGC{
		ttl:         ttl,
		granularity: granularity,
		expires:     make(map[time.Time]uint64),
		expireCh:    make(chan uint64, 1),
	}
	return t, nil
}

// ExpireCh is used to return a channel that streams the next index
// that should be expired
func (t *TombstoneGC) ExpireCh() <-chan uint64 {
	return t.expireCh
}

// Hint is used to indicate that keys at the given index have been
// deleted, and that their GC should be scheduled.
func (t *TombstoneGC) Hint(index uint64) {
	expires := t.nextExpires()

	t.expiresLock.Lock()
	defer t.expiresLock.Unlock()

	// Check for an existing expiration timer
	existing, ok := t.expires[expires]
	if ok {
		// Increment the highest index to be expired at that time
		if index > existing {
			t.expires[expires] = index
		}
		return
	}

	// Create new expiration time
	t.expires[expires] = index
	time.AfterFunc(expires.Sub(time.Now()), func() {
		t.expireTime(expires)
	})
}

// nextExpires is used to calculate the next experation time
func (t *TombstoneGC) nextExpires() time.Time {
	expires := time.Now().Add(t.ttl)
	remain := expires.UnixNano() % int64(t.granularity)
	adj := expires.Add(t.granularity - time.Duration(remain))
	return adj
}

// expireTime is used to expire the entries at the given time
func (t *TombstoneGC) expireTime(expires time.Time) {
	// Get the maximum index and clear the entry
	t.expiresLock.Lock()
	index := t.expires[expires]
	delete(t.expires, expires)
	t.expiresLock.Unlock()

	// Notify the expires channel
	t.expireCh <- index
}
