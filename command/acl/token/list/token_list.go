package tokenlist

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

	showMeta bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and Raft indices should be shown for each entry")
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

	tokens, _, err := client.ACL().TokenList(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to retrieve the token list: %v", err))
		return 1
	}

	first := true
	for _, token := range tokens {
		if first {
			first = false
		} else {
			c.UI.Info("")
		}
		acl.PrintTokenListEntry(token, c.UI, c.showMeta)
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "List ACL tokens"
const help = `
Usage: consul acl token list [options]

  List all the ACL tokens

          $ consul acl token list
`
