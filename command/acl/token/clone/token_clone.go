// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tokenclone

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/acl/token"
	"github.com/hashicorp/consul/command/flags"
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

	tokenAccessorID string
	description     string
	format          string

	tokenID string // DEPRECATED
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.tokenAccessorID, "accessor-id", "", "The Accessor ID of the token to clone. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs. The special value of 'anonymous' may "+
		"be provided instead of the anonymous tokens accessor ID")
	c.flags.StringVar(&c.description, "description", "", "A description of the new cloned token")
	c.flags.StringVar(
		&c.format,
		"format",
		token.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(token.GetSupportedFormats(), "|")),
	)
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
			c.UI.Error("Cannot update a token without specifying the -accessor-id parameter")
			return 1
		} else {
			tokenAccessor = c.tokenID
			c.UI.Warn("The -id parameter is deprecated. Use the -accessor-id parameter instead.")
		}
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	tok, err := acl.GetTokenAccessorIDFromPartial(client, tokenAccessor)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error determining token Accessor ID: %v", err))
		return 1
	}

	t, _, err := client.ACL().TokenClone(tok, c.description, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error cloning token: %v", err))
		return 1
	}

	formatter, err := token.NewFormatter(c.format, false)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	out, err := formatter.FormatToken(t)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	if out != "" {
		c.UI.Info(out)
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Clone an ACL token"
	help     = `
Usage: consul acl token clone [options]

    This command will clone a token. When cloning an alternate description may be given
    for use with the new token.

    Example:

        $ consul acl token clone -accessor-id abcd -description "replication"
`
)
