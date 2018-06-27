package watch

import (
	"fmt"
	"io"
	"sync"

	"github.com/miekg/dns"

	"github.com/coredns/coredns/pb"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
)

// Watcher handles watch creation, cancellation, and processing.
type Watcher interface {
	// Watch monitors a client stream and creates and cancels watches.
	Watch(pb.DnsService_WatchServer) error

	// Stop cancels open watches and stops the watch processing go routine.
	Stop()
}

// Manager contains all the data needed to manage watches
type Manager struct {
	changes Chan
	stopper chan bool
	counter int64
	watches map[string]watchlist
	plugins []Watchable
	mutex   sync.Mutex
}

type watchlist map[int64]pb.DnsService_WatchServer

// NewWatcher creates a Watcher, which is used to manage watched names.
func NewWatcher(plugins []Watchable) *Manager {
	w := &Manager{changes: make(Chan), stopper: make(chan bool), watches: make(map[string]watchlist), plugins: plugins}

	for _, p := range plugins {
		p.SetWatchChan(w.changes)
	}

	go w.process()
	return w
}

func (w *Manager) nextID() int64 {
	w.mutex.Lock()

	w.counter++
	id := w.counter

	w.mutex.Unlock()
	return id
}

// Watch monitors a client stream and creates and cancels watches.
func (w *Manager) Watch(stream pb.DnsService_WatchServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		create := in.GetCreateRequest()
		if create != nil {
			msg := new(dns.Msg)
			err := msg.Unpack(create.Query.Msg)
			if err != nil {
				log.Warningf("Could not decode watch request: %s\n", err)
				stream.Send(&pb.WatchResponse{Err: "could not decode request"})
				continue
			}
			id := w.nextID()
			if err := stream.Send(&pb.WatchResponse{WatchId: id, Created: true}); err != nil {
				// if we fail to notify client of watch creation, don't create the watch
				continue
			}

			// Normalize qname
			qname := (&request.Request{Req: msg}).Name()

			w.mutex.Lock()
			if _, ok := w.watches[qname]; !ok {
				w.watches[qname] = make(watchlist)
			}
			w.watches[qname][id] = stream
			w.mutex.Unlock()

			for _, p := range w.plugins {
				err := p.Watch(qname)
				if err != nil {
					log.Warningf("Failed to start watch for %s in plugin %s: %s\n", qname, p.Name(), err)
					stream.Send(&pb.WatchResponse{Err: fmt.Sprintf("failed to start watch for %s in plugin %s", qname, p.Name())})
				}
			}
			continue
		}

		cancel := in.GetCancelRequest()
		if cancel != nil {
			w.mutex.Lock()
			for qname, wl := range w.watches {
				ws, ok := wl[cancel.WatchId]
				if !ok {
					continue
				}

				// only allow cancels from the client that started it
				// TODO: test what happens if a stream tries to cancel a watchID that it doesn't own
				if ws != stream {
					continue
				}

				delete(wl, cancel.WatchId)

				// if there are no more watches for this qname, we should tell the plugins
				if len(wl) == 0 {
					for _, p := range w.plugins {
						p.StopWatching(qname)
					}
					delete(w.watches, qname)
				}

				// let the client know we canceled the watch
				stream.Send(&pb.WatchResponse{WatchId: cancel.WatchId, Canceled: true})
			}
			w.mutex.Unlock()
			continue
		}
	}
}

func (w *Manager) process() {
	for {
		select {
		case <-w.stopper:
			return
		case changed := <-w.changes:
			w.mutex.Lock()
			for qname, wl := range w.watches {
				if plugin.Zones([]string{changed}).Matches(qname) == "" {
					continue
				}
				for id, stream := range wl {
					wr := pb.WatchResponse{WatchId: id, Qname: qname}
					err := stream.Send(&wr)
					if err != nil {
						log.Warningf("Error sending change for %s to watch %d: %s. Removing watch.\n", qname, id, err)
						delete(w.watches[qname], id)
					}
				}
			}
			w.mutex.Unlock()
		}
	}
}

// Stop cancels open watches and stops the watch processing go routine.
func (w *Manager) Stop() {
	w.stopper <- true
	w.mutex.Lock()
	for wn, wl := range w.watches {
		for id, stream := range wl {
			wr := pb.WatchResponse{WatchId: id, Canceled: true}
			err := stream.Send(&wr)
			if err != nil {
				log.Warningf("Error notifiying client of cancellation: %s\n", err)
			}
		}
		delete(w.watches, wn)
	}
	w.mutex.Unlock()
}
