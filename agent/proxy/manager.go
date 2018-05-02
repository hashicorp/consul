package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-multierror"
)

// Manager starts, stops, snapshots, and restores managed proxies.
//
// The manager will not start or stop any processes until Start is called.
// Prior to this, any configuration, snapshot loading, etc. can be done.
// Even if a process is no longer running after loading the snapshot, it
// will not be restarted until Start is called.
//
// The Manager works by subscribing to change notifications on a local.State
// structure. Whenever a change is detected, the Manager syncs its internal
// state with the local.State and starts/stops any necessary proxies. The
// manager never holds a lock on local.State (except to read the proxies)
// and state updates may occur while the Manger is syncing. This is okay,
// since a change notification will be queued to trigger another sync.
//
// NOTE(mitchellh): Change notifications are not coalesced currently. Under
// conditions where managed proxy configurations are changing in a hot
// loop, it is possible for the manager to constantly attempt to sync. This
// is unlikely, but its also easy to introduce basic coalescing (even over
// millisecond intervals) to prevent total waste compute cycles.
type Manager struct {
	// State is the local state that is the source of truth for all
	// configured managed proxies.
	State *local.State

	// Logger is the logger for information about manager behavior.
	// Output for proxies will not go here generally but varies by proxy
	// implementation type.
	Logger *log.Logger

	// lock is held while reading/writing any internal state of the manager.
	// cond is a condition variable on lock that is broadcasted for runState
	// changes.
	lock *sync.Mutex
	cond *sync.Cond

	// runState is the current state of the manager. To read this the
	// lock must be held. The condition variable cond can be waited on
	// for changes to this value.
	runState managerRunState

	proxies map[string]Proxy
}

// defaultLogger is the defaultLogger for NewManager so there it is never nil
var defaultLogger = log.New(os.Stderr, "", log.LstdFlags)

// NewManager initializes a Manager. After initialization, the exported
// fields should be configured as desired. To start the Manager, execute
// Run in a goroutine.
func NewManager() *Manager {
	var lock sync.Mutex
	return &Manager{
		Logger:  defaultLogger,
		lock:    &lock,
		cond:    sync.NewCond(&lock),
		proxies: make(map[string]Proxy),
	}
}

// managerRunState is the state of the Manager.
//
// This is a basic state machine with the following transitions:
//
//   * idle     => running, stopped
//   * running  => stopping, stopped
//   * stopping => stopped
//   * stopped  => <>
//
type managerRunState uint8

const (
	managerStateIdle managerRunState = iota
	managerStateRunning
	managerStateStopping
	managerStateStopped
)

// Close stops the manager. Managed processes are NOT stopped.
func (m *Manager) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for {
		switch m.runState {
		case managerStateIdle:
			// Idle so just set it to stopped and return. We notify
			// the condition variable in case others are waiting.
			m.runState = managerStateStopped
			m.cond.Broadcast()
			return nil

		case managerStateRunning:
			// Set the state to stopping and broadcast to all waiters,
			// since Run is sitting on cond.Wait.
			m.runState = managerStateStopping
			m.cond.Broadcast()
			m.cond.Wait() // Wait on the stopping event

		case managerStateStopping:
			// Still stopping, wait...
			m.cond.Wait()

		case managerStateStopped:
			// Stopped, target state reached
			return nil
		}
	}
}

