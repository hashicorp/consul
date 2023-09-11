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

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/resource"
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
			parsedResource, err := resource.ParseResourceFromFile(c.filePath)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Failed to decode resource from input file: %v", err))
				return 1
			}

			if parsedResource == nil {
				c.UI.Error("Unable to parse the file argument")
				return 1
			}

			gvk = &api.GVK{
				Group:   parsedResource.Id.Type.GetGroup(),
				Version: parsedResource.Id.Type.GetGroupVersion(),
				Kind:    parsedResource.Id.Type.GetKind(),
			}
			opts = &api.QueryOptions{
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
		var err error
		// extract resource type
		gvk, err = getResourceType(c.flags.Args())
		if err != nil {
			c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %v", err))
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
			c.UI.Warn(fmt.Sprintf("File argument is ignored when resource definition is provided with the command"))
		}

		opts = &api.QueryOptions{
			Namespace:         c.http.Namespace(),
			Partition:         c.http.Partition(),
			Peer:              c.http.PeerName(),
			Token:             c.http.Token(),
			RequireConsistent: !c.http.Stale(),
		}
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entry, err := client.Resource().List(gvk, opts)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading resources for type %s: %v", gvk, err))
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

func getResourceType(args []string) (gvk *api.GVK, e error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("Must include resource type argument")
	}

	s := strings.Split(args[0], ".")
	if len(s) < 3 {
		return nil, fmt.Errorf("Must include resource type argument in group.verion.kind format")
	}
	gvk = &api.GVK{
		Group:   s[0],
		Version: s[1],
		Kind:    s[2],
	}

	return
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Reads all resources by type"
const help = `
Usage: consul resource list [type] -partition=<default> -namespace=<default> -peer=<local>
or
consul resource list -f [path/to/file.hcl]

Lists all the resources specified by the type under the given partition, namespace and peer
and outputs in JSON format.

Example:

$ consul resource list catalog.v1alpha1.Service card-processor -partition=billing -namespace=payments -peer=eu

$ consul resource list -f=demo.hcl

Sample demo.hcl:

ID {
	Type = gvk("group.version.kind")
	Name = "resource-name"
	Tenancy {
	  Namespace = "default"
	  Partition = "default"
	  PeerName = "local"
	}
  }
`
