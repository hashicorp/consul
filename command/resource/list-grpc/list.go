// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

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
	prefix   string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")
	c.flags.StringVar(&c.prefix, "p", "",
		"Name prefix for listing resources if you need ambiguous match")

	c.grpcFlags = &client.GRPCFlags{}
	c.resourceFlags = &client.ResourceFlags{}
	client.MergeFlags(c.flags, c.grpcFlags.ClientFlags())
	client.MergeFlags(c.flags, c.resourceFlags.ResourceFlags())
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
		c.UI.Error(fmt.Sprintf("Failed to run list command: %v", err))
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
	} else {
		var err error
		args := c.flags.Args()
		if err = validateArgs(args); err != nil {
			c.UI.Error(fmt.Sprintf("Incorrect argument format: %s", err))
			return 1
		}
		resourceType, err = resource.InferTypeFromResourceType(args[0])
		if err != nil {
			c.UI.Error(fmt.Sprintf("Incorrect argument format: %s", err))
			return 1
		}

		// skip resource type to parse remaining args
		inputArgs := c.flags.Args()[1:]
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

	// list resource
	res := resource.ResourceGRPC{C: resourceClient}
	entry, err := res.List(resourceType, resourceTenancy, c.prefix, c.resourceFlags.Stale())
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing resource %s/%s: %v", resourceType, c.prefix, err))
		return 1
	}

	// display response
	b, err := json.MarshalIndent(entry, "", resource.JSON_INDENT)
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(string(b))
	return 0
}

func validateArgs(args []string) error {
	if args == nil {
		return fmt.Errorf("Must include resource type or flag arguments")
	}
	if len(args) < 1 {
		return fmt.Errorf("Must include resource type argument")
	}
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		return fmt.Errorf("Must include flag arguments after resource type")
	}
	return nil
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
		Partition = "default"
		Namespace = "default"
		PeerName = "local"
	}
  }
`
