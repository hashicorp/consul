package memdb

import "time"

// WatchSet is a collection of watch channels.
type WatchSet map[<-chan struct{}]struct{}

// NewWatchSet constructs a new watch set.
func NewWatchSet() WatchSet {
	return make(map[<-chan struct{}]struct{})
}

// Add appends a watchCh to the WatchSet if non-nil.
func (w WatchSet) Add(watchCh <-chan struct{}) {
	if w == nil {
		return
	}

	if _, ok := w[watchCh]; !ok {
		w[watchCh] = struct{}{}
	}
}

// AddWithLimit appends a watchCh to the WatchSet if non-nil, and if the given
// softLimit hasn't been exceeded. Otherwise, it will watch the given alternate
// channel. It's expected that the altCh will be the same on many calls to this
// function, so you will exceed the soft limit a little bit if you hit this, but
// not by much.
//
// This is useful if you want to track individual items up to some limit, after
// which you watch a higher-level channel (usually a channel from start start of
// an iterator higher up in the radix tree) that will watch a superset of items.
func (w WatchSet) AddWithLimit(softLimit int, watchCh <-chan struct{}, altCh <-chan struct{}) {
	// This is safe for a nil WatchSet so we don't need to check that here.
	if len(w) < softLimit {
		w.Add(watchCh)
	} else {
		w.Add(altCh)
	}
}

// Watch is used to wait for either the watch set to trigger or a timeout.
// Returns true on timeout.
func (w WatchSet) Watch(timeoutCh <-chan time.Time) bool {
	if w == nil {
		return false
	}

	if n := len(w); n <= aFew {
		idx := 0
		chunk := make([]<-chan struct{}, aFew)
		for watchCh := range w {
			chunk[idx] = watchCh
			idx++
		}
		return watchFew(chunk, timeoutCh)
	} else {
		return w.watchMany(timeoutCh)
	}
}

// watchMany is used if there are many watchers.
func (w WatchSet) watchMany(timeoutCh <-chan time.Time) bool {
	// Make a fake timeout channel we can feed into watchFew to cancel all
	// the blocking goroutines.
	doneCh := make(chan time.Time)
	defer close(doneCh)

	// Set up a goroutine for each watcher.
	triggerCh := make(chan struct{}, 1)
	watcher := func(chunk []<-chan struct{}) {
		if timeout := watchFew(chunk, doneCh); !timeout {
			select {
			case triggerCh <- struct{}{}:
			default:
			}
		}
	}

	// Apportion the watch channels into chunks we can feed into the
	// watchFew helper.
	idx := 0
	chunk := make([]<-chan struct{}, aFew)
	for watchCh := range w {
		subIdx := idx % aFew
		chunk[subIdx] = watchCh
		idx++

		// Fire off this chunk and start a fresh one.
		if idx%aFew == 0 {
			go watcher(chunk)
			chunk = make([]<-chan struct{}, aFew)
		}
	}

	// Make sure to watch any residual channels in the last chunk.
	if idx%aFew != 0 {
		go watcher(chunk)
	}

	// Wait for a channel to trigger or timeout.
	select {
	case <-triggerCh:
		return false
	case <-timeoutCh:
		return true
	}
}
