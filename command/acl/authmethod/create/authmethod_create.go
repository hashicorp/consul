// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authmethodcreate

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl/authmethod"
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

	authMethodType string
	name           string
	displayName    string
	description    string
	maxTokenTTL    time.Duration
	tokenLocality  string
	config         string

	k8sHost              string
	k8sCACert            string
	k8sServiceAccountJWT string

	showMeta bool
	format   string

	testStdin io.Reader

	enterpriseCmd
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.BoolVar(
		&c.showMeta,
		"meta",
		false,
		"Indicates that auth method metadata such "+
			"as the raft indices should be shown for each entry.",
	)

	c.flags.StringVar(
		&c.authMethodType,
		"type",
		"",
		"The new auth method's type. This flag is required.",
	)
	c.flags.StringVar(
		&c.name,
		"name",
		"",
		"The new auth method's name. This flag is required.",
	)
	c.flags.StringVar(
		&c.displayName,
		"display-name",
		"",
		"An optional name to use instead of the name when displaying this auth method in a UI.",
	)
	c.flags.StringVar(
		&c.description,
		"description",
		"",
		"A description of the auth method.",
	)
	c.flags.DurationVar(
		&c.maxTokenTTL,
		"max-token-ttl",
		0,
		"Duration of time all tokens created by this auth method should be valid for",
	)
	c.flags.StringVar(
		&c.tokenLocality,
		"token-locality",
		"",
		"Defines the kind of token that this auth method should produce. "+
			"This can be either 'local' or 'global'. If empty the value of 'local' is assumed.",
	)

	c.flags.StringVar(
		&c.k8sHost,
		"kubernetes-host",
		"",
		"Address of the Kubernetes API server. This flag is required for type=kubernetes.",
	)
	c.flags.StringVar(
		&c.k8sCACert,
		"kubernetes-ca-cert",
		"",
		"PEM encoded CA cert for use by the TLS client used to talk with the "+
			"Kubernetes API. May be prefixed with '@' to indicate that the "+
			"value is a file path to load the cert from. "+
			"This flag is required for type=kubernetes.",
	)
	c.flags.StringVar(
		&c.k8sServiceAccountJWT,
		"kubernetes-service-account-jwt",
		"",
		"A Kubernetes service account JWT used to access the TokenReview API to "+
			"validate other JWTs during login. "+
			"This flag is required for type=kubernetes.",
	)
	c.flags.StringVar(
		&c.format,
		"format",
		authmethod.PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(authmethod.GetSupportedFormats(), "|")),
	)
	c.flags.StringVar(
		&c.config,
		"config",
		"",
		"The configuration for the auth method. Must be JSON. May be prefixed with '@' "+
			"to indicate that the value is a file path to load the config from. '-' may also be "+
			"given to indicate that the config is available on stdin",
	)

	c.initEnterpriseFlags()

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.authMethodType == "" {
		c.UI.Error(fmt.Sprintf("Missing required '-type' flag"))
		c.UI.Error(c.Help())
		return 1
	} else if c.name == "" {
		c.UI.Error(fmt.Sprintf("Missing required '-name' flag"))
		c.UI.Error(c.Help())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	newAuthMethod := &api.ACLAuthMethod{
		Type:          c.authMethodType,
		Name:          c.name,
		DisplayName:   c.displayName,
		Description:   c.description,
		TokenLocality: c.tokenLocality,
	}
	if c.maxTokenTTL > 0 {
		newAuthMethod.MaxTokenTTL = c.maxTokenTTL
	}

	if err := c.enterprisePopulateAuthMethod(newAuthMethod); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.config != "" {
		if c.k8sHost != "" || c.k8sCACert != "" || c.k8sServiceAccountJWT != "" {
			c.UI.Error(fmt.Sprintf("Cannot use command line arguments with '-config' flags"))
			return 1
		}
		data, err := helpers.LoadDataSource(c.config, c.testStdin)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error loading configuration file: %v", err))
			return 1
		}
		err = json.Unmarshal([]byte(data), &newAuthMethod.Config)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error parsing JSON configuration file: %v", err))
			return 1
		}

	}

	if c.authMethodType == "kubernetes" {
		if c.k8sHost == "" {
			c.UI.Error(fmt.Sprintf("Missing required '-kubernetes-host' flag"))
			return 1
		} else if c.k8sCACert == "" {
			c.UI.Error(fmt.Sprintf("Missing required '-kubernetes-ca-cert' flag"))
			return 1
		} else if c.k8sServiceAccountJWT == "" {
			c.UI.Error(fmt.Sprintf("Missing required '-kubernetes-service-account-jwt' flag"))
			return 1
		}

		c.k8sCACert, err = helpers.LoadDataSource(c.k8sCACert, c.testStdin)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Invalid '-kubernetes-ca-cert' value: %v", err))
			return 1
		} else if c.k8sCACert == "" {
			c.UI.Error(fmt.Sprintf("Kubernetes CA Cert is empty"))
			return 1
		}

		newAuthMethod.Config = map[string]interface{}{
			"Host":              c.k8sHost,
			"CACert":            c.k8sCACert,
			"ServiceAccountJWT": c.k8sServiceAccountJWT,
		}
	}

	method, _, err := client.ACL().AuthMethodCreate(newAuthMethod, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create new auth method: %v", err))
		return 1
	}

	formatter, err := authmethod.NewFormatter(c.format, c.showMeta)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	out, err := formatter.FormatAuthMethod(method)
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

const synopsis = "Create an ACL auth method"

const help = `
Usage: consul acl auth-method create -name NAME -type TYPE [options]

  Create a new auth method:

    $ consul acl auth-method create -type "kubernetes" \
                            -name "my-k8s" \
                            -description "This is an example kube method" \
                            -kubernetes-host "https://apiserver.example.com:8443" \
                            -kubernetes-ca-cert @/path/to/kube.ca.crt \
                            -kubernetes-service-account-jwt "JWT_CONTENTS"
`
