package routine

import (
	"context"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
)

type Routine func(ctx context.Context) error

type routineTracker struct {
	cancel    context.CancelFunc
	stoppedCh chan struct{} // closed when no longer running
}

func (r *routineTracker) running() bool {
	select {
	case <-r.stoppedCh:
		return false
	default:
		return true
	}
}

func (r *routineTracker) wait() {
	<-r.stoppedCh
}

type Manager struct {
	lock   sync.RWMutex
	logger hclog.Logger

	routines map[string]*routineTracker
}

func NewManager(logger hclog.Logger) *Manager {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Output: os.Stderr,
		})
	}

	return &Manager{
		logger:   logger,
		routines: make(map[string]*routineTracker),
	}
}

func (m *Manager) IsRunning(name string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if routine, ok := m.routines[name]; ok {
		return routine.running()
	}

	return false
}

func (m *Manager) Start(ctx context.Context, name string, routine Routine) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if instance, ok := m.routines[name]; ok && instance.running() {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	rtCtx, cancel := context.WithCancel(ctx)
	instance := &routineTracker{
		cancel:    cancel,
		stoppedCh: make(chan struct{}),
	}

	go m.execute(rtCtx, name, routine, instance.stoppedCh)

	m.routines[name] = instance
	m.logger.Info("started routine", "routine", name)
	return nil
}

// execute will run the given routine in the foreground and close the given channel when its done executing
func (m *Manager) execute(ctx context.Context, name string, routine Routine, done chan struct{}) {
	defer func() {
		close(done)
	}()

	err := routine(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		m.logger.Error("routine exited with error",
			"routine", name,
			"error", err,
		)
	} else {
		m.logger.Debug("stopped routine", "routine", name)
	}
}

func (m *Manager) Stop(name string) <-chan struct{} {
	instance := m.stopInstance(name)
	if instance == nil {
		// Fabricate a closed channel so it won't block forever.
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return instance.stoppedCh
}

func (m *Manager) stopInstance(name string) *routineTracker {
	m.lock.Lock()
	defer m.lock.Unlock()

	instance, ok := m.routines[name]
	if !ok {
		// no running instance
		return nil
	}

	if !instance.running() {
		return instance
	}

	m.logger.Debug("stopping routine", "routine", name)
	instance.cancel()

	delete(m.routines, name)

	return instance
}

// StopAll goroutines. Once StopAll is called, it is no longer safe to add no
// goroutines to the Manager.
func (m *Manager) StopAll() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for name, routine := range m.routines {
		if !routine.running() {
			continue
		}
		m.logger.Debug("stopping routine", "routine", name)
		routine.cancel()
	}
}

// Wait for all goroutines to stop after StopAll is called.
func (m *Manager) Wait() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, routine := range m.routines {
		routine.wait()
	}
}
