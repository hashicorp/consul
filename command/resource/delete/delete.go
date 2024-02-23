// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
	"errors"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/resource"
	"github.com/hashicorp/consul/command/resource/client"
	"github.com/hashicorp/consul/proto-public/pbresource"
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

	filePath string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	c.flags.StringVar(&c.filePath, "f", "", "File path with resource definition")
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	// TODO(peering/v2) add back ability to query peers
	// flags.Merge(c.flags, c.http.AddPeerName())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var gvk *resource.GVK
	var resourceName string
	var opts *client.QueryOptions

	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
	}

	if c.flags.Lookup("f").Value.String() != "" {
		if c.filePath != "" {
			parsedResource, err := resource.ParseResourceFromFile(c.filePath)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Failed to decode resource from input file: %v", err))
				return 1
			}

			if parsedResource == nil {
				c.UI.Error("Unable to parse the file argument")
				return 1
			}

			gvk = &resource.GVK{
				Group:   parsedResource.Id.Type.GetGroup(),
				Version: parsedResource.Id.Type.GetGroupVersion(),
				Kind:    parsedResource.Id.Type.GetKind(),
			}
			resourceName = parsedResource.Id.GetName()
			opts = &client.QueryOptions{
				Namespace: parsedResource.Id.Tenancy.GetNamespace(),
				Partition: parsedResource.Id.Tenancy.GetPartition(),
				Token:     c.http.Token(),
			}
		} else {
			c.UI.Error(fmt.Sprintf("Please provide an input file with resource definition"))
			return 1
		}
	} else {
		var err error
		var resourceType *pbresource.Type
		resourceType, resourceName, err = resource.GetTypeAndResourceName(args)
		gvk = &resource.GVK{
			Group:   resourceType.GetGroup(),
			Version: resourceType.GetGroupVersion(),
			Kind:    resourceType.GetKind(),
		}
		if err != nil {
			c.UI.Error(fmt.Sprintf("Incorrect argument format: %s", err))
			return 1
		}

		inputArgs := args[2:]
		err = resource.ParseInputParams(inputArgs, c.flags)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error parsing input arguments: %v", err))
			return 1
		}
		if c.filePath != "" {
			c.UI.Error("Incorrect argument format: File argument is not needed when resource information is provided with the command")
			return 1
		}
		opts = &client.QueryOptions{
			Namespace: c.http.Namespace(),
			Partition: c.http.Partition(),
			Token:     c.http.Token(),
		}
	}

	config := api.DefaultConfig()

	c.http.MergeOntoConfig(config)
	resourceClient, err := client.NewClient(config)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	res := resource.Resource{C: resourceClient}

	if err := res.Delete(gvk, resourceName, opts); err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting resource %s.%s.%s/%s: %v", gvk.Group, gvk.Version, gvk.Kind, resourceName, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("%s.%s.%s/%s deleted", gvk.Group, gvk.Version, gvk.Kind, resourceName))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Delete resource information"
const help = `
Usage: You have two options to delete the resource specified by the given
type, name, partition and namespace and outputs its JSON representation.

consul resource delete [type] [name] -partition=<default> -namespace=<default>
consul resource delete -f [resource_file_path]

But you could only use one of the approaches.

Example:

$ consul resource delete catalog.v2beta1.Service card-processor -partition=billing -namespace=payments
$ consul resource delete -f resource.hcl

In resource.hcl, it could be:
ID {
  Type = gvk("catalog.v2beta1.Service")
  Name = "card-processor"
  Tenancy {
    Namespace = "payments"
    Partition = "billing"
  }
}
`
