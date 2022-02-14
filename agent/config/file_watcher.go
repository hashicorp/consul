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
		reconcileTimeout: timeoutDuration,
		done:             make(chan interface{}),
	}
	w.logger = logger.Named("file-watcher")
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
	filename = filepath.Clean(filename)
	w.logger.Trace("adding file", "file", filename)
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

func (w *FileWatcher) Stop() error {
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
			w.logger.Info("received watcher event", "event", event)
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
	w.logger.Info("event received ", "filename", event.Name, "OP", event.Op)
	// we only want Create and Remove events to avoid triggering a relaod on file modification
	if !isCreate(event) && !isRemove(event) && !isWrite(event) && !isRename(event) {
		return nil
	}
	filename := filepath.Clean(event.Name)
	configFile, basename, ok := w.isWatched(filename)
	if !ok {
		return fmt.Errorf("file %s is not watched", event.Name)
	}

	// we only want to update inode and re-add if the event is on the watched file itself
	if filename == basename {
		if isRemove(event) {
			// If the file was removed, try to re-add it right away
			err := w.watcher.Add(filename)
			if err != nil {
				w.logger.Info("failed to re-add file, retry in reconcile ", "filename", event.Name, "OP", event.Op)
				//TODO(autoconfigreload): add debug log here.
				// re-add failed, set it to retry later in reconcile
				configFile.id = 0
				configFile.modId = 0
				return nil
			}
		}
	}
	if isCreate(event) || isWrite(event) || isRename(event) {
		go w.handleFunc(&WatcherEvent{Filename: filename})
	}
	return nil
}

func (w *FileWatcher) isWatched(filename string) (*watchedFile, string, bool) {
	path := filename
	configFile, ok := w.configFiles[path]
	if ok {
		return configFile, path, true
	}

	stat, err := os.Stat(filename)
	if err != nil {
		return nil, path, false
	}
	if !stat.IsDir() {
		w.logger.Trace("not a dir")
		// try to see if the watched path is the parent dir
		NewPath := filepath.Dir(path)
		w.logger.Trace("get dir", "dir", NewPath)
		configFile, ok = w.configFiles[NewPath]
	}
	return configFile, path, ok
}

func (w *FileWatcher) reconcile() {
	for filename, configFile := range w.configFiles {
		newId, err := w.getFileId(filename)
		if err != nil {
			w.logger.Error("failed to get file id", "file", filename, "err", err)
			continue
		}

		err = w.watcher.Add(filename)
		if err != nil {
			w.logger.Error("failed to add file to watcher", "file", filename, "err", err)
			continue
		}
		if configFile.id != newId {
			w.logger.Info("reconcile file", "filename", filename, "old id", configFile.id, "new id", newId)
			w.configFiles[filename].id = newId
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
