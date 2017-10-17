package version

import (
	"fmt"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui, version string) *cmd {
	return &cmd{UI: ui, version: version}
}

type cmd struct {
	UI      cli.Ui
	version string
}

func (c *cmd) Run(_ []string) int {
	c.UI.Output(fmt.Sprintf("Consul %s", c.version))

	rpcProtocol, err := config.DefaultRPCProtocol()
	if err != nil {
		c.UI.Error(err.Error())
		return 2
	}
	var supplement string
	if rpcProtocol < consul.ProtocolVersionMax {
		supplement = fmt.Sprintf(" (agent will automatically use protocol >%d when speaking to compatible agents)",
			rpcProtocol)
	}
	c.UI.Output(fmt.Sprintf("Protocol %d spoken by default, understands %d to %d%s",
		rpcProtocol, consul.ProtocolVersionMin, consul.ProtocolVersionMax, supplement))

	return 0
}

func (c *cmd) Synopsis() string {
	return "Prints the Consul version"
}

func (c *cmd) Help() string {
	return ""
}
