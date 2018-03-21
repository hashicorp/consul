package proxy

import (
	"errors"
	"log"
	"os"
)

var (
	// ErrExists is the error returned when adding a proxy that exists already.
	ErrExists = errors.New("proxy with that name already exists")
	// ErrNotExist is the error returned when removing a proxy that doesn't exist.
	ErrNotExist = errors.New("proxy with that name doesn't exist")
)

// Manager implements the logic for configuring and running a set of proxiers.
// Typically it's used to run one PublicListener and zero or more Upstreams.
type Manager struct {
	ch chan managerCmd

	// stopped is used to signal the caller of StopAll when the run loop exits
	// after stopping all runners. It's only closed.
	stopped chan struct{}

	// runners holds the currently running instances. It should only by accessed
	// from within the `run` goroutine.
	runners map[string]*Runner

	logger *log.Logger
}

type managerCmd struct {
	name  string
	p     Proxier
	errCh chan error
}

// NewManager creates a manager of proxier instances.
func NewManager() *Manager {
	return NewManagerWithLogger(log.New(os.Stdout, "", log.LstdFlags))
}

// NewManagerWithLogger creates a manager of proxier instances with the
// specified logger.
func NewManagerWithLogger(logger *log.Logger) *Manager {
	m := &Manager{
		ch:      make(chan managerCmd),
		stopped: make(chan struct{}),
		runners: make(map[string]*Runner),
		logger:  logger,
	}
	go m.run()
	return m
}

// RunProxier starts a new Proxier instance in the manager. It is safe to call
// from separate goroutines. If there is already a running proxy with the same
// name it returns ErrExists.
func (m *Manager) RunProxier(name string, p Proxier) error {
	cmd := managerCmd{
		name:  name,
		p:     p,
		errCh: make(chan error),
	}
	m.ch <- cmd
	return <-cmd.errCh
}

// StopProxier stops a Proxier instance by name. It is safe to call from
// separate goroutines. If the instance with that name doesn't exist it returns
// ErrNotExist.
func (m *Manager) StopProxier(name string) error {
	cmd := managerCmd{
		name:  name,
		p:     nil,
		errCh: make(chan error),
	}
	m.ch <- cmd
	return <-cmd.errCh
}

// StopAll shuts down the manager instance and stops all running proxies. It is
// safe to call from any goroutine but must only be called once.
func (m *Manager) StopAll() error {
	close(m.ch)
	<-m.stopped
	return nil
}

// run is the main manager processing loop. It keeps all actions in a single
// goroutine triggered by channel commands to keep it simple to reason about
// lifecycle events for each proxy.
func (m *Manager) run() {
	defer close(m.stopped)

	// range over channel blocks and loops on each message received until channel
	// is closed.
	for cmd := range m.ch {
		if cmd.p == nil {
			m.remove(&cmd)
		} else {
			m.add(&cmd)
		}
	}

	// Shutting down, Stop all the runners
	for _, r := range m.runners {
		r.Stop()
	}
}

// add the named proxier instance and stop it. Should only be called from the
// run loop.
func (m *Manager) add(cmd *managerCmd) {
	// Check existing
	if _, ok := m.runners[cmd.name]; ok {
		cmd.errCh <- ErrExists
		return
	}

	// Start new runner
	r := NewRunnerWithLogger(cmd.name, cmd.p, m.logger)
	m.runners[cmd.name] = r
	go r.Listen()
	cmd.errCh <- nil
}

// remove the named proxier instance and stop it. Should only be called from the
// run loop.
func (m *Manager) remove(cmd *managerCmd) {
	// Fetch proxier by name
	r, ok := m.runners[cmd.name]
	if !ok {
		cmd.errCh <- ErrNotExist
		return
	}
	err := r.Stop()
	delete(m.runners, cmd.name)
	cmd.errCh <- err
}
