package command

import (
	"fmt"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/mitchellh/cli"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	HumanVersion string
	UI           cli.Ui
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Run(_ []string) int {
	c.UI.Output(fmt.Sprintf("Consul %s", c.HumanVersion))

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

func (c *VersionCommand) Synopsis() string {
	return "Prints the Consul version"
}
