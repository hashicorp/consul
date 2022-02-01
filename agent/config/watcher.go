package config

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/fsnotify/fsnotify"
	"os"
	"time"
)

type Watcher struct {
	watcher     *fsnotify.Watcher
	configFiles map[string]string
	reloadFunc  func() error
}

func New(reloadFunc func() error) (*Watcher, error) {
	ws, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cfgFiles := make(map[string]string)
	return &Watcher{watcher: ws, configFiles: cfgFiles, reloadFunc: reloadFunc}, nil
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
	w.configFiles[filename] = hash
	return nil
}

func (w Watcher) hashFile(filename string) (string, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()

	toString := hex.EncodeToString(hasher.Sum(file))
	return toString, nil
}

func (w Watcher) watch() {
	const timeoutDuration = 10 * time.Second
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				// log not ok (channel closed)
			}
			err := w.handleEvent(event)
			if err != nil {
				// log error
			}
			timer.Reset(timeoutDuration)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				// log not ok (error channel closed)
			}
			timer.Reset(timeoutDuration)
		case <-timer.C:
			w.reconcile()
			timer.Reset(timeoutDuration)
		}
	}

}

func (w Watcher) handleEvent(event fsnotify.Event) error {
	if !isWrite(event) || !isRemove(event) || !isCreate(event) {
		return nil
	}
	// If the file was removed, re-add the watch.
	if isRemove(event) {
		if err := w.watcher.Add(event.Name); err != nil {
			//log.Error(err, "error re-watching file")
		}
	}
	return w.reloadFunc()
}

func (w Watcher) reconcile() {
	for filename, hash := range w.configFiles {
		newHash, err := w.hashFile(filename)
		if err != nil {
			continue
		}
		if hash != newHash {
			w.reloadFunc()
			return
		}
	}
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
