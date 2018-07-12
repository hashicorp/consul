package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-multierror"
)

const (
	// ManagerCoalescePeriod and ManagerQuiescentPeriod relate to how
	// notifications in updates from the local state are colaesced to prevent
	// lots of churn in the manager.
	//
	// When the local state updates, the manager will wait for quiescence.
	// For each update, the quiscence timer is reset. If the coalesce period
	// is reached, the manager will update proxies regardless of the frequent
	// changes. Then the whole cycle resets.
	ManagerCoalescePeriod  = 5 * time.Second
	ManagerQuiescentPeriod = 500 * time.Millisecond

	// ManagerSnapshotPeriod is the interval that snapshots are taken.
	// The last snapshot state is preserved and if it matches a file isn't
	// written, so its safe for this to be reasonably frequent.
	ManagerSnapshotPeriod = 1 * time.Second
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
// The change notifications from the local state are coalesced (see
// ManagerCoalescePeriod) so that frequent changes within the local state
// do not trigger dozens of proxy resyncs.
type Manager struct {
	// State is the local state that is the source of truth for all
	// configured managed proxies.
	State *local.State

	// Logger is the logger for information about manager behavior.
	// Output for proxies will not go here generally but varies by proxy
	// implementation type.
	Logger *log.Logger

	// DataDir is the path to the directory where data for proxies is
	// written, including snapshots for any state changes in the manager.
	// Within the data dir, files will be written in the following locatins:
	//
	//   * logs/ - log files named <service id>-std{out|err}.log
	//   * pids/ - pid files for daemons named <service id>.pid
	//   * snapshot.json - the state of the manager
	//
	DataDir string

	// Extra environment variables to set for the proxies
	ProxyEnv []string

	// SnapshotPeriod is the duration between snapshots. This can be set
	// relatively low to ensure accuracy, because if the new snapshot matches
	// the last snapshot taken, no file will be written. Therefore, setting
	// this low causes only slight CPU/memory usage but doesn't result in
	// disk IO. If this isn't set, ManagerSnapshotPeriod will be the default.
	//
	// This only has an effect if snapshots are enabled (DataDir is set).
	SnapshotPeriod time.Duration

	// CoalescePeriod and QuiescencePeriod control the timers for coalescing
	// updates from the local state. See the defaults at the top of this
	// file for more documentation. These will be set to those defaults
	// by NewManager.
	CoalescePeriod  time.Duration
	QuiescentPeriod time.Duration

	// AllowRoot configures whether proxies can be executed as root (EUID == 0).
	// If this is false then the manager will run and proxies can be added
	// and removed but none will be started an errors will be logged
	// to the logger.
	AllowRoot bool

	// lock is held while reading/writing any internal state of the manager.
	// cond is a condition variable on lock that is broadcasted for runState
	// changes.
	lock *sync.Mutex
	cond *sync.Cond

	// runState is the current state of the manager. To read this the
	// lock must be held. The condition variable cond can be waited on
	// for changes to this value.
	runState managerRunState

	// lastSnapshot stores a pointer to the last snapshot that successfully
	// wrote to disk. This is used for dup detection to prevent rewriting
	// the same snapshot multiple times. snapshots should never be that
	// large so keeping it in-memory should be cheap even for thousands of
	// proxies (unlikely scenario).
	lastSnapshot *snapshot

	proxies map[string]Proxy
}

// NewManager initializes a Manager. After initialization, the exported
// fields should be configured as desired. To start the Manager, execute
// Run in a goroutine.
func NewManager() *Manager {
	var lock sync.Mutex
	return &Manager{
		Logger:          defaultLogger,
		SnapshotPeriod:  ManagerSnapshotPeriod,
		CoalescePeriod:  ManagerCoalescePeriod,
		QuiescentPeriod: ManagerQuiescentPeriod,
		lock:            &lock,
		cond:            sync.NewCond(&lock),
		proxies:         make(map[string]Proxy),
	}
}

// defaultLogger is the defaultLogger for NewManager so there it is never nil
var defaultLogger = log.New(os.Stderr, "", log.LstdFlags)

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

	return m.stop(func(p Proxy) error {
		return p.Close()
	})
}

