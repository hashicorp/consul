package command

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/mitchellh/cli"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	HumanVersion string
	Ui           cli.Ui
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Run(_ []string) int {
	c.Ui.Output(fmt.Sprintf("Consul Version: %s", c.HumanVersion))
	c.Ui.Output(fmt.Sprintf("Supported Protocol Version(s): %d to %d",
		consul.ProtocolVersionMin, consul.ProtocolVersionMax))
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Consul version"
}
