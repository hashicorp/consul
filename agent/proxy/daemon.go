package proxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
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

	// ProxyToken is the special local-only ACL token that allows a proxy
	// to communicate to the Connect-specific endpoints.
	ProxyToken string

	// Logger is where logs will be sent around the management of this
	// daemon. The actual logs for the daemon itself will be sent to
	// a file.
	Logger *log.Logger

	// process is the started process
	lock    sync.Mutex
	process *os.Process
	stopCh  chan struct{}
}

// Start starts the daemon and keeps it running.
//
// This function returns after the process is successfully started.
func (p *Daemon) Start() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// If the daemon is already started, return no error.
	if p.stopCh != nil {
		return nil
	}

	// Start it for the first time
	process, err := p.start()
	if err != nil {
		return err
	}

	// Create the stop channel we use to notify when we've gracefully stopped
	stopCh := make(chan struct{})
	p.stopCh = stopCh

	// Store the process so that we can signal it later
	p.process = process

	go p.keepAlive(stopCh)

	return nil
}

func (p *Daemon) keepAlive(stopCh chan struct{}) {
	p.lock.Lock()
	process := p.process
	p.lock.Unlock()

	for {
		if process == nil {
			p.lock.Lock()

			// If we gracefully stopped (stopCh is closed) then don't restart. We
			// check stopCh and not p.stopCh because the latter could reference
			// a new process.
			select {
			case <-stopCh:
				p.lock.Unlock()
				return
			default:
			}

			// Process isn't started currently. We're restarting.
			var err error
			process, err = p.start()
			if err != nil {
				p.Logger.Printf("[ERR] agent/proxy: error restarting daemon: %s", err)
			}

			p.process = process
			p.lock.Unlock()
		}

		_, err := process.Wait()
		process = nil
		p.Logger.Printf("[INFO] agent/proxy: daemon exited: %s", err)
	}
}

// start starts and returns the process. This will create a copy of the
// configured *exec.Command with the modifications documented on Daemon
// such as setting the proxy token environmental variable.
func (p *Daemon) start() (*os.Process, error) {
	cmd := *p.Command

	// Add the proxy token to the environment. We first copy the env because
	// it is a slice and therefore the "copy" above will only copy the slice
	// reference. We allocate an exactly sized slice.
	cmd.Env = make([]string, len(p.Command.Env), len(p.Command.Env)+1)
	copy(cmd.Env, p.Command.Env)
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", EnvProxyToken, p.ProxyToken))

	// Start it
	err := cmd.Start()
	return cmd.Process, err
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
	defer p.lock.Unlock()

	// If we don't have a stopCh then we never even started yet.
	if p.stopCh == nil {
		return nil
	}

	// If stopCh is closed, then we're already stopped
	select {
	case <-p.stopCh:
		return nil
	default:
	}

	err := p.process.Signal(os.Interrupt)

	// This signals that we've stopped and therefore don't want to restart
	close(p.stopCh)
	p.stopCh = nil

	return err
	//return p.Command.Process.Kill()
}
