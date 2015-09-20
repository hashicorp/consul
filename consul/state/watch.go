package state

import (
	"github.com/hashicorp/go-memdb"
)

type WatchManager interface {
	Start(notifyCh chan struct{})
	Stop(notifyCh chan struct{})
	Notify()
}

type FullTableWatch struct {
	notify NotifyGroup
}

func (w *FullTableWatch) Start(notifyCh chan struct{}) {
	w.notify.Wait(notifyCh)
}

func (w *FullTableWatch) Stop(notifyCh chan struct{}) {
	w.notify.Clear(notifyCh)
}

func (w *FullTableWatch) Notify() {
	w.notify.Notify()
}

func newWatchManagers(schema *memdb.DBSchema) (map[string]WatchManager, error) {
	watches := make(map[string]WatchManager)
	for table, _ := range schema.Tables {
		watches[table] = &FullTableWatch{}
	}
	return watches, nil
}
