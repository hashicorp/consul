package policycreate

import (
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	aclhelpers "github.com/hashicorp/consul/command/acl"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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

	name        string
	description string
	datacenters []string
	rules       string

	fromToken     string
	tokenIsSecret bool
	showMeta      bool

	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.showMeta, "meta", false, "Indicates that policy metadata such "+
		"as the content hash and raft indices should be shown for each entry")
	c.flags.StringVar(&c.name, "name", "", "The new policy's name. This flag is required.")
	c.flags.StringVar(&c.description, "description", "", "A description of the policy")
	c.flags.Var((*flags.AppendSliceValue)(&c.datacenters), "valid-datacenter", "Datacenter "+
		"that the policy should be valid within. This flag may be specified multiple times")
	c.flags.StringVar(&c.rules, "rules", "", "The policy rules. May be prefixed with '@' "+
		"to indicate that the value is a file path to load the rules from. '-' may also be "+
		"given to indicate that the rules are available on stdin")
	c.flags.StringVar(&c.fromToken, "from-token", "", "The legacy token to retrieve the rules "+
		"for when creating this policy. When this is specified no other rules should be given. "+
		"Similar to the -rules option the token to use can be loaded from stdin or from a file")
	c.flags.BoolVar(&c.tokenIsSecret, "token-secret", false, "Indicates the token provided with "+
		"-from-token is a SecretID and not an AccessorID")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) getRules(client *api.Client) (string, error) {
	if c.fromToken != "" && c.rules != "" {
		return "", fmt.Errorf("Cannot specify both -rules and -from-token")
	}

	if c.fromToken != "" {
		tokenID, err := helpers.LoadDataSource(c.fromToken, c.testStdin)
		if err != nil {
			return "", fmt.Errorf("Invalid -from-token value: %v", err)
		}

		rules, err := aclhelpers.GetRulesFromLegacyToken(client, tokenID, c.tokenIsSecret)
		if err != nil {
			return "", err
		}

		translated, err := acl.TranslateLegacyRules([]byte(rules))
		return string(translated), err
	}

	return helpers.LoadDataSource(c.rules, c.testStdin)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.name == "" {
		c.UI.Error(fmt.Sprintf("Missing require '-name' flag"))
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	rules, err := c.getRules(client)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error loading rules: %v", err))
		return 1
	}

	newPolicy := &api.ACLPolicy{
		Name:        c.name,
		Description: c.description,
		Datacenters: c.datacenters,
		Rules:       rules,
	}

	policy, _, err := client.ACL().PolicyCreate(newPolicy, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create new policy: %v", err))
		return 1
	}

	aclhelpers.PrintPolicy(policy, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Create an ACL policy"
const help = `
Usage: consul acl policy create -name NAME [options]

    Both the -rules and -from-token option values allow loading the value
    from stdin, a file or the raw value. To use stdin pass '-' as the value.
    To load the value from a file prefix the value with an '@'. Any other
    values will be used directly.

    Create a new policy:

        $ consul acl policy create -name "new-policy" \
                                   -description "This is an example policy" \
                                   -datacenter "dc1" \
                                   -datacenter "dc2" \
                                   -rules @rules.hcl

    Creation a policy from a legacy token:

        $ consul acl policy create -name "legacy-policy" \
                                   -description "Token Converted to policy" \
                                   -from-token "c1e34113-e7ab-4451-b1a6-336ddcc58fc6"
`
