package authmethodupdate

import (
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/acl"
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

	name string

	description string

	k8sHost              string
	k8sCACert            string
	k8sServiceAccountJWT string

	noMerge  bool
	showMeta bool

	testStdin io.Reader
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
		&c.name,
		"name",
		"",
		"The auth method name.",
	)

	c.flags.StringVar(
		&c.description,
		"description",
		"",
		"A description of the auth method.",
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

	c.flags.BoolVar(&c.noMerge, "no-merge", false, "Do not merge the current auth method "+
		"information with what is provided to the command. Instead overwrite all fields "+
		"with the exception of the name which is immutable.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.name == "" {
		c.UI.Error(fmt.Sprintf("Cannot update an auth method without specifying the -name parameter"))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Regardless of merge, we need to fetch the prior immutable fields first.
	currentAuthMethod, _, err := client.ACL().AuthMethodRead(c.name, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error when retrieving current auth method: %v", err))
		return 1
	} else if currentAuthMethod == nil {
		c.UI.Error(fmt.Sprintf("Auth method not found with name %q", c.name))
		return 1
	}

	if c.k8sCACert != "" {
		c.k8sCACert, err = helpers.LoadDataSource(c.k8sCACert, c.testStdin)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Invalid '-kubernetes-ca-cert' value: %v", err))
			return 1
		} else if c.k8sCACert == "" {
			c.UI.Error(fmt.Sprintf("Kubernetes CA Cert is empty"))
			return 1
		}
	}

	var method *api.ACLAuthMethod
	if c.noMerge {
		method = &api.ACLAuthMethod{
			Name:        currentAuthMethod.Name,
			Type:        currentAuthMethod.Type,
			Description: c.description,
		}

		if currentAuthMethod.Type == "kubernetes" {
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

			method.Config = map[string]interface{}{
				"Host":              c.k8sHost,
				"CACert":            c.k8sCACert,
				"ServiceAccountJWT": c.k8sServiceAccountJWT,
			}
		}
	} else {
		methodCopy := *currentAuthMethod
		method = &methodCopy

		if c.description != "" {
			method.Description = c.description
		}
		if method.Config == nil {
			method.Config = make(map[string]interface{})
		}
		if currentAuthMethod.Type == "kubernetes" {
			if c.k8sHost != "" {
				method.Config["Host"] = c.k8sHost
			}
			if c.k8sCACert != "" {
				method.Config["CACert"] = c.k8sCACert
			}
			if c.k8sServiceAccountJWT != "" {
				method.Config["ServiceAccountJWT"] = c.k8sServiceAccountJWT
			}
		}
	}

	method, _, err = client.ACL().AuthMethodUpdate(method, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error updating auth method %q: %v", c.name, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Auth method updated successfully"))
	acl.PrintAuthMethod(method, c.UI, c.showMeta)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Update an ACL auth method"
const help = `
Usage: consul acl auth-method update -name NAME [options]

  Updates an auth method. By default it will merge the auth method
  information with its current state so that you do not have to provide all
  parameters. This behavior can be disabled by passing -no-merge.

  Update all editable fields of the auth method:

    $ consul acl auth-method update -name "my-k8s" \
                            -description "new description" \
                            -kubernetes-host "https://new-apiserver.example.com:8443" \
                            -kubernetes-ca-file /path/to/new-kube.ca.crt \
                            -kubernetes-service-account-jwt "NEW_JWT_CONTENTS"
`
