// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package login

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib/file"
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

	shutdownCh <-chan struct{}

	bearerToken string

	// flags
	authMethodName string
	authMethodType string

	bearerTokenFile string
	tokenSinkFile   string
	meta            map[string]string

	aws AWSLogin

	enterpriseCmd
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.authMethodName, "method", "",
		"Name of the auth method to login to.")

	c.flags.StringVar(&c.authMethodType, "type", "",
		"Type of the auth method to login to. This field is optional and defaults to no type.")

	c.flags.StringVar(&c.bearerTokenFile, "bearer-token-file", "",
		"Path to a file containing a secret bearer token to use with this auth method.")

	c.flags.StringVar(&c.tokenSinkFile, "token-sink-file", "",
		"The most recent token's SecretID is kept up to date in this file.")

	c.flags.Var((*flags.FlagMapValue)(&c.meta), "meta",
		"Metadata to set on the token, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")
	c.initEnterpriseFlags()

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.aws.flags())
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	if len(c.flags.Args()) > 0 {
		c.UI.Error(fmt.Sprintf("Should have no non-flag arguments."))
		return 1
	}

	if c.authMethodName == "" {
		c.UI.Error(fmt.Sprintf("Missing required '-method' flag"))
		return 1
	}
	if c.tokenSinkFile == "" {
		c.UI.Error(fmt.Sprintf("Missing required '-token-sink-file' flag"))
		return 1
	}

	return c.login()
}

func (c *cmd) bearerTokenLogin() int {
	if err := c.aws.checkFlags(); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.aws.autoBearerToken {
		if c.bearerTokenFile != "" {
			c.UI.Error("Cannot use '-bearer-token-file' flag with '-aws-auto-bearer-token'")
			return 1
		}

		if token, err := c.aws.createAWSBearerToken(); err != nil {
			c.UI.Error(fmt.Sprintf("Error with aws-iam auth method: %s", err))
			return 1
		} else {
			c.bearerToken = token
		}
	} else if c.bearerTokenFile == "" {
		c.UI.Error("Missing required '-bearer-token-file' flag")
		return 1
	} else {
		data, err := os.ReadFile(c.bearerTokenFile)
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.bearerToken = strings.TrimSpace(string(data))

		if c.bearerToken == "" {
			c.UI.Error(fmt.Sprintf("No bearer token found in %s", c.bearerTokenFile))
			return 1
		}
	}

	// Ensure that we don't try to use a token when performing a login
	// operation.
	c.http.SetToken("")
	c.http.SetTokenFile("")

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Do the login.
	req := &api.ACLLoginParams{
		AuthMethod:  c.authMethodName,
		BearerToken: c.bearerToken,
		Meta:        c.meta,
	}
	tok, _, err := client.ACL().Login(req, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error logging in: %s", err))
		return 1
	}

	if err := c.writeToSink(tok); err != nil {
		c.UI.Error(fmt.Sprintf("Error writing token to file sink: %s", err))
		return 1
	}

	return 0
}

func (c *cmd) writeToSink(tok *api.ACLToken) error {
	payload := []byte(tok.SecretID)
	return file.WriteAtomicWithPerms(c.tokenSinkFile, payload, 0o755, 0o600)
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Login to Consul using an auth method"

const help = `
Usage: consul login [options]

  The login command will exchange the provided third party credentials with the
  requested auth method for a newly minted Consul ACL token. The companion
  command 'consul logout' should be used to destroy any tokens created this way
  to avoid a resource leak.
`
