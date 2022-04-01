package config

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
)

type rateLimitedFileWatcher struct {
	watcher          Watcher
	eventCh          chan *FileWatcherEvent
	coalesceInterval time.Duration
}

func (r rateLimitedFileWatcher) Start(ctx context.Context) {
	r.watcher.Start(ctx)
	r.coalesceTimer(r.watcher.EventsCh(), r.coalesceInterval)
}

func (r rateLimitedFileWatcher) Stop() error {
	return r.watcher.Stop()
}

func (r rateLimitedFileWatcher) Add(filename string) error {
	return r.watcher.Add(filename)
}

func (r rateLimitedFileWatcher) Remove(filename string) {
	r.watcher.Remove(filename)
}

func (r rateLimitedFileWatcher) Replace(oldFile, newFile string) error {
	return r.watcher.Replace(oldFile, newFile)
}

func (r rateLimitedFileWatcher) EventsCh() chan *FileWatcherEvent {
	return r.eventCh
}

func NewRateLimitedFileWatcher(configFiles []string, logger hclog.Logger, coalesceInterval time.Duration) (Watcher, error) {

	watcher, err := NewFileWatcher(configFiles, logger)
	if err != nil {
		return nil, err
	}
	return rateLimitedFileWatcher{
		watcher:          watcher,
		coalesceInterval: coalesceInterval,
		eventCh:          make(chan *FileWatcherEvent),
	}, nil
}

func (r rateLimitedFileWatcher) coalesceTimer(inputCh chan *FileWatcherEvent, coalesceDuration time.Duration) {
	var coalesceTimer *time.Timer = nil
	sendCh := make(chan struct{})
	FileWatcherEvents := make([]string, 0)
	go func() {
		for {
			select {
			case event, ok := <-inputCh:
				if !ok {
					if len(FileWatcherEvents) > 0 {
						r.eventCh <- &FileWatcherEvent{Filenames: FileWatcherEvents}
					}
					close(r.eventCh)
					return
				}
				FileWatcherEvents = append(FileWatcherEvents, event.Filenames...)
				if coalesceTimer == nil {
					coalesceTimer = time.AfterFunc(coalesceDuration, func() {
						// This runs in another goroutine so we can't just do the send
						// directly here as access to snap is racy. Instead, signal the main
						// loop above.
						sendCh <- struct{}{}
					})
				}
			case <-sendCh:
				coalesceTimer = nil
				r.eventCh <- &FileWatcherEvent{Filenames: FileWatcherEvents}
				FileWatcherEvents = make([]string, 0)
			}
		}
	}()
}
