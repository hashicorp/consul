// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package deregister

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/services"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	flagId string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagId, "id", "",
		"ID to delete. This must not be set if arguments are given.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Check for arg validation
	args = c.flags.Args()
	if len(args) == 0 && c.flagId == "" {
		c.UI.Error("Service deregistration requires at least one argument or -id.")
		return 1
	} else if len(args) > 0 && c.flagId != "" {
		c.UI.Error("Service deregistration requires arguments or -id, not both.")
		return 1
	}

	svcs := []*api.AgentServiceRegistration{{
		ID: c.flagId,
	}}
	if len(args) > 0 {
		var err error
		svcs, err = services.ServicesFromFiles(c.UI, args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error: %s", err))
			return 1
		}
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Create all the services
	for _, svc := range svcs {
		id := svc.ID
		if id == "" {
			id = svc.Name
		}
		if id == "" {
			continue
		}

		if err := client.Agent().ServiceDeregister(id); err != nil {
			c.UI.Error(fmt.Sprintf("Error deregistering service %q: %s",
				svc.Name, err))
			return 1
		}

		c.UI.Output(fmt.Sprintf("Deregistered service: %s", id))
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Deregister services with the local agent"
	help     = `
Usage: consul services deregister [options] [FILE...]

  Deregister one or more services that were previously registered with
  the local agent.

      $ consul services deregister web.json db.json

  The -id flag may be used to deregister a single service by ID:

      $ consul services deregister -id=web

  Services are deregistered from the local agent catalog. This command must
  be run against the same agent where the service was registered.
`
)
