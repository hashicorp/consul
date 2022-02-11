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
	handleFunc       func(event *WatcherEvent)
	logger           hclog.Logger
	reconcileTimeout time.Duration
	cancel           context.CancelFunc
	done             chan interface{}
}

type watchedFile struct {
	id    uint64
	modId uint64
}

type WatcherEvent struct {
	Filename string
}

func NewFileWatcher(handleFunc func(event *WatcherEvent), configFiles []string, logger hclog.Logger) (*FileWatcher, error) {
	ws, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	cfgFiles := make(map[string]*watchedFile)

	w := &FileWatcher{watcher: ws,
		configFiles:      cfgFiles,
		handleFunc:       handleFunc,
		logger:           logger,
		reconcileTimeout: timeoutDuration,
		done:             make(chan interface{}),
	}
	for _, f := range configFiles {
		err = w.add(f)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (w *FileWatcher) Start(ctx context.Context) {
	cancelCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	go w.watch(cancelCtx)
}

func (w *FileWatcher) add(filename string) error {
	if isSymLink(filename) {
		return fmt.Errorf("symbolic link are not supported %s", filename)
	}
	if err := w.watcher.Add(filename); err != nil {
		return err
	}
	iNode, err := w.getFileId(filename)
	if err != nil {
		return err
	}
	w.configFiles[filename] = &watchedFile{id: iNode}
	return nil
}

func isSymLink(filename string) bool {
	fi, err := os.Lstat(filename)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink != 0 {
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
		case <-ctx.Done():
			close(w.done)
			return
		}
	}

}

func (w *FileWatcher) handleEvent(event fsnotify.Event) error {
	w.logger.Debug("event received ", "filename", event.Name, "OP", event.Op)
	// we only want Create and Remove events to avoid triggering a relaod on file modification
	if !isCreate(event) && !isRemove(event) && !isWrite(event) && !isRename(event) {
		return nil
	}
	configFile, basename, ok := w.isWatched(event.Name)
	if !ok {
		return fmt.Errorf("file %s is not watched", event.Name)
	}

	// we only want to update inode and re-add if the event is on the watched file itself
	if event.Name == basename {
		if isRemove(event) {
			// If the file was removed, try to re-add it right away
			err := w.watcher.Add(event.Name)
			if err != nil {
				// re-add failed, set it to retry later in reconcile
				configFile.id = 0
				configFile.modId = 0
				return nil
			}
		}
	}
	if isCreate(event) || isWrite(event) || isRename(event) {
		go w.handleFunc(&WatcherEvent{Filename: event.Name})
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

		err = w.watcher.Add(filename)
		if err != nil {
			w.logger.Error("failed to add file to watcher", "file", filename, "err", err)
			continue
		}
		if configFile.id != newInode {
			w.configFiles[filename].id = newInode
			go w.handleFunc(&WatcherEvent{Filename: filename})
		}
	}
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func isRemove(event fsnotify.Event) bool {
	return event.Op&fsnotify.Remove == fsnotify.Remove
}

func isWrite(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write
}

func isRename(event fsnotify.Event) bool {
	return event.Op&fsnotify.Rename == fsnotify.Rename
}

func (w *FileWatcher) getFileId(filename string) (uint64, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}

	return uint64(fileInfo.ModTime().Nanosecond()), err
}
