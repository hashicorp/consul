// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apply

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/resource"
	"github.com/hashicorp/consul/command/resource/client"
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

	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")

	c.grpcFlags = &client.GRPCFlags{}
	client.MergeFlags(c.flags, c.grpcFlags.ClientFlags())
	c.help = client.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
		c.UI.Error(fmt.Sprintf("Failed to run apply command: %v", err))
		return 1
	}

	// parse resource
	input := c.filePath
	if input == "" {
		c.UI.Error("Required '-f' flag was not provided to specify where to load the resource content from")
		return 1
	}
	parsedResource, err := resource.ParseResourceInput(input, c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to decode resource from input file: %v", err))
		return 1
	}
	if parsedResource == nil {
		c.UI.Error("Unable to parse the file argument")
		return 1
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

	// write resource
	res := resource.ResourceGRPC{C: resourceClient}
	entry, err := res.Apply(parsedResource)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing resource %s/%s: %v", parsedResource.Id.Type, parsedResource.Id.GetName(), err))
		return 1
	}

	// display response
	b, err := json.MarshalIndent(entry, "", resource.JSON_INDENT)
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}
	c.UI.Info(fmt.Sprintf("%s.%s.%s '%s' created.", parsedResource.Id.Type.Group, parsedResource.Id.Type.GroupVersion, parsedResource.Id.Type.Kind, parsedResource.Id.GetName()))
	c.UI.Info(string(b))

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return client.Usage(c.help, nil)
}

const synopsis = "Writes/updates resource information"

const help = `
Usage: consul resource apply [options] <resource>

	Write and/or update a resource by providing the definition. The configuration
	argument is either a file path or '-' to indicate that the resource
    should be read from stdin. The data should be either in HCL or
	JSON form.

	Example (with flag):

	$ consul resource apply -f=demo.hcl

	Example (from stdin):

	$ consul resource apply -f - < demo.hcl

	Sample demo.hcl:

	ID {
		Type = gvk("group.version.kind")
		Name = "resource-name"
		Tenancy {
			Partition = "default"
			Namespace = "default"
			PeerName = "local"
		}
	}

	Data {
		Name = "demo"
	}

	Metadata = {
		"foo" = "bar"
	}
`
