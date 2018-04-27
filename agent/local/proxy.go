package local

import (
	"fmt"
	"os/exec"

	"github.com/hashicorp/consul/agent/proxy"
	"github.com/hashicorp/consul/agent/structs"
)

// newProxyProcess returns the proxy.Proxy for the given ManagedProxy
// state entry. proxy.Proxy is the actual managed process. The returned value
// is the initialized struct but isn't explicitly started.
func (s *State) newProxyProcess(p *structs.ConnectManagedProxy, pToken string) (proxy.Proxy, error) {
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
		return &proxy.Daemon{
			Command:    &cmd,
			ProxyToken: pToken,
			Logger:     s.logger,
		}, nil

	case structs.ProxyExecModeScript:
		return &proxy.Noop{}, nil

	default:
		return nil, fmt.Errorf("unsupported managed proxy type: %q", p.ExecMode)
	}
}
