package kubernetes

import (
	"github.com/coredns/coredns/plugin/pkg/watch"
)

// SetWatchChan implements watch.Watchable
func (k *Kubernetes) SetWatchChan(c watch.Chan) {
	k.APIConn.SetWatchChan(c)
}

// Watch is called when a watch is started for a name.
func (k *Kubernetes) Watch(qname string) error {
	return k.APIConn.Watch(qname)
}

// StopWatching is called when no more watches remain for a name
func (k *Kubernetes) StopWatching(qname string) {
	k.APIConn.StopWatching(qname)
}
