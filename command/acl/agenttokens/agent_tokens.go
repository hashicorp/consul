// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agenttokens

import (
	"flag"
	"fmt"
	"io"

	"github.com/mitchellh/cli"

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

	testStdin io.Reader
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

	tokenType, token, err := c.dataFromArgs(c.flags.Args())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error! %s", err))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul Agent: %s", err))
		return 1
	}

	switch tokenType {
	case "default":
		_, err = client.Agent().UpdateDefaultACLToken(token, nil)
	case "agent":
		_, err = client.Agent().UpdateAgentACLToken(token, nil)
	case "recovery":
		_, err = client.Agent().UpdateAgentRecoveryACLToken(token, nil)
	case "replication":
		_, err = client.Agent().UpdateReplicationACLToken(token, nil)
	case "config_file_service_registration":
		_, err = client.Agent().UpdateConfigFileRegistrationToken(token, nil)
	default:
		c.UI.Error(fmt.Sprintf("Unknown token type"))
		return 1
	}

	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to set ACL token %q: %v", tokenType, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("ACL token %q set successfully", tokenType))
	return 0
}

func (c *cmd) dataFromArgs(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		return "", "", fmt.Errorf("Missing TYPE and TOKEN arguments")
	case 1:
		switch args[0] {
		case "default", "agent", "recovery", "replication":
			return "", "", fmt.Errorf("Missing TOKEN argument")
		default:
			return "", "", fmt.Errorf("MISSING TYPE argument")
		}
	case 2:
		data, err := helpers.LoadDataSource(args[1], c.testStdin)
		if err != nil {
			return "", "", err
		}

		return args[0], data, nil
	default:
		return "", "", fmt.Errorf("Too many arguments: expected 2 got %d", len(args))
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Assign tokens for the Consul Agent's usage"
const help = `
Usage: consul acl set-agent-token [options] TYPE TOKEN

  This command will set the corresponding token for the agent to use. If token
  persistence is not enabled, then tokens uploaded this way are not persisted
  and if the agent reloads then the tokens will need to be set again.

  Token Types:

    default                           The default token is the token that the agent will use for
                                      both internal agent operations and operations initiated by
                                      the HTTP and DNS interfaces when no specific token is provided.
                                      If not set the agent will use the anonymous token.

    agent                             The token that the agent will use for internal agent operations.
                                      If not given then the default token is used for these operations.

    recovery                          This sets the token that can be used to access the Agent APIs in
                                      the event that the ACL datacenter cannot be reached.

    replication                       This is the token that the agent will use for replication
                                      operations. This token will need to be configured with read access
                                      to whatever data is being replicated.

    config_file_service_registration  This is the token that the agent uses to register services
                                      and checks defined in config files. This token needs to
                                      be configured with permission for the service or checks
                                      being registered. If not set, the default token is used.
                                      If a service or check definition contains a 'token'
                                      field, then that token is used instead.

  Example:

    $ consul acl set-agent-token default c4d0f8df-3aba-4ab6-a7a0-35b760dc29a1
`
