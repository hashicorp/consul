package tokenread

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
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

	tokenID  string
	self     bool
	showMeta bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and Raft indices should be shown for each entry")
	c.flags.BoolVar(&c.self, "self", false, "Indicates that the current HTTP token "+
		"should be read by secret ID instead of expecting a -id option")
	c.flags.StringVar(&c.tokenID, "id", "", "The Accessor ID of the token to read. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.tokenID == "" && !c.self {
		c.UI.Error(fmt.Sprintf("Must specify the -id parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var token *api.ACLToken
	if !c.self {
		tokenID, err := acl.GetTokenIDFromPartial(client, c.tokenID)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
			return 1
		}

		token, _, err = client.ACL().TokenRead(tokenID, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading token %q: %v", tokenID, err))
			return 1
		}
	} else {
		token, _, err = client.ACL().TokenReadSelf(nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading token: %v", err))
			return 1
		}
	}

	acl.PrintToken(token, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Read an ACL token"
const help = `
Usage: consul acl token read [options] -id TOKENID

  This command will retrieve and print out the details of
  a single token.

  Using a partial ID:

          $ consul acl token read -id 4be56c77-82

  Using the full ID:

          $ consul acl token read -id 4be56c77-8244-4c7d-b08c-667b8c71baed
`
