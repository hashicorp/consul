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
		return &proxy.Daemon{
			Command:    exec.Command(p.Command),
			ProxyToken: pToken,
			Logger:     s.logger,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported managed proxy type: %q", p.ExecMode)
	}
}
