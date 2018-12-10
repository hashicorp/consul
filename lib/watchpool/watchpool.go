package watchpool

import (
	"context"
	"reflect"
	"sync"

	"github.com/hashicorp/go-memdb"
)

// WatchPool is a shared indirection for memdb.WatchSet. It allows multiple
// goroutines (e.g. blocking RPCs) to watch the same set of chans (i.e. radix
// tree nodes), with only a single WatchSet.Watch, which may spawn many
// goroutines for large sets.
//
// memdb.WatchSet can watch up to `aFew` (= 32 currently) chans with no
// additional goroutine overhead, but beyond that it spawns ceiling(N/32)
// goroutines. For a watch set containing 2048 radix nodes (e.g. ~682 service
// instances with checks and nodes) that means each blocking query is running 64
// goroutines on top of the one actually serving the request. With hundreds or
// thousands of clients watching the same things, this quickly adds up and
// places a lot of load on the server. See
// https://github.com/hashicorp/consul/issues/4984 for more info.
//
// By allowing different blocking queries that are watching the same radix nodes
// to share a WatchSet.Watch, we can limit that number so in the example above,
// one client still uses 64 goroutines to watch, but now 1000 clients can all be
// blocking on that same query and still only 64 goroutines are needed (actually
// 65 since we add a constant one more here per WatchSet).
//
// In the case where only a single RPC is blocked on a specific set of radix
// nodes, we add no extra goroutines of overhead - the "leader" goroutine
// processes the Watch just as before inline, if it times out with no change
// observed, then it passes the leadership to one other watcher if any have come
// along.
//
// To use it, just build your memdb.WatchSet like normal, but instead of calling
// Watch or WatchCtx directly, call `pool.Watch(ctx, ws)`.
type WatchPool struct {
	l         sync.Mutex
	watchSets map[uint64]*wsEntry
}

type wsEntry struct {
	ws memdb.WatchSet
	// done is closed by the leader Watch when any chan in the WatchSet is closed.
	done chan struct{}
	// leaderDone is NOT closed, but has a message sent on it if the current
	// leader times out and returns to it's caller. If there are "follower" Watch
	// calls, one of them will receive the message and take over being the leader.
	leaderDone chan struct{}
	// refs is a reference count for how many outstanding Watch calls there are
	// for this WatchSet. It is incremented and decremented atomically.
	refs uint32
}

// Watch performs a shared watch on a memdb.WatchSet. It will deduplicate calls
// with the same WatchSets (i.e. exactly the same nodes in the state store being
// watched) and run only one ws.WatchCtx call regardless of the number of
// concurrent callers.
func (p *WatchPool) Watch(ctx context.Context, ws memdb.WatchSet) error {
	key := p.wsKey(ws)

	// Whatever happens, we need to come back and decrement the ref count and
	// possibly clean up at the end.
	defer p.watcherDone(key)

	p.l.Lock()

	// Lazy initialize the map
	if p.watchSets == nil {
		p.watchSets = make(map[uint64]*wsEntry)
	}

	// See if the key already exists in the share map
	entry, ok := p.watchSets[key]
	if !ok {
		// Create a new entry
		entry = &wsEntry{
			ws:   ws,
			done: make(chan struct{}),
			// Buffer in leaderChan is important to not block leader if it times out
			// with no followers. We rely on it to not deadlock the initial call too.
			leaderDone: make(chan struct{}, 1),
			refs:       1,
		}
		p.watchSets[key] = entry

		// Trigger one of the Watch goroutines for this set to become the initial
		// leader. It will likely be us in the select below but it could be another
		// concurrent call that races. Note that if the select below takes a
		// non-leader case because our context is cancelled for example, then either
		// another Watcher will become leader instead, or all watchers will return
		// for another reason and the state will be cleaned up by watcherDone
		// preserving invariant that there is always one leader Watch for a given
		// entry in the map. This won't block because we buffer this chan.
		entry.leaderDone <- struct{}{}
	} else {
		// Add ourselves to the ref count
		entry.refs++
	}
	// Unlock now before processing any more
	p.l.Unlock()

	// We are a follower
	select {
	case <-ctx.Done():
		// Our context is done, just return it's error.
		return ctx.Err()

	case <-entry.done:
		// The leader notified of a change in the watchset.
		return nil

	case <-entry.leaderDone:
		// The current leader timed out (or we just setup the state above) and we
		// won the race to become next leader. We need to watch until our context is
		// done
		watchErr := entry.ws.WatchCtx(ctx)
		if watchErr == context.Canceled || watchErr == context.DeadlineExceeded {
			// We timed out, pass the leader buck on to someone else (if there are any
			// followers, if not this is harmless since it's a buffered chan).
			entry.leaderDone <- struct{}{}
		} else {
			// An actual change happened, in this case notify followers and we can all
			// be done so no need to pass on leadership.
			close(entry.done)
		}
		return watchErr
	}
}

func (p *WatchPool) watcherDone(key uint64) {
	p.l.Lock()
	defer p.l.Unlock()

	entry, ok := p.watchSets[key]
	if !ok || entry.refs < 1 {
		// Shouldn't happen!
		return
	}
	// Un-count watcher and possibly cleanup state.
	entry.refs--
	if entry.refs == 0 {
		// Just remove the state. Any new watches will start a whole new leader etc.
		delete(p.watchSets, key)
	}
}

func (p *WatchPool) wsKey(ws memdb.WatchSet) uint64 {
	// Hash the map. hashstructure doesn't know how to hash a chan although
	// reflect can do it without technically using anything unsafe via uintptr.
	// Since memory addresses are unique already and we need them unordered since
	// this is a set and map order is non-determinisic, just use the address as
	// the hash an XOR them.
	var hash uint64
	for ch := range ws {
		val := reflect.ValueOf(ch)
		// Write the chan's address (!) into our buffer
		// XOR the chan's address with our current hash
		hash ^= uint64(val.Pointer())
	}
	return hash
}
