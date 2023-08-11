// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package read

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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

	consistent bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.consistent, "consistent", false, "The reading mode.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	flags.Merge(c.flags, c.http.AddPeerName())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	args = c.flags.Args()
	gvk, resourceName, e := parseArgs(args)
	if e != nil {
		c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %s", e))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	opts := &api.QueryOptions{
		Namespace:         c.http.Namespace(),
		Partition:         c.http.Partition(),
		Peer:              c.http.PeerName(),
		Token:             c.http.Token(),
		RequireConsistent: c.consistent,
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

func parseArgs(args []string) (gvk *api.GVK, resourceName string, e error) {
	fmt.Println(args)
	if len(args) < 2 {
		return nil, "", fmt.Errorf("Must specify two arguments: resource types and resource name")
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
