package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
)

const timeoutDuration = 200 * time.Millisecond

type FileWatcher struct {
	watcher          *fsnotify.Watcher
	configFiles      map[string]*watchedFile
	handleFunc       func(event *WatcherEvent) error
	logger           hclog.Logger
	reconcileTimeout time.Duration
	cancel           context.CancelFunc
	done             chan interface{}
	toBeRemoved      chan string
	toBeAdded        chan string
}

type watchedFile struct {
	iNode   uint64
	watched bool
}

type WatcherEvent struct {
	Filename string
}

func NewFileWatcher(handleFunc func(event *WatcherEvent) error) (*FileWatcher, error) {
	ws, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	cfgFiles := make(map[string]*watchedFile)
	w := &FileWatcher{watcher: ws, configFiles: cfgFiles, handleFunc: handleFunc, logger: hclog.New(&hclog.LoggerOptions{}), reconcileTimeout: timeoutDuration, done: make(chan interface{}), toBeAdded: make(chan string), toBeRemoved: make(chan string), cancel: cancelFunc}

	go w.watch(ctx)
	return w, nil
}

func (w *FileWatcher) add(filename string) error {
	if err := w.watcher.Add(filename); err != nil {
		return err
	}
	iNode, err := w.getFileId(filename)
	if err != nil {
		return err
	}
	w.configFiles[filename] = &watchedFile{iNode: iNode, watched: true}
	return nil
}

func (w *FileWatcher) remove(filename string) error {
	if err := w.watcher.Remove(filename); err != nil {
		return err
	}
	delete(w.configFiles, filename)
	return nil
}
func (w *FileWatcher) Remove(filename string) error {
	timeout := time.After(timeoutDuration)
	select {
	case w.toBeRemoved <- filename:
		return nil
	case <-timeout:
		return fmt.Errorf("file remove timedout %s", filename)
	}
}

func (w *FileWatcher) Add(filename string) error {
	timeout := time.After(timeoutDuration)

	// explicitly do not support symlink as the behaviour is not consistent between OSs
	if isSymLink(filename) {
		return fmt.Errorf("symbolic link are not supported %s", filename)
	}
	select {
	case w.toBeAdded <- filename:
		return nil
	case <-timeout:
		return fmt.Errorf("file add timedout %s", filename)
	}
}

func isSymLink(filename string) bool {
	symlinks, err := os.Readlink(filename)
	if err == nil && symlinks != filename {
		return true
	}
	return false
}

func (w *FileWatcher) Close() error {
	w.cancel()
	<-w.done
	return w.watcher.Close()
}

func (w *FileWatcher) watch(ctx context.Context) {
	ticker := time.NewTicker(w.reconcileTimeout)
	defer ticker.Stop()
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
		case _, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Error("watcher error channel is closed")
				return
			}
		case <-ticker.C:
			w.reconcile()
		case filename := <-w.toBeAdded:
			err := w.add(filename)
			if err != nil {
				w.logger.Error("error adding a file", "file", filename, "err", err)
			}
		case filename := <-w.toBeRemoved:
			err := w.remove(filename)
			if err != nil {
				w.logger.Error("error removing a file", "file", filename, "err", err)
			}
		case <-ctx.Done():
			close(w.done)
			return
		}
	}

}

func (w *FileWatcher) handleEvent(event fsnotify.Event) error {
	w.logger.Debug("event received ", "filename", event.Name, "OP", event.Op)
	// we only want Create and Remove events to avoid triggering a relaod on file modification
	if !isCreate(event) && !isRemove(event) {
		return nil
	}
	configFile, basename, ok := w.isWatched(event.Name)
	if !ok {
		return fmt.Errorf("file %s is not watched", event.Name)
	}

	// we only want to update inode and re-add if the event is on the watched file itself
	if event.Name == basename {
		if isRemove(event) {
			// If the file was removed, set it to be re-added to watch when created
			err := w.watcher.Add(event.Name)
			if err != nil {
				configFile.watched = false
				configFile.iNode = 0
				return nil
			}
		}
	}
	if isCreate(event) {
		return w.handleFunc(&WatcherEvent{Filename: event.Name})
	}
	return nil
}

func (w *FileWatcher) isWatched(filename string) (*watchedFile, string, bool) {
	configFile, ok := w.configFiles[filename]
	if ok {
		return configFile, filename, true
	}
	path := filepath.Dir(filename)
	if path == filename || path == "" {
		return nil, "", false
	}
	return w.isWatched(path)
}

func (w *FileWatcher) reconcile() {
	for filename, configFile := range w.configFiles {
		newInode, err := w.getFileId(filename)
		if err != nil {
			w.logger.Error("failed to get file id", "file", filename, "err", err)
			continue
		}

		if !configFile.watched {
			if err := w.watcher.Add(filename); err != nil {
				w.logger.Error("failed to add file to watcher", "file", filename, "err", err)
				continue
			} else {
				configFile.watched = true
			}
		}

		if configFile.iNode != newInode {
			w.configFiles[filename].iNode = newInode
			err = w.handleFunc(&WatcherEvent{Filename: filename})
			if err != nil {
				w.logger.Error("event handle failed", "file", filename, "err", err)
			}
		}
	}
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func isRemove(event fsnotify.Event) bool {
	return event.Op&fsnotify.Remove == fsnotify.Remove
}
