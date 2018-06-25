package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/lib/file"
	"github.com/mitchellh/mapstructure"
)

// Constants related to restart timers with the daemon mode proxies. At some
// point we will probably want to expose these knobs to an end user, but
// reasonable defaults are chosen.
const (
	DaemonRestartHealthy    = 10 * time.Second // time before considering healthy
	DaemonRestartBackoffMin = 3                // 3 attempts before backing off
	DaemonRestartMaxWait    = 1 * time.Minute  // maximum backoff wait time
)

// Daemon is a long-running proxy process. It is expected to keep running
// and to use blocking queries to detect changes in configuration, certs,
// and more.
//
// Consul will ensure that if the daemon crashes, that it is restarted.
type Daemon struct {
	// Command is the command to execute to start this daemon. This must
	// be a Cmd that isn't yet started.
	Command *exec.Cmd

	// ProxyID is the ID of the proxy service. This is required for API
	// requests (along with the token) and is passed via env var.
	ProxyID string

	// ProxyToken is the special local-only ACL token that allows a proxy
	// to communicate to the Connect-specific endpoints.
	ProxyToken string

	// Logger is where logs will be sent around the management of this
	// daemon. The actual logs for the daemon itself will be sent to
	// a file.
	Logger *log.Logger

	// PidPath is the path where a pid file will be created storing the
	// pid of the active process. If this is empty then a pid-file won't
	// be created. Under erroneous conditions, the pid file may not be
	// created but the error will be logged to the Logger.
	PidPath string

	// For tests, they can set this to change the default duration to wait
	// for a graceful quit.
	gracefulWait time.Duration

	// process is the started process
	lock     sync.Mutex
	stopped  bool
	stopCh   chan struct{}
	exitedCh chan struct{}
	process  *os.Process
}

// Start starts the daemon and keeps it running.
//
// This function returns after the process is successfully started.
func (p *Daemon) Start() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// A stopped proxy cannot be restarted
	if p.stopped {
		return fmt.Errorf("stopped")
	}

	// If we're already running, that is okay
	if p.process != nil {
		return nil
	}

	// Setup our stop channel
	stopCh := make(chan struct{})
	exitedCh := make(chan struct{})
	p.stopCh = stopCh
	p.exitedCh = exitedCh

	// Start the loop.
	go p.keepAlive(stopCh, exitedCh)

	return nil
}

// keepAlive starts and keeps the configured process alive until it
// is stopped via Stop.
func (p *Daemon) keepAlive(stopCh <-chan struct{}, exitedCh chan<- struct{}) {
	defer close(exitedCh)

	p.lock.Lock()
	process := p.process
	p.lock.Unlock()

	// attemptsDeadline is the time at which we consider the daemon to have
	// been alive long enough that we can reset the attempt counter.
	//
	// attempts keeps track of the number of restart attempts we've had and
	// is used to calculate the wait time using an exponential backoff.
	var attemptsDeadline time.Time
	var attempts uint32

	// Assume the process is adopted, we reset this when we start a new process
	// ourselves below and use it to decide on a strategy for waiting.
	adopted := true

	for {
		if process == nil {
			// If we're passed the attempt deadline then reset the attempts
			if !attemptsDeadline.IsZero() && time.Now().After(attemptsDeadline) {
				attempts = 0
			}
			// Set ourselves a deadline - we have to make it at least this long before
			// we come around the loop to consider it to have been a "successful"
			// daemon startup and rest the counter above. Note that if the daemon
			// fails before this, we reset the deadline to zero below so that backoff
			// sleeps in the loop don't count as "success" time.
			attemptsDeadline = time.Now().Add(DaemonRestartHealthy)
			attempts++

			// Calculate the exponential backoff and wait if we have to
			if attempts > DaemonRestartBackoffMin {
				exponent := (attempts - DaemonRestartBackoffMin)
				if exponent > 31 {
					exponent = 31
				}
				waitTime := (1 << exponent) * time.Second
				if waitTime > DaemonRestartMaxWait {
					waitTime = DaemonRestartMaxWait
				}

				if waitTime > 0 {
					// If we are waiting, reset the success deadline so we don't
					// accidentally interpret backoff sleep as successful runtime.
					attemptsDeadline = time.Time{}

					p.Logger.Printf(
						"[WARN] agent/proxy: waiting %s before restarting daemon",
						waitTime)

					timer := time.NewTimer(waitTime)
					select {
					case <-timer.C:
						// Timer is up, good!

					case <-stopCh:
						// During our backoff wait, we've been signalled to
						// quit, so just quit.
						timer.Stop()
						return
					}
				}
			}

			p.lock.Lock()

			// If we gracefully stopped then don't restart.
			if p.stopped {
				p.lock.Unlock()
				return
			}

			// Process isn't started currently. We're restarting. Start it
			// and save the process if we have it.
			var err error
			process, err = p.start()
			if err == nil {
				p.process = process
				adopted = false
			}
			p.lock.Unlock()

			if err != nil {
				p.Logger.Printf("[ERR] agent/proxy: error restarting daemon: %s", err)
				continue
			}

		}

		var ps *os.ProcessState
		var err error

		if adopted {
			// assign to err outside scope
			_, err = findProcess(process.Pid)
			if err == nil {
				// Process appears to be running still, wait a bit before we poll again.
				// We want a busy loop, but not too busy. 1 second between detecting a
				// process death seems reasonable.
				//
				// SUBTELTY: we must NOT select on stopCh here since the Stop function
				// assumes that as soon as this method returns and closes exitedCh, that
				// the process is no longer running. If we are polling then we don't
				// know that is true until we've polled again so we have to keep polling
				// until the process goes away even if we know the Daemon is stopping.
				time.Sleep(1 * time.Second)

				// Restart the loop, process is still set so we effectively jump back to
				// the findProcess call above.
				continue
			}
		} else {
			// Wait for child to exit
			ps, err = process.Wait()
		}

		// Process exited somehow.
		process = nil
		if err != nil {
			p.Logger.Printf("[INFO] agent/proxy: daemon exited with error: %s", err)
		} else if ps != nil && !ps.Exited() {
			p.Logger.Printf("[INFO] agent/proxy: daemon left running")
		} else if status, ok := exitStatus(ps); ok {
			p.Logger.Printf("[INFO] agent/proxy: daemon exited with exit code: %d", status)
		}
	}
}

