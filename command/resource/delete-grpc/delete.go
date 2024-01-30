// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
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
	UI            cli.Ui
	flags         *flag.FlagSet
	grpcFlags     *client.GRPCFlags
	resourceFlags *client.ResourceFlags
	help          string

	filePath string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")

	c.grpcFlags = &client.GRPCFlags{}
	c.resourceFlags = &client.ResourceFlags{}
	client.MergeFlags(c.flags, c.grpcFlags.ClientFlags())
	client.MergeFlags(c.flags, c.resourceFlags.ResourceFlags())
	c.help = client.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var resourceType *pbresource.Type
	var resourceTenancy *pbresource.Tenancy
	var resourceName string

	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
		c.UI.Error(fmt.Sprintf("Failed to run delete command: %v", err))
		return 1
	}

	// collect resource type, name and tenancy
	if c.flags.Lookup("f").Value.String() != "" {
		if c.filePath == "" {
			c.UI.Error(fmt.Sprintf("Please provide an input file with resource definition"))
			return 1
		}
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
		resourceName = parsedResource.Id.Name
	} else {
		var err error
		resourceType, resourceName, err = resource.GetTypeAndResourceName(args)
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
		resourceTenancy = &pbresource.Tenancy{
			Partition: c.resourceFlags.Partition(),
			Namespace: c.resourceFlags.Namespace(),
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

	// delete resource
	res := resource.ResourceGRPC{C: resourceClient}
	err = res.Delete(resourceType, resourceTenancy, resourceName)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting resource %s/%s: %v", resourceType, resourceName, err))
		return 1
	}

	c.UI.Info(fmt.Sprintf("%s.%s.%s/%s deleted", resourceType.Group, resourceType.GroupVersion, resourceType.Kind, resourceName))
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

$ consul resource delete catalog.v2beta1.Service card-processor -partition=billing -namespace=payments -peer=eu
$ consul resource delete -f resource.hcl

In resource.hcl, it could be:
ID {
	Type = gvk("catalog.v2beta1.Service")
	Name = "card-processor"
	Tenancy {
		Partition = "billing"
		Namespace = "payments"
		PeerName = "eu"
	}
}
`
