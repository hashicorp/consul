// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

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
	UI        cli.Ui
	flags     *flag.FlagSet
	grpcFlags *client.GRPCFlags
	help      string

	filePath string
	prefix   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")
	c.flags.StringVar(&c.prefix, "p", "",
		"Name prefix for listing resources if you need ambiguous match")

	c.grpcFlags = &client.GRPCFlags{}
	client.MergeFlags(c.flags, c.grpcFlags.ClientFlags())
	c.help = client.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var resourceType *pbresource.Type
	var resourceTenancy *pbresource.Tenancy

	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
		c.UI.Error(fmt.Sprintf("Failed to run apply command: %v", err))
		return 1
	}

	// collect resource type, name and tenancy
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

			resourceType = parsedResource.Id.Type
			resourceTenancy = parsedResource.Id.Tenancy
		} else {
			c.UI.Error(fmt.Sprintf("Please provide an input file with resource definition"))
			return 1
		}
	} else {
		var err error
		//TODO fix the have to provide resource name error
		resourceType, _, err = resource.GetTypeAndResourceName(args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Incorrect argument format: %s", err))
			return 1
		}

		inputArgs := args[2:]
		fmt.Printf("inputArgs: %v\n", inputArgs)
		err = resource.ParseInputParams(inputArgs, c.flags)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error parsing input arguments: %v", err))
			return 1
		}
		if c.filePath != "" {
			c.UI.Error("Incorrect argument format: File argument is not needed when resource information is provided with the command")
			return 1
		}
		resourceTenancy = &pbresource.Tenancy{
			Namespace: c.grpcFlags.Namespace(),
			Partition: c.grpcFlags.Partition(),
			PeerName:  c.grpcFlags.Peername(),
		}
	}

	// initialize client
	config, err := client.LoadGRPCConfig(nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error loading config: %s", err))
		return 1
	}
	c.grpcFlags.MergeFlagsIntoGRPCConfig(config)
	resourceClient, err := client.NewGRPCClient(config)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	// list resource
	res := resource.ResourceGRPC{C: resourceClient}
	entry, err := res.List(resourceType, resourceTenancy, c.prefix, c.grpcFlags.Stale())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing resource %s/%s: %v", resourceType, c.prefix, err))
		return 1
	}

	// display response
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

const synopsis = "Lists all resources by name prefix"
const help = `
Usage: consul resource list [type] -partition=<default> -namespace=<default> -peer=<local>
or
consul resource list -f [path/to/file.hcl]

Lists all the resources specified by the type under the given partition, namespace and peer
and outputs in JSON format.

Example:

$ consul resource list catalog.v2beta1.Service -p=card -partition=billing -namespace=payments -peer=eu

$ consul resource list -f=demo.hcl -p=card

Sample demo.hcl:

ID {
	Type = gvk("group.version.kind")
	Tenancy {
	  Namespace = "default"
	  Partition = "default"
	  PeerName = "local"
	}
  }
`
