// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package leafcert

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// rootWatcher helps let multiple requests for leaf certs to coordinate sharing
// a single long-lived watch for the root certs. This allows the leaf cert
// requests to notice when the roots rotate and trigger their reissuance.
type rootWatcher struct {
	// This is the "top-level" internal context. This is used to cancel
	// background operations.
	ctx context.Context

	// rootsReader is an interface to access connect CA roots.
	rootsReader RootsReader

	// lock protects access to the subscribers map and cancel
	lock sync.Mutex
	// subscribers is a set of chans, one for each currently in-flight
	// Fetch. These chans have root updates delivered from the root watcher.
	subscribers map[chan struct{}]struct{}
	// cancel is a func to call to stop the background root watch if any.
	// You must hold lock to read (e.g. call) or write the value.
	cancel func()

	// testStart/StopCount are testing helpers that allow tests to
	// observe the reference counting behavior that governs the shared root watch.
	// It's not exactly pretty to expose internals like this, but seems cleaner
	// than constructing elaborate and brittle test cases that we can infer
	// correct behavior from, and simpler than trying to probe runtime goroutine
	// traces to infer correct behavior that way. They must be accessed
	// atomically.
	testStartCount uint32
	testStopCount  uint32
}

// Subscribe is called on each fetch that is about to block and wait for
// changes to the leaf. It subscribes a chan to receive updates from the shared
// root watcher and triggers root watcher if it's not already running.
func (r *rootWatcher) Subscribe(rootUpdateCh chan struct{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	// Lazy allocation
	if r.subscribers == nil {
		r.subscribers = make(map[chan struct{}]struct{})
	}
	// Make sure a root watcher is running. We don't only do this on first request
	// to be more tolerant of errors that could cause the root watcher to fail and
	// exit.
	if r.cancel == nil {
		ctx, cancel := context.WithCancel(r.ctx)
		r.cancel = cancel
		go r.rootWatcher(ctx)
	}
	r.subscribers[rootUpdateCh] = struct{}{}
}

// Unsubscribe is called when a blocking call exits to unsubscribe from root
// updates and possibly stop the shared root watcher if it's no longer needed.
// Note that typically root CA is still being watched by clients directly and
// probably by the ProxyConfigManager so it will stay hot in cache for a while,
// we are just not monitoring it for updates any more.
func (r *rootWatcher) Unsubscribe(rootUpdateCh chan struct{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.subscribers, rootUpdateCh)
	if len(r.subscribers) == 0 && r.cancel != nil {
		// This was the last request. Stop the root watcher.
		r.cancel()
		r.cancel = nil
	}
}

func (r *rootWatcher) notifySubscribers() {
	r.lock.Lock()
	defer r.lock.Unlock()

	for ch := range r.subscribers {
		select {
		case ch <- struct{}{}:
		default:
			// Don't block - chans are 1-buffered so this default case
			// means the subscriber already holds an update signal.
		}
	}
}

// rootWatcher is the shared rootWatcher that runs in a background goroutine
// while needed by one or more inflight Fetch calls.
func (r *rootWatcher) rootWatcher(ctx context.Context) {
	atomic.AddUint32(&r.testStartCount, 1)
	defer atomic.AddUint32(&r.testStopCount, 1)

	ch := make(chan cache.UpdateEvent, 1)

	if err := r.rootsReader.Notify(ctx, "roots", ch); err != nil {
		// Trigger all inflight watchers. We don't pass the error, but they will
		// reload from cache and observe the same error and return it to the caller,
		// or if it's transient, will continue and the next Fetch will get us back
		// into the right state. Seems better than busy loop-retrying here given
		// that almost any error we would see here would also be returned from the
		// cache get this will trigger.
		r.notifySubscribers()
		return
	}

	var oldRoots *structs.IndexedCARoots
	// Wait for updates to roots or all requests to stop
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			// Root response changed in some way. Note this might be the initial
			// fetch.
			if e.Err != nil {
				// See above rationale about the error propagation
				r.notifySubscribers()
				continue
			}

			roots, ok := e.Result.(*structs.IndexedCARoots)
			if !ok {
				// See above rationale about the error propagation
				r.notifySubscribers()
				continue
			}

			// Check that the active root is actually different from the last CA
			// config there are many reasons the config might have changed without
			// actually updating the CA root that is signing certs in the cluster.
			// The Fetch calls will also validate this since the first call here we
			// don't know if it changed or not, but there is no point waking up all
			// Fetch calls to check this if we know none of them will need to act on
			// this update.
			if oldRoots != nil && oldRoots.ActiveRootID == roots.ActiveRootID {
				continue
			}

			// Distribute the update to all inflight requests - they will decide
			// whether or not they need to act on it.
			r.notifySubscribers()
			oldRoots = roots
		}
	}
}