// Kill will Close the manager and Kill all proxies that were being managed.
// Only ONE of Kill or Close must be called. If Close has been called already
// then this will have no effect.
func (m *Manager) Kill() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.stop(func(p Proxy) error {
		return p.Stop()
	})
}

// stop stops the run loop and cleans up all the proxies by calling
// the given cleaner. If the cleaner returns an error the proxy won't be
// removed from the map.
//
// The lock must be held while this is called.
func (m *Manager) stop(cleaner func(Proxy) error) error {
	for {
		// Special case state that exits the for loop
		if m.runState == managerStateStopped {
			break
		}

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
		}
	}

	// Clean up all the proxies
	var err error
	for id, proxy := range m.proxies {
		if err := cleaner(proxy); err != nil {
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

	// Start the timer for snapshots. We don't use a ticker because disk
	// IO can be slow and we don't want overlapping notifications. So we only
	// reset the timer once the snapshot is complete rather than continously.
	snapshotTimer := time.NewTimer(m.SnapshotPeriod)
	defer snapshotTimer.Stop()

	m.Logger.Println("[DEBUG] agent/proxy: managed Connect proxy manager started")
SYNC:
	for {
		// Sync first, before waiting on further notifications so that
		// we can start with a known-current state.
		m.sync()

		// Note for these variables we don't use a time.Timer because both
		// periods are relatively short anyways so they end up being eligible
		// for GC very quickly, so overhead is not a concern.
		var quiescent, quantum <-chan time.Time

		// Start a loop waiting for events from the local state store. This
		// loops rather than just `select` so we can coalesce many state
		// updates over a period of time.
		for {
			select {
			case <-notifyCh:
				// If this is our first notification since the last sync,
				// reset the quantum timer which is the max time we'll wait.
				if quantum == nil {
					quantum = time.After(m.CoalescePeriod)
				}

				// Always reset the quiescent timer
				quiescent = time.After(m.QuiescentPeriod)

			case <-quantum:
				continue SYNC

			case <-quiescent:
				continue SYNC

			case <-snapshotTimer.C:
				// Perform a snapshot
				if path := m.SnapshotPath(); path != "" {
					if err := m.snapshot(path, true); err != nil {
						m.Logger.Printf("[WARN] agent/proxy: failed to snapshot state: %s", err)
					}
				}

				// Reset
				snapshotTimer.Reset(m.SnapshotPeriod)

			case <-stopCh:
				// Stop immediately, no cleanup
				m.Logger.Println("[DEBUG] agent/proxy: Stopping managed Connect proxy manager")
				return
			}
		}
	}
}

// sync syncs data with the local state store to update the current manager
// state and start/stop necessary proxies.
func (m *Manager) sync() {
	m.lock.Lock()
	defer m.lock.Unlock()

	// If we don't allow root and we're root, then log a high sev message.
	if !m.AllowRoot && isRoot() {
		m.Logger.Println("[WARN] agent/proxy: running as root, will not start managed proxies")
		return
	}

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
		if ok {
			// Remove the proxy from the state so we don't start it new.
			delete(state, id)

			// Make the proxy so we can compare. This does not start it.
			proxy2, err := m.newProxy(stateProxy)
			if err != nil {
				m.Logger.Printf("[ERROR] agent/proxy: failed to initialize proxy for %q: %s", id, err)
				continue
			}

			// If the proxies are equal, then do nothing
			if proxy.Equal(proxy2) {
				continue
			}

			// Proxies are not equal, so we should stop it. We add it
			// back to the state here (unlikely case) so the loop below starts
			// the new one.
			state[id] = stateProxy

			// Continue out of `if` as if proxy didn't exist so we stop it
		}

		// Proxy is deregistered. Remove it from our map and stop it
		delete(m.proxies, id)
		if err := proxy.Stop(); err != nil {
			m.Logger.Printf("[ERROR] agent/proxy: failed to stop deregistered proxy for %q: %s", id, err)
		}
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

	// We reuse the service ID a few times
	id := p.ProxyService.ID

	// Create the Proxy. We could just as easily switch on p.ExecMode
	// but I wanted there to be only location where ExecMode => Proxy so
	// it lowers the chance that is wrong.
	proxy, err := m.newProxyFromMode(p.ExecMode, id)
	if err != nil {
		return nil, err
	}

	// Depending on the proxy type we configure the rest from our ManagedProxy
	switch proxy := proxy.(type) {
	case *Daemon:
		command := p.Command

		// This should never happen since validation should happen upstream
		// but verify it because the alternative is to panic below.
		if len(command) == 0 {
			return nil, fmt.Errorf("daemon mode managed proxy requires command")
		}

		// Build the command to execute.
		var cmd exec.Cmd
		cmd.Path = command[0]
		cmd.Args = command // idx 0 is path but preserved since it should be
		if err := m.configureLogDir(id, &cmd); err != nil {
			return nil, fmt.Errorf("error configuring proxy logs: %s", err)
		}

		// Pass in the environmental variables for the proxy process
		cmd.Env = append(m.ProxyEnv, os.Environ()...)

		// Build the daemon structure
		proxy.Command = &cmd
		proxy.ProxyID = id
		proxy.ProxyToken = mp.ProxyToken
		return proxy, nil

	default:
		return nil, fmt.Errorf("unsupported managed proxy type: %q", p.ExecMode)
	}
}

// newProxyFromMode just initializes the proxy structure from only the mode
// and the service ID. This is a shared method between newProxy and Restore
// so that we only have one location where we turn ExecMode into a Proxy.
func (m *Manager) newProxyFromMode(mode structs.ProxyExecMode, id string) (Proxy, error) {
	switch mode {
	case structs.ProxyExecModeDaemon:
		return &Daemon{
			Logger:  m.Logger,
			PidPath: pidPath(filepath.Join(m.DataDir, "pids"), id),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported managed proxy type: %q", mode)
	}
}

// configureLogDir sets up the file descriptors to stdout/stderr so that
// they log to the proper file path for the given service ID.
func (m *Manager) configureLogDir(id string, cmd *exec.Cmd) error {
	// Create the log directory
	logDir := ""
	if m.DataDir != "" {
		logDir = filepath.Join(m.DataDir, "logs")
		if err := os.MkdirAll(logDir, 0700); err != nil {
			return err
		}
	}

	// Configure the stdout, stderr paths
	stdoutPath := logPath(logDir, id, "stdout")
	stderrPath := logPath(logDir, id, "stderr")

	// Open the files. We want to append to each. We expect these files
	// to be rotated by some external process.
	stdoutF, err := os.OpenFile(stdoutPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("error creating stdout file: %s", err)
	}
	stderrF, err := os.OpenFile(stderrPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		// Don't forget to close stdoutF which successfully opened
		stdoutF.Close()

		return fmt.Errorf("error creating stderr file: %s", err)
	}

	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF
	return nil
}

// logPath is a helper to return the path to the log file for the given
// directory, service ID, and stream type (stdout or stderr).
func logPath(dir, id, stream string) string {
	return filepath.Join(dir, fmt.Sprintf("%s-%s.log", id, stream))
}

// pidPath is a helper to return the path to the pid file for the given
// directory and service ID.
func pidPath(dir, id string) string {
	// If no directory is given we do not write a pid
	if dir == "" {
		return ""
	}

	return filepath.Join(dir, fmt.Sprintf("%s.pid", id))
}
