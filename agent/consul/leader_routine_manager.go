package consul

import (
	"context"
	"os"
	"sync"

	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
)

type LeaderRoutine func(ctx context.Context) error

// cancelCh is the ctx.Done()
// When cancel() is called, if the routine is running a blocking call (e.g. some ACL replication RPCs),
//   stoppedCh won't be closed till the blocking call returns, while cancelCh will be closed immediately.
// cancelCh is used to properly detect routine running status between cancel() and close(stoppedCh)
type leaderRoutine struct {
	cancel    context.CancelFunc
	cancelCh  <-chan struct{} // closed when ctx is done
	stoppedCh chan struct{}   // closed when no longer running
}

func (r *leaderRoutine) running() bool {
	select {
	case <-r.stoppedCh:
		return false
	case <-r.cancelCh:
		return false
	default:
		return true
	}
}

type LeaderRoutineManager struct {
	lock   sync.RWMutex
	logger hclog.Logger

	routines map[string]*leaderRoutine
}

func NewLeaderRoutineManager(logger hclog.Logger) *LeaderRoutineManager {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Output: os.Stderr,
		})
	}

	return &LeaderRoutineManager{
		logger:   logger.Named(logging.Leader),
		routines: make(map[string]*leaderRoutine),
	}
}

func (m *LeaderRoutineManager) IsRunning(name string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if routine, ok := m.routines[name]; ok {
		return routine.running()
	}

	return false
}

func (m *LeaderRoutineManager) Start(name string, routine LeaderRoutine) error {
	return m.StartWithContext(context.TODO(), name, routine)
}

func (m *LeaderRoutineManager) StartWithContext(parentCtx context.Context, name string, routine LeaderRoutine) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if instance, ok := m.routines[name]; ok && instance.running() {
		return nil
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(parentCtx)
	instance := &leaderRoutine{
		cancel:    cancel,
		cancelCh:  ctx.Done(),
		stoppedCh: make(chan struct{}),
	}

	go func() {
		defer func() {
			close(instance.stoppedCh)
		}()

		err := routine(ctx)
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			m.logger.Error("routine exited with error",
				"routine", name,
				"error", err,
			)
		} else {
			m.logger.Info("stopped routine", "routine", name)
		}
	}()

	m.routines[name] = instance
	m.logger.Info("started routine", "routine", name)
	return nil
}

// Caveat: The returned stoppedCh indicates that the routine is completed
//         It's possible that ctx is canceled, but stoppedCh not yet closed
//         Use mgr.IsRunning(name) than this stoppedCh to tell whether the
//         instance is still running (not cancelled or completed).
func (m *LeaderRoutineManager) Stop(name string) <-chan struct{} {
	instance := m.stopInstance(name)
	if instance == nil {
		// Fabricate a closed channel so it won't block forever.
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return instance.stoppedCh
}

func (m *LeaderRoutineManager) stopInstance(name string) *leaderRoutine {
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

	m.logger.Info("stopping routine", "routine", name)
	instance.cancel()

	delete(m.routines, name)

	return instance
}

func (m *LeaderRoutineManager) StopAll() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for name, routine := range m.routines {
		if !routine.running() {
			continue
		}
		m.logger.Info("stopping routine", "routine", name)
		routine.cancel()
	}

	// just wipe out the entire map
	m.routines = make(map[string]*leaderRoutine)
}
