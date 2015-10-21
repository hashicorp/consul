package state

import (
	"fmt"
	"sync"

	"github.com/armon/go-radix"
)

// Watch is the external interface that's common to all the different flavors.
type Watch interface {
	// Wait registers the given channel and calls it back when the watch
	// fires.
	Wait(notifyCh chan struct{})

	// Clear deregisters the given channel.
	Clear(notifyCh chan struct{})
}

// FullTableWatch implements a single notify group for a table.
type FullTableWatch struct {
	group NotifyGroup
}

// NewFullTableWatch returns a new full table watch.
func NewFullTableWatch() *FullTableWatch {
	return &FullTableWatch{}
}

// See Watch.
func (w *FullTableWatch) Wait(notifyCh chan struct{}) {
	w.group.Wait(notifyCh)
}

// See Watch.
func (w *FullTableWatch) Clear(notifyCh chan struct{}) {
	w.group.Clear(notifyCh)
}

// Notify wakes up all the watchers registered for this table.
func (w *FullTableWatch) Notify() {
	w.group.Notify()
}

// DumbWatchManager is a wrapper that allows nested code to arm full table
// watches multiple times but fire them only once. This doesn't have any
// way to clear the state, and it's not thread-safe, so it should be used once
// and thrown away inside the context of a single thread.
type DumbWatchManager struct {
	// tableWatches holds the full table watches.
	tableWatches map[string]*FullTableWatch

	// armed tracks whether the table should be notified.
	armed map[string]bool
}

// NewDumbWatchManager returns a new dumb watch manager.
func NewDumbWatchManager(tableWatches map[string]*FullTableWatch) *DumbWatchManager {
	return &DumbWatchManager{
		tableWatches: tableWatches,
		armed:        make(map[string]bool),
	}
}

// Arm arms the given table's watch.
func (d *DumbWatchManager) Arm(table string) {
	if _, ok := d.tableWatches[table]; !ok {
		panic(fmt.Sprintf("unknown table: %s", table))
	}

	if _, ok := d.armed[table]; !ok {
		d.armed[table] = true
	}
}

// Notify fires watches for all the armed tables.
func (d *DumbWatchManager) Notify() {
	for table, _ := range d.armed {
		d.tableWatches[table].Notify()
	}
}

// PrefixWatch maintains a notify group for each prefix, allowing for much more
// fine-grained watches.
type PrefixWatch struct {
	// watches has the set of notify groups, organized by prefix.
	watches *radix.Tree

	// lock protects the watches tree.
	lock sync.Mutex
}

// NewPrefixWatch returns a new prefix watch.
func NewPrefixWatch() *PrefixWatch {
	return &PrefixWatch{
		watches: radix.New(),
	}
}

// GetSubwatch returns the notify group for the given prefix.
func (w *PrefixWatch) GetSubwatch(prefix string) *NotifyGroup {
	w.lock.Lock()
	defer w.lock.Unlock()

	if raw, ok := w.watches.Get(prefix); ok {
		return raw.(*NotifyGroup)
	}

	group := &NotifyGroup{}
	w.watches.Insert(prefix, group)
	return group
}

// Notify wakes up all the watchers associated with the given prefix. If subtree
// is true then we will also notify all the tree under the prefix, such as when
// a key is being deleted.
func (w *PrefixWatch) Notify(prefix string, subtree bool) {
	w.lock.Lock()
	defer w.lock.Unlock()

	var cleanup []string
	fn := func(k string, v interface{}) bool {
		group := v.(*NotifyGroup)
		group.Notify()
		if k != "" {
			cleanup = append(cleanup, k)
		}
		return false
	}

	// Invoke any watcher on the path downward to the key.
	w.watches.WalkPath(prefix, fn)

	// If the entire prefix may be affected (e.g. delete tree),
	// invoke the entire prefix.
	if subtree {
		w.watches.WalkPrefix(prefix, fn)
	}

	// Delete the old notify groups.
	for i := len(cleanup) - 1; i >= 0; i-- {
		w.watches.Delete(cleanup[i])
	}

	// TODO (slackpad) If a watch never fires then we will never clear it
	// out of the tree. The old state store had the same behavior, so this
	// has been around for a while. We should probably add a prefix scan
	// with a function that clears out any notify groups that are empty.
}

// MultiWatch wraps several watches and allows any of them to trigger the
// caller.
type MultiWatch struct {
	// watches holds the list of subordinate watches to forward events to.
	watches []Watch
}

// NewMultiWatch returns a new new multi watch over the given set of watches.
func NewMultiWatch(watches ...Watch) *MultiWatch {
	return &MultiWatch{
		watches: watches,
	}
}

// See Watch.
func (w *MultiWatch) Wait(notifyCh chan struct{}) {
	for _, watch := range w.watches {
		watch.Wait(notifyCh)
	}
}

// See Watch.
func (w *MultiWatch) Clear(notifyCh chan struct{}) {
	for _, watch := range w.watches {
		watch.Clear(notifyCh)
	}
}
