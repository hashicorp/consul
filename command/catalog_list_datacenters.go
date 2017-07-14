package command

import (
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

var _ cli.Command = (*CatalogListDatacentersCommand)(nil)

type CatalogListDatacentersCommand struct {
	BaseCommand
}

func (c *CatalogListDatacentersCommand) Help() string {
	helpText := `
Usage: consul catalog datacenters [options]

  Retrieves the list of all known datacenters. This datacenters are sorted in
  ascending order based on the estimated median round trip time from the servers
  in this datacenter to the servers in the other datacenters.

  To retrieve the list of datacenters:

      $ consul catalog datacenters

  For a full list of options and examples, please see the Consul documentation.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *CatalogListDatacentersCommand) Run(args []string) int {
	f := c.BaseCommand.NewFlagSet(c)

	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	if l := len(f.Args()); l > 0 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 0, got %d)", l))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	dcs, err := client.Catalog().Datacenters()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error listing datacenters: %s", err))
	}

	for _, dc := range dcs {
		c.UI.Info(dc)
	}

	return 0
}

func (c *CatalogListDatacentersCommand) Synopsis() string {
	return "Lists all known datacenters"
}
