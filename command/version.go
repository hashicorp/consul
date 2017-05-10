package command

import (
	"fmt"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/consul"
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
	config := agent.DefaultConfig()

	c.UI.Output(c.HumanVersion)
	c.UI.Output(fmt.Sprintf("default protocol: %d", config.Protocol))
	c.UI.Output(fmt.Sprintf("minimum protocol: %d", consul.ProtocolVersionMin))
	c.UI.Output(fmt.Sprintf("maximum protocol: %d", consul.ProtocolVersionMax))

	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Consul version"
}
