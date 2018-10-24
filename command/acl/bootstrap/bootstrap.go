package bootstrap

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	token, _, err := client.ACL().Bootstrap()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed ACL bootstrapping: %v", err))
		return 1
	}

	acl.PrintToken(token, c.UI, false)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Bootstrap Consul's ACL system"

// TODO (ACL-V2) - maybe embed link to bootstrap reset docs
const help = `
Usage: consul acl bootstrap [options]

  The bootstrap command will request Consul to generate a new token with unlimited privileges to use
  for management purposes and output its details. This can only be done once and afterwards bootstrapping
  will be disabled. If all tokens are lost and you need to bootstrap again you can follow the bootstrap
  reset procedure
`
