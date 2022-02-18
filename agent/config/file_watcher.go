package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
)

const timeoutDuration = 200 * time.Millisecond

type FileWatcher struct {
	watcher          *fsnotify.Watcher
	configFiles      map[string]*watchedFile
	logger           hclog.Logger
	reconcileTimeout time.Duration
	cancel           context.CancelFunc
	done             chan interface{}
	stopOnce         sync.Once

	//EventsCh Channel where an event will be emitted when a file change is detected
	// a call to Start is needed before any event is emitted
	// after a Call to Stop succeed, the channel will be closed
	EventsCh chan *FileWatcherEvent
}

type watchedFile struct {
	modTime time.Time
}

type FileWatcherEvent struct {
	Filename string
}

//NewFileWatcher create a file watcher that will watch all the files/folders from configFiles
// if success a FileWatcher will be returned and a nil error
// otherwise an error and a nil FileWatcher are returned
func NewFileWatcher(configFiles []string, logger hclog.Logger) (*FileWatcher, error) {
	ws, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &FileWatcher{
		watcher:          ws,
		logger:           logger.Named("file-watcher"),
		configFiles:      make(map[string]*watchedFile),
		EventsCh:         make(chan *FileWatcherEvent),
		reconcileTimeout: timeoutDuration,
		done:             make(chan interface{}),
		stopOnce:         sync.Once{},
	}
	for _, f := range configFiles {
		err = w.add(f)
		if err != nil {
			return nil, fmt.Errorf("error adding file %q: %w", f, err)
		}
	}

	return w, nil
}

// Start start a file watcher, with a copy of the passed context.
// calling Start multiple times is a noop
func (w *FileWatcher) Start(ctx context.Context) {
	if w.cancel == nil {
		cancelCtx, cancel := context.WithCancel(ctx)
		w.cancel = cancel
		go w.watch(cancelCtx)
	}
}

// Stop the file watcher
// calling Stop multiple times is a noop, Stop must be called after a Start
func (w *FileWatcher) Stop() error {
	var err error
	w.stopOnce.Do(func() {
		w.cancel()
		<-w.done
		close(w.EventsCh)
		err = w.watcher.Close()
	})
	return err
}

func (w *FileWatcher) add(filename string) error {
	if isSymLink(filename) {
		return fmt.Errorf("symbolic links are not supported %s", filename)
	}
	filename = filepath.Clean(filename)
	w.logger.Trace("adding file", "file", filename)
	if err := w.watcher.Add(filename); err != nil {
		return err
	}
	modTime, err := w.getFileModifiedTime(filename)
	if err != nil {
		return err
	}
	w.configFiles[filename] = &watchedFile{modTime: modTime}
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

func (w *FileWatcher) watch(ctx context.Context) {
	ticker := time.NewTicker(w.reconcileTimeout)
	defer ticker.Stop()
	defer close(w.done)

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Error("watcher event channel is closed")
				return
			}
			w.logger.Trace("received watcher event", "event", event)
			if err := w.handleEvent(ctx, event); err != nil {
				w.logger.Error("error handling watcher event", "error", err, "event", event)
			}
		case _, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Error("watcher error channel is closed")
				return
			}
		case <-ticker.C:
			w.reconcile(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (w *FileWatcher) handleEvent(ctx context.Context, event fsnotify.Event) error {
	w.logger.Trace("event received ", "filename", event.Name, "OP", event.Op)
	// we only want Create and Remove events to avoid triggering a reload on file modification
	if !isCreateEvent(event) && !isRemoveEvent(event) && !isWriteEvent(event) && !isRenameEvent(event) {
		return nil
	}
	filename := filepath.Clean(event.Name)
	configFile, basename, ok := w.isWatched(filename)
	if !ok {
		return fmt.Errorf("file %s is not watched", event.Name)
	}

	// we only want to update mod time and re-add if the event is on the watched file itself
	if filename == basename {
		if isRemoveEvent(event) {
			// If the file was removed, try to reconcile and see if anything changed.
			w.logger.Trace("attempt a reconcile ", "filename", event.Name, "OP", event.Op)
			configFile.modTime = time.Time{}
			w.reconcile(ctx)
		}
	}
	if isCreateEvent(event) || isWriteEvent(event) || isRenameEvent(event) {
		w.logger.Trace("call the handler", "filename", event.Name, "OP", event.Op)
		select {
		case w.EventsCh <- &FileWatcherEvent{Filename: filename}:
		case <-ctx.Done():
			return ctx.Err()
		}

	}
	return nil
}

func (w *FileWatcher) isWatched(filename string) (*watchedFile, string, bool) {
	path := filename
	configFile, ok := w.configFiles[path]
	if ok {
		return configFile, path, true
	}

	stat, err := os.Lstat(filename)

	// if the error is a not exist still try to find if the event for a configured file
	if os.IsNotExist(err) || (!stat.IsDir() && stat.Mode()&os.ModeSymlink == 0) {
		w.logger.Trace("not a dir and not a symlink to a dir")
		// try to see if the watched path is the parent dir
		newPath := filepath.Dir(path)
		w.logger.Trace("get dir", "dir", newPath)
		configFile, ok = w.configFiles[newPath]
	}
	return configFile, path, ok
}

func (w *FileWatcher) reconcile(ctx context.Context) {
	for filename, configFile := range w.configFiles {
		w.logger.Trace("reconciling", "filename", filename)
		newModTime, err := w.getFileModifiedTime(filename)
		if err != nil {
			w.logger.Error("failed to get file modTime", "file", filename, "err", err)
			continue
		}

		err = w.watcher.Add(filename)
		if err != nil {
			w.logger.Error("failed to add file to watcher", "file", filename, "err", err)
			continue
		}
		if !configFile.modTime.Equal(newModTime) {
			w.logger.Trace("call the handler", "filename", filename, "old modTime", configFile.modTime, "new modTime", newModTime)
			w.configFiles[filename].modTime = newModTime
			select {
			case w.EventsCh <- &FileWatcherEvent{Filename: filename}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func isCreateEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func isRemoveEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Remove == fsnotify.Remove
}

func isWriteEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write
}

func isRenameEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Rename == fsnotify.Rename
}

func (w *FileWatcher) getFileModifiedTime(filename string) (time.Time, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return time.Time{}, err
	}

	return fileInfo.ModTime(), err
}
