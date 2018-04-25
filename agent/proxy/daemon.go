package proxy

import (
	"os/exec"
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
}

// Start starts the daemon and keeps it running.
func (p *Daemon) Start() error {
}
