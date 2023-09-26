// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package read

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/resource"
	"github.com/hashicorp/consul/command/resource/client"
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
				Namespace:         parsedResource.Id.Tenancy.GetNamespace(),
				Partition:         parsedResource.Id.Tenancy.GetPartition(),
				Peer:              parsedResource.Id.Tenancy.GetPeerName(),
				Token:             c.http.Token(),
				RequireConsistent: !c.http.Stale(),
			}
		} else {
			c.UI.Error(fmt.Sprintf("Please provide an input file with resource definition"))
			return 1
		}
	} else {
		if len(args) < 2 {
			c.UI.Error("Incorrect argument format: Must specify two arguments: resource type and resource name")
			return 1
		}
		var err error
		gvk, resourceName, err = resource.GetTypeAndResourceName(args)
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
			Namespace:         c.http.Namespace(),
			Partition:         c.http.Partition(),
			Peer:              c.http.PeerName(),
			Token:             c.http.Token(),
			RequireConsistent: !c.http.Stale(),
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

	entry, err := res.Read(gvk, resourceName, opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading resource %s/%s: %v", gvk, resourceName, err))
		return 1
	}

	b, err := json.MarshalIndent(entry, "", "    ")
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(string(b))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Read resource information"
const help = `
Usage: You have two options to read the resource specified by the given
type, name, partition, namespace and peer and outputs its JSON representation.

consul resource read [type] [name] -partition=<default> -namespace=<default> -peer=<local>
consul resource read -f [resource_file_path]

But you could only use one of the approaches.

Example:

$ consul resource read catalog.v2beta1.Service card-processor -partition=billing -namespace=payments -peer=eu
$ consul resource read -f resource.hcl

In resource.hcl, it could be:
ID {
  Type = gvk("catalog.v2beta1.Service")
  Name = "card-processor"
  Tenancy {
    Namespace = "payments"
    Partition = "billing"
    PeerName = "eu"
  }
}
`
