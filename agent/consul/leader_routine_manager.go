package consul

import (
	"context"
	"log"
	"os"
	"sync"
)

type LeaderRoutine func(ctx context.Context) error

type leaderRoutine struct {
	running bool
	cancel  context.CancelFunc
}

type LeaderRoutineManager struct {
	lock   sync.RWMutex
	logger *log.Logger

	routines map[string]*leaderRoutine
}

func NewLeaderRoutineManager(logger *log.Logger) *LeaderRoutineManager {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	return &LeaderRoutineManager{
		logger:   logger,
		routines: make(map[string]*leaderRoutine),
	}
}

func (m *LeaderRoutineManager) IsRunning(name string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if routine, ok := m.routines[name]; ok {
		return routine.running
	}

	return false
}

func (m *LeaderRoutineManager) Start(name string, routine LeaderRoutine) error {
	return m.StartWithContext(nil, name, routine)
}

func (m *LeaderRoutineManager) StartWithContext(parentCtx context.Context, name string, routine LeaderRoutine) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if instance, ok := m.routines[name]; ok && instance.running {
		return nil
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(parentCtx)
	instance := &leaderRoutine{
		running: true,
		cancel:  cancel,
	}

	go func() {
		err := routine(ctx)
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			m.logger.Printf("[ERROR] leader: %s routine exited with error: %v", name, err)
		} else {
			m.logger.Printf("[DEBUG] leader: stopped %s routine", name)
		}

		m.lock.Lock()
		instance.running = false
		m.lock.Unlock()
	}()

	m.routines[name] = instance
	m.logger.Printf("[INFO] leader: started %s routine", name)
	return nil
}

func (m *LeaderRoutineManager) Stop(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	instance, ok := m.routines[name]
	if !ok {
		// no running instance
		return nil
	}

	if !instance.running {
		return nil
	}

	m.logger.Printf("[DEBUG] leader: stopping %s routine", name)
	instance.cancel()
	delete(m.routines, name)
	return nil
}

func (m *LeaderRoutineManager) StopAll() {
	m.lock.Lock()
	defer m.lock.Unlock()

	for name, routine := range m.routines {
		if !routine.running {
			continue
		}
		m.logger.Printf("[DEBUG] leader: stopping %s routine", name)
		routine.cancel()
	}

	// just whipe out the entire map
	m.routines = make(map[string]*leaderRoutine)
}
