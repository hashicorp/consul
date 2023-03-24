package rules

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/acl"
	aclhelpers "github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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

	tokenAccessor bool
	tokenSecret   bool

	// testStdin is the input for testing
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.tokenAccessor, "token-accessor", false, "Specifies that "+
		"the TRANSLATE argument refers to a ACL token AccessorID. "+
		"The rules to translate will then be read from the retrieved token")

	c.flags.BoolVar(&c.tokenSecret, "token-secret", false,
		"Specifies that the TRANSLATE argument refers to a ACL token SecretID. "+
			"The rules to translate will then be read from the retrieved token")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.tokenSecret && c.tokenAccessor {
		c.UI.Error(fmt.Sprintf("Error - cannot specify both -token-secret and -token-accessor"))
		return 1
	}

	data, err := c.dataFromArgs(c.flags.Args())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error! %v", err))
		return 1
	}

	if c.tokenSecret || c.tokenAccessor {
		client, err := c.http.APIClient()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error connecting to Consul Agent: %s", err))
			return 1
		}

		// Trim whitespace and newlines (e.g. from echo without -n)
		data = strings.TrimSpace(data)

		// It is not a bug that this doesn't look at tokenAccessor. We already know that we want the rules from
		// a token and just need to tell the helper function whether it should be retrieved by its secret or accessor
		if rules, err := aclhelpers.GetRulesFromLegacyToken(client, data, c.tokenSecret); err != nil {
			c.UI.Error(err.Error())
			return 1
		} else {
			data = rules
		}
	}

	translated, err := acl.TranslateLegacyRules([]byte(data))
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error translating rules: %s", err))
		return 1
	}

	c.UI.Info(string(translated))
	return 0
}

func (c *cmd) dataFromArgs(args []string) (string, error) {
	switch len(args) {
	case 0:
		return "", fmt.Errorf("Missing TRANSLATE argument")
	case 1:
		data, err := helpers.LoadDataSource(args[0], c.testStdin)
		if err != nil {
			return "", err
		}

		return data, nil
	default:
		return "", fmt.Errorf("Too many arguments: expected 1 got %d", len(args))
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Translate the legacy rule syntax into the current syntax"
const help = `
Usage: consul acl translate-rules  [options] TRANSLATE

  Translates the legacy ACL rule syntax into the current syntax.

  Translate rules within a file:

      $ consul acl translate-rules @rules.hcl

  Translate rules from stdin:

      $ consul acl translate-rules -

  Translate rules from a string argument:

      $ consul acl translate-rules 'key "" { policy = "write"}'

  Translate rules for a legacy ACL token using its SecretID passed from stdin:

      $ consul acl translate-rules -token-secret -

  Translate rules for a legacy ACL token using its AccessorID:

      $ consul acl translate-rules -token-accessor 429cd746-03d5-4bbb-a83a-18b164171c89
`
