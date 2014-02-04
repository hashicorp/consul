package consul

import (
	"sync"
)

// NotifyGroup is used to allow a simple notification mechanism.
// Channels can be marked as waiting, and when notify is invoked,
// all the waiting channels get a message and are cleared from the
// notify list.
type NotifyGroup struct {
	l      sync.Mutex
	notify []chan struct{}
}

// Notify will do a non-blocking send to all waiting channels, and
// clear the notify list
func (n *NotifyGroup) Notify() {
	n.l.Lock()
	defer n.l.Unlock()
	for _, ch := range n.notify {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	n.notify = n.notify[:0]
}

// Wait adds a channel to the notify group
func (n *NotifyGroup) Wait(ch chan struct{}) {
	n.l.Lock()
	defer n.l.Unlock()
	n.notify = append(n.notify, ch)
}

// WaitCh allocates a channel that is subscribed to notifications
func (n *NotifyGroup) WaitCh() chan struct{} {
	ch := make(chan struct{}, 1)
	n.Wait(ch)
	return ch
}