// start starts and returns the process. This will create a copy of the
// configured *exec.Command with the modifications documented on Daemon
// such as setting the proxy token environmental variable.
func (p *Daemon) start() (*os.Process, error) {
	cmd := *p.Command

	// Add the proxy token to the environment. We first copy the env because it is
	// a slice and therefore the "copy" above will only copy the slice reference.
	// We allocate an exactly sized slice.
	//
	// Note that anything we add to the Env here is NOT persisted in the snapshot
	// which only looks at p.Command.Env so it needs to be reconstructible exactly
	// from data in the snapshot otherwise.
	cmd.Env = make([]string, len(p.Command.Env), len(p.Command.Env)+2)
	copy(cmd.Env, p.Command.Env)
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("%s=%s", EnvProxyID, p.ProxyID),
		fmt.Sprintf("%s=%s", EnvProxyToken, p.ProxyToken))

	// Update the Daemon env

	// Args must always contain a 0 entry which is usually the executed binary.
	// To be safe and a bit more robust we default this, but only to prevent
	// a panic below.
	if len(cmd.Args) == 0 {
		cmd.Args = []string{cmd.Path}
	}

	// Perform system-specific setup. In particular, Unix-like systems
	// shuld set sid so that killing the agent doesn't kill the daemon.
	configureDaemon(&cmd)

	// Start it
	p.Logger.Printf("[DEBUG] agent/proxy: starting proxy: %q %#v", cmd.Path, cmd.Args[1:])
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Write the pid file. This might error and that's okay.
	if p.PidPath != "" {
		pid := strconv.FormatInt(int64(cmd.Process.Pid), 10)
		if err := file.WriteAtomic(p.PidPath, []byte(pid)); err != nil {
			p.Logger.Printf(
				"[DEBUG] agent/proxy: error writing pid file %q: %s",
				p.PidPath, err)
		}
	}

	return cmd.Process, nil
}

