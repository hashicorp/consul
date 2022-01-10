package delete

import (
	"errors"
	"flag"
	"fmt"

	"github.com/hashicorp/consul/command/flags"
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

	kind        string
	name        string
	cas         bool
	modifyIndex uint64
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.kind, "kind", "", "The kind of configuration to delete.")
	c.flags.StringVar(&c.name, "name", "", "The name of configuration to delete.")
	c.flags.BoolVar(&c.cas, "cas", false,
		"Perform a Check-And-Set operation. Specifying this value also "+
			"requires the -modify-index flag to be set. The default value "+
			"is false.")
	c.flags.Uint64Var(&c.modifyIndex, "modify-index", 0,
		"Unsigned integer representing the ModifyIndex of the config entry. "+
			"This is used in combination with the -cas flag.")
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

	if err := c.validateArgs(); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}
	entries := client.ConfigEntries()

	var deleted bool
	if c.cas {
		deleted, _, err = entries.DeleteCAS(c.kind, c.name, c.modifyIndex, nil)
	} else {
		_, err = entries.Delete(c.kind, c.name, nil)
		deleted = err == nil
	}

	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting config entry %s/%s: %v", c.kind, c.name, err))
		return 1
	}

	if !deleted {
		c.UI.Error(fmt.Sprintf("Config entry not deleted: %s/%s", c.kind, c.name))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Config entry deleted: %s/%s", c.kind, c.name))
	return 0
}

func (c *cmd) validateArgs() error {
	if c.kind == "" {
		return errors.New("Must specify the -kind parameter")
	}

	if c.name == "" {
		return errors.New("Must specify the -name parameter")
	}

	if c.cas && c.modifyIndex == 0 {
		return errors.New("Must specify a -modify-index greater than 0 with -cas")
	}

	if c.modifyIndex != 0 && !c.cas {
		return errors.New("Cannot specify -modify-index without -cas")
	}

	return nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const (
	synopsis = "Delete a centralized config entry"
	help     = `
Usage: consul config delete [options] -kind <config kind> -name <config name>

  Deletes the configuration entry specified by the kind and name.

  Example:

    $ consul config delete -kind service-defaults -name web
`
)
