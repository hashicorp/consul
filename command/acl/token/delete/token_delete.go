package tokendelete

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/flags"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI              cli.Ui
	flags           *flag.FlagSet
	http            *flags.HTTPFlags
	help            string
	tokenAccessorID string

	tokenID string // DEPRECATED
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.tokenAccessorID, "accessor-id", "", "The Accessor ID of the token to delete. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs")
	c.http = &flags.HTTPFlags{}
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)

	// Deprecations
	c.flags.StringVar(&c.tokenID, "id", "",
		"DEPRECATED. Use -accessor-id instead.")
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	tokenAccessor := c.tokenAccessorID
	if tokenAccessor == "" {
		if c.tokenID == "" {
			c.UI.Error("Must specify the -accessor-id parameter")
			return 1
		} else {
			tokenAccessor = c.tokenID
			c.UI.Warn("Use the -accessor-id parameter to specify token by Accessor ID.")
		}
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	tok, err := acl.GetTokenAccessorIDFromPartial(client, tokenAccessor)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
		return 1
	}

	if _, err := client.ACL().TokenDelete(tok, nil); err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting token %q: %v", tok, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Token %q deleted successfully", tok))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Delete an ACL token"
	help     = `
Usage: consul acl token delete [options] -accessor-id TOKEN

  Deletes an ACL token by providing either the ID or a unique ID prefix.

      Delete by prefix:

          $ consul acl token delete -accessor-id b6b85

      Delete by full ID:

          $ consul acl token delete -accessor-id b6b856da-5193-4e78-845a-7d61ca8371ba
`
)
