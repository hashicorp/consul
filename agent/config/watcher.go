package config

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
)

const timeoutDuration = 10 * time.Second

type Watcher struct {
	watcher          *fsnotify.Watcher
	configFiles      map[string]*watchedFile
	handleFunc       func(event *WatcherEvent) error
	logger           hclog.Logger
	reconcileTimeout time.Duration
	done             chan interface{}
}

type watchedFile struct {
	iNode   uint64
	watched bool
}

type WatcherEvent struct {
	Filename string
}

func New(handleFunc func(event *WatcherEvent) error) (*Watcher, error) {
	ws, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cfgFiles := make(map[string]*watchedFile)
	return &Watcher{watcher: ws, configFiles: cfgFiles, handleFunc: handleFunc, logger: hclog.New(&hclog.LoggerOptions{}), reconcileTimeout: timeoutDuration, done: make(chan interface{})}, nil
}

func (w Watcher) Add(filename string) error {
	if err := w.watcher.Add(filename); err != nil {
		return err
	}
	iNode, err := w.getINode(filename)
	if err != nil {
		return err
	}
	w.configFiles[filename] = &watchedFile{iNode: iNode, watched: true}
	return nil
}

func (w Watcher) Remove(filename string) error {
	if err := w.watcher.Remove(filename); err != nil {
		return err
	}
	delete(w.configFiles, filename)
	return nil
}

func (w Watcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}

func (w Watcher) watch() {
	timer := time.NewTimer(w.reconcileTimeout)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Error("watcher event channel is closed")
				return
			}
			w.logger.Debug("received watcher event", "event", event)
			if err := w.handleEvent(event); err != nil {
				w.logger.Error("error handling watcher event", "error", err, "event", event)
			}
			timer.Reset(w.reconcileTimeout)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Error("watcher error channel is closed")
				return
			}
			timer.Reset(w.reconcileTimeout)
		case <-timer.C:
			w.reconcile()
			timer.Reset(w.reconcileTimeout)
		case <-w.done:
			return
		}
	}

}

func (w Watcher) handleEvent(event fsnotify.Event) error {
	// we only want Create and Remove events to avoid triggering a relaod on file modification
	if !isCreate(event) && !isRemove(event) {
		return nil
	}
	// If the file was removed, set it to be re-added to watch when created
	if isRemove(event) {
		w.configFiles[event.Name].watched = false
	}
	w.reconcileINodes()
	return w.handleFunc(&WatcherEvent{Filename: event.Name})
}

func (w Watcher) reconcile() {
	for filename, configFile := range w.configFiles {
		newInode, err := w.getINode(filename)
		if err != nil {
			continue
		}

		if !configFile.watched {
			if err := w.watcher.Add(filename); err != nil {
				continue
			} else {
				configFile.watched = true
			}
		}
		if configFile.iNode != newInode {
			w.reconcileINodes()
			w.handleFunc(&WatcherEvent{Filename: filename})
		}
	}
}

func (w Watcher) reconcileINodes() {
	for filename := range w.configFiles {
		iNode, err := w.getINode(filename)
		if err != nil {
			continue
		}
		w.configFiles[filename].iNode = iNode
	}
}

func (w Watcher) Start() {
	go w.watch()
}

func isWrite(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func isRemove(event fsnotify.Event) bool {
	return event.Op&fsnotify.Remove == fsnotify.Remove
}
