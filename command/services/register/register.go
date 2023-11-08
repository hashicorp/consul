// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package register

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/services"
	"github.com/mitchellh/cli"
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

	// flags
	flagKind            string
	flagId              string
	flagName            string
	flagAddress         string
	flagPort            int
	flagSocketPath      string
	flagTags            []string
	flagMeta            map[string]string
	flagTaggedAddresses map[string]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagId, "id", "",
		"ID of the service to register for arg-based registration. If this "+
			"isn't set, it will default to the -name value.")
	c.flags.StringVar(&c.flagName, "name", "",
		"Name of the service to register for arg-based registration.")
	c.flags.StringVar(&c.flagAddress, "address", "",
		"Address of the service to register for arg-based registration.")
	c.flags.IntVar(&c.flagPort, "port", 0,
		"Port of the service to register for arg-based registration.")
	c.flags.StringVar(&c.flagSocketPath, "socket", "",
		"Path to the Unix domain socket to register for arg-based registration (conflicts with address and port).")
	c.flags.Var((*flags.FlagMapValue)(&c.flagMeta), "meta",
		"Metadata to set on the service, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")
	c.flags.Var((*flags.AppendSliceValue)(&c.flagTags), "tag",
		"Tag to add to the service. This flag can be specified multiple "+
			"times to set multiple tags.")
	c.flags.Var((*flags.FlagMapValue)(&c.flagTaggedAddresses), "tagged-address",
		"Tagged address to set on the service, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple addresses.")
	c.flags.StringVar(&c.flagKind, "kind", "", "The services 'kind'")

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

	var taggedAddrs map[string]api.ServiceAddress
	if len(c.flagTaggedAddresses) > 0 {
		taggedAddrs = make(map[string]api.ServiceAddress)
		for k, v := range c.flagTaggedAddresses {
			addr, err := api.ParseServiceAddr(v)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Invalid Tagged Address: %v", err))
				return 1
			}
			taggedAddrs[k] = addr
		}
	}

	svcs := []*api.AgentServiceRegistration{{
		Kind:            api.ServiceKind(c.flagKind),
		ID:              c.flagId,
		Name:            c.flagName,
		Address:         c.flagAddress,
		Port:            c.flagPort,
		SocketPath:      c.flagSocketPath,
		Tags:            c.flagTags,
		Meta:            c.flagMeta,
		TaggedAddresses: taggedAddrs,
	}}

	// Check for arg validation
	args = c.flags.Args()
	if len(args) == 0 && c.flagName == "" {
		c.UI.Error("Service registration requires at least one argument or flags.")
		return 1
	} else if len(args) > 0 && c.flagName != "" {
		c.UI.Error("Service registration requires arguments or -id, not both.")
		return 1
	}

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
		if err := client.Agent().ServiceRegister(svc); err != nil {
			c.UI.Error(fmt.Sprintf("Error registering service %q: %s",
				svc.Name, err))
			return 1
		}

		c.UI.Output(fmt.Sprintf("Registered service: %s", svc.Name))
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
	synopsis = "Register services with the local agent"
	help     = `
Usage: consul services register [options] [FILE...]

  Register one or more services using the local agent API. Services can
  be registered from standard Consul configuration files (HCL or JSON) or
  using flags. The service is registered and the command returns. The caller
  must remember to call "consul services deregister" or a similar API to
  deregister the service when complete.

      $ consul services register web.json

  Additional flags and more advanced use cases are detailed below.
`
)
