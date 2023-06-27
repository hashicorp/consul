// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package event

import (
	"flag"
	"fmt"
	"regexp"

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
	UI      cli.Ui
	flags   *flag.FlagSet
	http    *flags.HTTPFlags
	name    string
	node    string
	service string
	tag     string
	help    string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.name, "name", "",
		"Name of the event.")
	c.flags.StringVar(&c.node, "node", "",
		"Regular expression to filter on node names.")
	c.flags.StringVar(&c.service, "service", "",
		"Regular expression to filter on service instances.")
	c.flags.StringVar(&c.tag, "tag", "",
		"Regular expression to filter on service tags. Must be used with -service.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Check for a name
	if c.name == "" {
		c.UI.Error("Event name must be specified")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Validate the filters
	if c.node != "" {
		if _, err := regexp.Compile(c.node); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile node filter regexp: %v", err))
			return 1
		}
	}
	if c.service != "" {
		if _, err := regexp.Compile(c.service); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile service filter regexp: %v", err))
			return 1
		}
	}
	if c.tag != "" {
		if _, err := regexp.Compile(c.tag); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile tag filter regexp: %v", err))
			return 1
		}
	}
	if c.tag != "" && c.service == "" {
		c.UI.Error("Cannot provide tag filter without service filter.")
		return 1
	}

	// Check for a payload
	var payload []byte
	args = c.flags.Args()
	switch len(args) {
	case 0:
	case 1:
		payload = []byte(args[0])
	default:
		c.UI.Error("Too many command line arguments.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Agent().NodeName()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	// Prepare the request
	event := client.Event()
	params := &api.UserEvent{
		Name:          c.name,
		Payload:       payload,
		NodeFilter:    c.node,
		ServiceFilter: c.service,
		TagFilter:     c.tag,
	}

	// Fire the event
	id, _, err := event.Fire(params, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error firing event: %s", err))
		return 1
	}

	// Write out the ID
	c.UI.Output(fmt.Sprintf("Event ID: %s", id))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Fire a new event"
const help = `
Usage: consul event [options] [payload]

  Dispatches a custom user event across a datacenter. An event must provide
  a name, but a payload is optional. Events support filtering using
  regular expressions on node name, service, and tag definitions.
`
