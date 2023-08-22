// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package read

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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

	if args[0] == "-f" {
		if err := c.flags.Parse(args); err != nil {
			if err == flag.ErrHelp {
				return 0
			}
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
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
				Group: parsedResource.Id.Type.GetGroup(),
				Version: parsedResource.Id.Type.GetGroupVersion(),
				Kind: parsedResource.Id.Type.GetKind(),
			}
			resourceName = parsedResource.Id.GetName()
			opts = &api.QueryOptions{
				Namespace: parsedResource.Id.Tenancy.GetNamespace(),
				Partition: parsedResource.Id.Tenancy.GetPartition(),
				Peer:      parsedResource.Id.Tenancy.GetPeerName(),
				Token:     c.http.Token(),
			}
		} else {
			c.UI.Error("Please input file path")
			return 1
		}
	} else {
		var err error
		gvk, resourceName, err = getTypeAndResourceName(args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %s", err))
			return 1
		}

		inputArgs := args[2:]
		if err := c.flags.Parse(inputArgs); err != nil {
			if err == flag.ErrHelp {
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

	entry, err := client.Resource().Read(gvk, resourceName, opts)
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

func getTypeAndResourceName(args []string) (gvk *api.GVK, resourceName string, e error) {
	if len(args) < 2 {
		return nil, "", fmt.Errorf("Must specify two arguments: resource type and resource name")
	}

	s := strings.Split(args[0], ".")
	gvk = &api.GVK{
		Group:   s[0],
		Version: s[1],
		Kind:    s[2],
	}

	resourceName = args[1]
	return
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Read resource information"
const help = `
Usage: consul resource read [type] [name] -partition=<default> -namespace=<default> -peer=<local> -consistent=<false> -json

Reads the resource specified by the given type, name, partition, namespace, peer and reading mode
and outputs its JSON representation.

Example:

$ consul resource read catalog.v1alpha1.Service card-processor -partition=billing -namespace=payments -peer=eu
`
