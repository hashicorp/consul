package config

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
	"os"
	"time"
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
	hash           string
	isEventWatched bool
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
	err := w.watcher.Add(filename)
	if err != nil {
		return err
	}
	hash, err := w.hashFile(filename)
	if err != nil {
		return err
	}
	w.configFiles[filename] = &watchedFile{hash: hash, isEventWatched: true}
	return nil
}

func (w Watcher) Remove(filename string) error {
	err := w.watcher.Remove(filename)
	if err != nil {
		return err
	}
	delete(w.configFiles, filename)
	return nil
}

func (w Watcher) Close() error {
	close(w.done)
	err := w.watcher.Close()
	if err != nil {
		return err
	}
	return nil
}

func (w Watcher) hashFile(filename string) (string, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()

	hash := hex.EncodeToString(hasher.Sum(file))
	return hash, nil
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
			err := w.handleEvent(event)
			if err != nil {
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
	if !isWrite(event) && !isRemove(event) && !isCreate(event) {
		return nil
	}
	// If the file was removed, set it to be readded to watch when created
	if isRemove(event) {
		w.configFiles[event.Name].isEventWatched = false
	}
	return w.handleFunc(&WatcherEvent{Filename: event.Name})
}

func (w Watcher) reconcile() {
	for filename, configFile := range w.configFiles {
		newHash, err := w.hashFile(filename)
		if err != nil {
			continue
		}

		if !configFile.isEventWatched {
			err = w.watcher.Add(filename)
			if err == nil {
				configFile.isEventWatched = true
			}
		}
		if configFile.hash != newHash {
			w.handleFunc(&WatcherEvent{Filename: filename})
		}
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
