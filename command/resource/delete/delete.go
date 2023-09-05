// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
	"errors"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/command/resource"
	"github.com/hashicorp/consul/internal/resourcehcl"
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
	flags.Merge(c.flags, c.http.AddPeerName())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var gvk *api.GVK
	var resourceName string
	var opts *api.QueryOptions

	if len(args) == 0 {
		c.UI.Error("Please provide required arguments")
		return 1
	}

	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
	}

	if c.flags.Lookup("f").Value.String() != "" {
		if c.filePath != "" {
			data, err := helpers.LoadDataSourceNoRaw(c.filePath, nil)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
				return 1
			}
			parsedResource, err := resourcehcl.Unmarshal([]byte(data), consul.NewTypeRegistry())
			if err != nil {
				c.UI.Error(fmt.Sprintf("Failed to decode resource from input file: %v", err))
				return 1
			}

			gvk = &api.GVK{
				Group:   parsedResource.Id.Type.GetGroup(),
				Version: parsedResource.Id.Type.GetGroupVersion(),
				Kind:    parsedResource.Id.Type.GetKind(),
			}
			resourceName = parsedResource.Id.GetName()
			opts = &api.QueryOptions{
				Namespace: parsedResource.Id.Tenancy.GetNamespace(),
				Partition: parsedResource.Id.Tenancy.GetPartition(),
				Peer:      parsedResource.Id.Tenancy.GetPeerName(),
				Token:     c.http.Token(),
			}
		} else {
			c.UI.Error(fmt.Sprintf("Please provide an input file with resource definition"))
			return 1
		}
	} else {
		if len(args) < 2 {
			c.UI.Error("Must specify two arguments: resource type and resource name")
			return 1
		}
		var err error
		gvk, resourceName, err = resource.GetTypeAndResourceName(args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %s", err))
			return 1
		}

		inputArgs := args[2:]
		if err := c.flags.Parse(inputArgs); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
		if c.filePath != "" {
			c.UI.Error("You need to provide all information in the HCL file if provide its file path")
			return 1
		}
		opts = &api.QueryOptions{
			Namespace: c.http.Namespace(),
			Partition: c.http.Partition(),
			Peer:      c.http.PeerName(),
			Token:     c.http.Token(),
		}
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	if err := client.Resource().Delete(gvk, resourceName, opts); err != nil {
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
type, name, partition, namespace and peer and outputs its JSON representation.

consul resource delete [type] [name] -partition=<default> -namespace=<default> -peer=<local>
consul resource delete -f [resource_file_path]

But you could only use one of the approaches.

Example:

$ consul resource delete catalog.v1alpha1.Service card-processor -partition=billing -namespace=payments -peer=eu
$ consul resource delete -f resource.hcl

In resource.hcl, it could be:
ID {
  Type = gvk("catalog.v1alpha1.Service")
  Name = "card-processor"
  Tenancy {
    Namespace = "payments"
    Partition = "billing"
    PeerName = "eu"
  }
}
`