// Stop stops the daemon.
//
// This will attempt a graceful stop (SIGINT) before force killing the
// process (SIGKILL). In either case, the process won't be automatically
// restarted unless Start is called again.
//
// This is safe to call multiple times. If the daemon is already stopped,
// then this returns no error.
func (p *Daemon) Stop() error {
	p.lock.Lock()

	// If we're already stopped or never started, then no problem.
	if p.stopped || p.process == nil {
		// In the case we never even started, calling Stop makes it so
		// that we can't ever start in the future, either, so mark this.
		p.stopped = true
		p.lock.Unlock()
		return nil
	}

	// Note that we've stopped
	p.stopped = true
	close(p.stopCh)
	process := p.process
	p.lock.Unlock()

	gracefulWait := p.gracefulWait
	if gracefulWait == 0 {
		gracefulWait = 5 * time.Second
	}

	// Defer removing the pid file. Even under error conditions we
	// delete the pid file since Stop means that the manager is no
	// longer managing this proxy and therefore nothing else will ever
	// clean it up.
	if p.PidPath != "" {
		defer func() {
			if err := os.Remove(p.PidPath); err != nil && !os.IsNotExist(err) {
				p.Logger.Printf(
					"[DEBUG] agent/proxy: error removing pid file %q: %s",
					p.PidPath, err)
			}
		}()
	}

	// First, try a graceful stop
	err := process.Signal(os.Interrupt)
	if err == nil {
		select {
		case <-p.exitedCh:
			// Success!
			return nil

		case <-time.After(gracefulWait):
			// Interrupt didn't work
			p.Logger.Printf("[DEBUG] agent/proxy: graceful wait of %s passed, "+
				"killing", gracefulWait)
		}
	} else if isProcessAlreadyFinishedErr(err) {
		// This can happen due to races between signals and polling.
		return nil
	} else {
		p.Logger.Printf("[DEBUG] agent/proxy: sigint failed, killing: %s", err)
	}

	// Graceful didn't work (e.g. on windows where SIGINT isn't implemented),
	// forcibly kill
	err = process.Kill()
	if err != nil && isProcessAlreadyFinishedErr(err) {
		return nil
	}
	return err
}

// Close implements Proxy by stopping the run loop but not killing the process.
// One Close is called, Stop has no effect.
func (p *Daemon) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// If we're already stopped or never started, then no problem.
	if p.stopped || p.process == nil {
		p.stopped = true
		return nil
	}

	// Note that we've stopped
	p.stopped = true
	close(p.stopCh)

	return nil
}

// Equal implements Proxy to check for equality.
func (p *Daemon) Equal(raw Proxy) bool {
	p2, ok := raw.(*Daemon)
	if !ok {
		return false
	}

	// We compare equality on a subset of the command configuration
	return p.ProxyToken == p2.ProxyToken &&
		p.ProxyID == p2.ProxyID &&
		p.Command.Path == p2.Command.Path &&
		p.Command.Dir == p2.Command.Dir &&
		reflect.DeepEqual(p.Command.Args, p2.Command.Args) &&
		reflect.DeepEqual(p.Command.Env, p2.Command.Env)
}

// MarshalSnapshot implements Proxy
func (p *Daemon) MarshalSnapshot() map[string]interface{} {
	p.lock.Lock()
	defer p.lock.Unlock()

	// If we're stopped or have no process, then nothing to snapshot.
	if p.stopped || p.process == nil {
		return nil
	}

	return map[string]interface{}{
		"Pid":         p.process.Pid,
		"CommandPath": p.Command.Path,
		"CommandArgs": p.Command.Args,
		"CommandDir":  p.Command.Dir,
		"CommandEnv":  p.Command.Env,
		"ProxyToken":  p.ProxyToken,
		"ProxyID":     p.ProxyID,
	}
}

// UnmarshalSnapshot implements Proxy
func (p *Daemon) UnmarshalSnapshot(m map[string]interface{}) error {
	var s daemonSnapshot
	if err := mapstructure.Decode(m, &s); err != nil {
		return err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	// Set the basic fields
	p.ProxyToken = s.ProxyToken
	p.ProxyID = s.ProxyID
	p.Command = &exec.Cmd{
		Path: s.CommandPath,
		Args: s.CommandArgs,
		Dir:  s.CommandDir,
		Env:  s.CommandEnv,
	}

	// FindProcess on many systems returns no error even if the process
	// is now dead. We perform an extra check that the process is alive.
	proc, err := findProcess(s.Pid)
	if err != nil {
		return err
	}

	// "Start it"
	stopCh := make(chan struct{})
	exitedCh := make(chan struct{})
	p.stopCh = stopCh
	p.exitedCh = exitedCh
	p.process = proc
	go p.keepAlive(stopCh, exitedCh)

	return nil
}

// daemonSnapshot is the structure of the marshalled data for snapshotting.
//
// Note we don't have to store the ProxyId because this is stored directly
// within the manager snapshot and is restored automatically.
type daemonSnapshot struct {
	// Pid of the process. This is the only value actually required to
	// regain management control. The remainder values are for Equal.
	Pid int

	// Command information
	CommandPath string
	CommandArgs []string
	CommandDir  string
	CommandEnv  []string

	// NOTE(mitchellh): longer term there are discussions/plans to only
	// store the hash of the token but for now we need the full token in
	// case the process dies and has to be restarted.
	ProxyToken string

	ProxyID string
}
