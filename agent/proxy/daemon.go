package proxy

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strconv"
	"strings"
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
	// Path is the path to the executable to run
	Path string

	// Args are the arguments to run with, the first element should be the same as
	// Path.
	Args []string

	// ProxyId is the ID of the proxy service. This is required for API
	// requests (along with the token) and is passed via env var.
	ProxyId string

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

	// StdoutPath, StderrPath are the paths to the files that stdout and stderr
	// should be written to.
	StdoutPath, StderrPath string

	// gracefulWait can be set for tests and controls how long Stop() will wait
	// for process to terminate before killing. If not set defaults to 5 seconds.
	// If this is lowered for tests, it must remain higher than pollInterval
	// (preferably a factor of 2 or more) or the tests will potentially return
	// errors from Stop() where the process races to Kill() a process that was
	// already stopped by SIGINT but we didn't yet detect the change since poll
	// didn't occur.
	gracefulWait time.Duration

	// pollInterval can be set for tests and controls how frequently the child
	// process is sent SIG 0 to check it's still running. If not set defaults to 1
	// second.
	pollInterval time.Duration

	// daemonizeCmd is set only in tests to control the path and args to the
	// daemonize command.
	daemonizeCmd []string

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

	// Ensure log dirs exist
	if p.StdoutPath != "" {
		if err := os.MkdirAll(path.Dir(p.StdoutPath), 0755); err != nil {
			return err
		}
	}
	if p.StderrPath != "" {
		if err := os.MkdirAll(path.Dir(p.StderrPath), 0755); err != nil {
			return err
		}
	}

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

	for {
		if process == nil {
			// If we're passed the attempt deadline then reset the attempts
			if !attemptsDeadline.IsZero() && time.Now().After(attemptsDeadline) {
				attempts = 0
			}
			attemptsDeadline = time.Now().Add(DaemonRestartHealthy)
			attempts++

			// Calculate the exponential backoff and wait if we have to
			if attempts > DaemonRestartBackoffMin {
				delayedAttempts := attempts - DaemonRestartBackoffMin
				waitTime := time.Duration(0)
				if delayedAttempts > 31 {
					// don't shift off the end of the uint32 if the process is in a crash
					// loop forever
					waitTime = DaemonRestartMaxWait
				} else {
					waitTime = (1 << delayedAttempts) * time.Second
				}
				if waitTime > DaemonRestartMaxWait {
					waitTime = DaemonRestartMaxWait
				}

				if waitTime > 0 {
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

			// Process isn't started currently. We're restarting. Start it and save
			// the process if we have it.
			var err error
			process, err = p.start()
			if err == nil {
				p.process = process
			}
			p.lock.Unlock()

			if err != nil {
				p.Logger.Printf("[ERR] agent/proxy: error restarting daemon: %s", err)
				continue
			}
			// NOTE: it is a postcondition of this method that we don't return while
			// the process is still running. See NOTE below but it's essential that
			// from here to the <-waitCh below nothing can cause this method to exit
			// or loop if err is non-nil.
		}

		// Wait will never work since child is detached and released, so poll the
		// PID with sig 0 to check it's still alive.
		interval := p.pollInterval
		if interval < 1 {
			interval = 1 * time.Second
		}
		waitCh, closer := externalWait(process.Pid, interval)
		defer closer()

		// NOTE: we must not select on anything else here; Stop() requires the
		// postcondition for this method to be that the managed process is not
		// running when we return and the defer above closes p.exitedCh. If we
		// select on stopCh or another signal here we introduce races where we might
		// exit early and leave the actual process running (for example because
		// SIGINT was ignored but Stop saw us exit and assumed all was good). That
		// means that there is no way to stop the Daemon without killing the process
		// but that's OK because agent Shutdown can just persist the state and exit
		// without Stopping this and the right thing will happen. If we ever need to
		// stop managing a process without killing it at a time when the agent
		// process is not shutting down then we might have to re-think that.
		<-waitCh

		// Note that we don't need to call Release explicitly. It's a no-op for Unix
		// but even on Windows it's called automatically by a Finalizer during
		// garbage collection to free the process handle.
		// (https://github.com/golang/go/blob/1174ad3a8f6f9d2318ac45fca3cd90f12915cf04/src/os/exec.go#L26)
		process = nil
		p.Logger.Printf("[INFO] agent/proxy: daemon exited")
	}
}

// start starts and returns the process. This will create a copy of the
// configured *exec.Command with the modifications documented on Daemon
// such as setting the proxy token environmental variable.
func (p *Daemon) start() (*os.Process, error) {
	// Add the proxy token to the environment. We first copy the env because
	// it is a slice and therefore the "copy" above will only copy the slice
	// reference. We allocate an exactly sized slice.
	baseEnv := os.Environ()
	env := make([]string, len(baseEnv), len(baseEnv)+2)
	copy(env, baseEnv)
	env = append(env,
		fmt.Sprintf("%s=%s", EnvProxyId, p.ProxyId),
		fmt.Sprintf("%s=%s", EnvProxyToken, p.ProxyToken))

	// Args must always contain a 0 entry which is usually the executed binary.
	// To be safe and a bit more robust we default this, but only to prevent
	// a panic below.
	if len(p.Args) == 0 {
		p.Args = []string{p.Path}
	}

	// Watch closely, we now swap out the exec.Cmd args for ones to run the same
	// command via the connect daemonize command which takes care of correctly
	// "double forking" to ensure the child is fully detached and adopted by the
	// init process while the agent keeps running.
	var daemonCmd exec.Cmd

	dCmd, err := p.daemonizeCommand()
	if err != nil {
		return nil, err
	}
	daemonCmd.Path = dCmd[0]
	// First arguments are for the stdout, stderr
	daemonCmd.Args = append(dCmd, p.StdoutPath)
	daemonCmd.Args = append(daemonCmd.Args, p.StderrPath)
	daemonCmd.Args = append(daemonCmd.Args, p.Args...)
	daemonCmd.Env = env

	// setup stdout so we can read the PID
	var out bytes.Buffer
	daemonCmd.Stdout = &out
	daemonCmd.Stderr = &out

	// Run it to completion - it should exit immediately (this calls wait to
	// ensure we don't leave the daemonize command as a zombie)
	p.Logger.Printf("[DEBUG] agent/proxy: starting proxy: %q %#v", daemonCmd.Path,
		daemonCmd.Args[1:])
	if err := daemonCmd.Run(); err != nil {
		p.Logger.Printf("[DEBUG] agent/proxy: daemonize output: %s", out.String())
		return nil, err
	}

	// Read the PID from stdout
	outStr, err := out.ReadString('\n')
	if err != nil {
		return nil, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(outStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PID from output of daemonize: %s",
			err)
	}

	// Write the pid file. This might error and that's okay.
	if p.PidPath != "" {
		pid := strconv.Itoa(pid)
		if err := file.WriteAtomic(p.PidPath, []byte(pid)); err != nil {
			p.Logger.Printf(
				"[DEBUG] agent/proxy: error writing pid file %q: %s",
				p.PidPath, err)
		}
	}

	// Finally, adopt the process so we can send signals
	return findProcess(pid)
}

// daemonizeCommand returns the daemonize command.
func (p *Daemon) daemonizeCommand() ([]string, error) {
	// test override
	if p.daemonizeCmd != nil {
		return p.daemonizeCmd, nil
	}
	// Get the path to the current executable. This is cached once by the
	// library so this is effectively just a variable read.
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return []string{execPath, "connect", "daemonize"}, nil
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

	// First, try a graceful stop.
	err := process.Signal(os.Interrupt)
	if err == nil {
		select {
		case <-p.exitedCh:
			// Success!
			return nil

		case <-time.After(gracefulWait):
			// Interrupt didn't work
			p.Logger.Printf("[DEBUG] agent/proxy: gracefull wait of %s passed, "+
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

// stopKeepAlive is like Stop but keeps the process running. This is
// used only for tests.
func (p *Daemon) stopKeepAlive() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// If we're already stopped or never started, then no problem.
	if p.stopped || p.process == nil {
		p.stopped = true
		p.lock.Unlock()
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
		p.Path == p2.Path &&
		reflect.DeepEqual(p.Args, p2.Args)
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
		"Pid":        p.process.Pid,
		"Path":       p.Path,
		"Args":       p.Args,
		"ProxyToken": p.ProxyToken,
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
	p.Path = s.Path
	p.Args = s.Args

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
	Path string
	Args []string

	// NOTE(mitchellh): longer term there are discussions/plans to only
	// store the hash of the token but for now we need the full token in
	// case the process dies and has to be restarted.
	ProxyToken string
}