// Kill will Close the manager and Kill all proxies that were being managed.
//
// This is safe to call with Close already called since Close is idempotent.
func (m *Manager) Kill() error {
	// Close first so that we aren't getting changes in proxies
	if err := m.Close(); err != nil {
		return err
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	var err error
	for id, proxy := range m.proxies {
		if err := proxy.Stop(); err != nil {
			err = multierror.Append(
				err, fmt.Errorf("failed to stop proxy %q: %s", id, err))
			continue
		}

		// Remove it since it is already stopped successfully
		delete(m.proxies, id)
	}

	return err
}

// Run syncs with the local state and supervises existing proxies.
//
// This blocks and should be run in a goroutine. If another Run is already
// executing, this will do nothing and return.
func (m *Manager) Run() {
	m.lock.Lock()
	if m.runState != managerStateIdle {
		m.lock.Unlock()
		return
	}

	// Set the state to running
	m.runState = managerStateRunning
	m.lock.Unlock()

	// Start a goroutine that just waits for a stop request
	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		m.lock.Lock()
		defer m.lock.Unlock()

		// We wait for anything not running, just so we're more resilient
		// in the face of state machine issues. Basically any state change
		// will cause us to quit.
		for m.runState == managerStateRunning {
			m.cond.Wait()
		}
	}()

	// When we exit, we set the state to stopped and broadcast to any
	// waiting Close functions that they can return.
	defer func() {
		m.lock.Lock()
		m.runState = managerStateStopped
		m.cond.Broadcast()
		m.lock.Unlock()
	}()

	// Register for proxy catalog change notifications
	notifyCh := make(chan struct{}, 1)
	m.State.NotifyProxy(notifyCh)
	defer m.State.StopNotifyProxy(notifyCh)

	for {
		// Sync first, before waiting on further notifications so that
		// we can start with a known-current state.
		m.sync()

		select {
		case <-notifyCh:
			// Changes exit select so we can reloop and reconfigure proxies

		case <-stopCh:
			// Stop immediately, no cleanup
			return
		}
	}
}

// sync syncs data with the local state store to update the current manager
// state and start/stop necessary proxies.
func (m *Manager) sync() {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Get the current set of proxies
	state := m.State.Proxies()

	// Go through our existing proxies that we're currently managing to
	// determine if they're still in the state or not. If they're in the
	// state, we need to diff to determine if we're starting a new proxy
	// If they're not in the state, then we need to stop the proxy since it
	// is now orphaned.
	for id, proxy := range m.proxies {
		// Get the proxy.
		stateProxy, ok := state[id]
		if !ok {
			// Proxy is deregistered. Remove it from our map and stop it
			delete(m.proxies, id)
			if err := proxy.Stop(); err != nil {
				m.Logger.Printf("[ERROR] agent/proxy: failed to stop deregistered proxy for %q: %s", id, err)
			}

			continue
		}

		// Proxy is in the state. Always delete it so that the remainder
		// are NEW proxies that we start after this loop.
		delete(state, id)

		// TODO: diff and restart if necessary
		println("DIFF", id, stateProxy)
	}

	// Remaining entries in state are new proxies. Start them!
	for id, stateProxy := range state {
		proxy, err := m.newProxy(stateProxy)
		if err != nil {
			m.Logger.Printf("[ERROR] agent/proxy: failed to initialize proxy for %q: %s", id, err)
			continue
		}

		if err := proxy.Start(); err != nil {
			m.Logger.Printf("[ERROR] agent/proxy: failed to start proxy for %q: %s", id, err)
			continue
		}

		m.proxies[id] = proxy
	}
}

// newProxy creates the proper Proxy implementation for the configured
// local managed proxy.
func (m *Manager) newProxy(mp *local.ManagedProxy) (Proxy, error) {
	// Defensive because the alternative is to panic which is not desired
	if mp == nil || mp.Proxy == nil {
		return nil, fmt.Errorf("internal error: nil *local.ManagedProxy or Proxy field")
	}

	p := mp.Proxy
	switch p.ExecMode {
	case structs.ProxyExecModeDaemon:
		command := p.Command
		if len(command) == 0 {
			command = p.CommandDefault
		}

		// This should never happen since validation should happen upstream
		// but verify it because the alternative is to panic below.
		if len(command) == 0 {
			return nil, fmt.Errorf("daemon mode managed proxy requires command")
		}

		// Build the command to execute.
		var cmd exec.Cmd
		cmd.Path = command[0]
		cmd.Args = command[1:]

		// Build the daemon structure
		return &Daemon{
			Command:    &cmd,
			ProxyToken: mp.ProxyToken,
			Logger:     m.Logger,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported managed proxy type: %q", p.ExecMode)
	}
}
