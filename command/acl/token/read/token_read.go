package tokenread

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
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
	self            bool
	showMeta        bool
	format          string
	expanded        bool

	tokenID string // DEPRECATED
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that token metadata such "+
		"as the content hash and Raft indices should be shown for each entry")
	c.flags.BoolVar(&c.self, "self", false, "Indicates that the current HTTP token "+
		"should be read by secret ID instead of expecting a -accessor-id option")
	c.flags.BoolVar(&c.expanded, "expanded", false, "Indicates that the contents of the "+
		" policies and roles affecting the token should also be shown.")
	c.flags.StringVar(&c.tokenAccessorID, "accessor-id", "", "The Accessor ID of the token to read. "+
		"It may be specified as a unique ID prefix but will error if the prefix "+
		"matches multiple token Accessor IDs")
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

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	var t *api.ACLToken
	var expanded *api.ACLTokenExpanded
	if !c.self {
		tokenAccessor := c.tokenAccessorID
		if tokenAccessor == "" {
			if c.tokenID == "" {
				c.UI.Error("Must specify the -accessor-id parameter")
				return 1
			} else {
				tokenAccessor = c.tokenID
				c.UI.Warn("Use the -accessor-id parameter to specify token by Accessor ID")
			}
		}

		tok, err := acl.GetTokenAccessorIDFromPartial(client, tokenAccessor)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error determining token ID: %v", err))
			return 1
		}

		if !c.expanded {
			t, _, err = client.ACL().TokenRead(tok, nil)
		} else {
			expanded, _, err = client.ACL().TokenReadExpanded(tok, nil)
		}

		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading token %q: %v", tok, err))
			return 1
		}
	} else {
		// TODO: consider updating this CLI command and underlying HTTP API endpoint
		// to support expanded read of a "self" token, which is a much better user workflow.
		if c.expanded {
			c.UI.Error("Cannot use both -expanded and -self. Instead, use -expanded and -accessor-id=<accessor id>.")
			return 1
		}

		t, _, err = client.ACL().TokenReadSelf(nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading token: %v", err))
			return 1
		}
	}

	formatter, err := token.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	var out string
	if !c.expanded {
		out, err = formatter.FormatToken(t)
	} else {
		out, err = formatter.FormatTokenExpanded(expanded)
	}
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
	synopsis = "Read an ACL token"
	help     = `
Usage: consul acl token read [options] -accessor-id TOKENID

  This command will retrieve and print out the details of
  a single token.

  Using a partial ID:

          $ consul acl token read -accessor-id 4be56c77-82

  Using the full ID:

          $ consul acl token read -accessor-id 4be56c77-8244-4c7d-b08c-667b8c71baed
`
)
