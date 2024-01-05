// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delete

import (
	"errors"
	"flag"
	"fmt"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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
	filename    string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.filename, "filename", "", "The filename of the config entry to delete")
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

	kind := c.kind
	name := c.name
	var err error
	if c.filename != "" {
		data, err := helpers.LoadDataSourceNoRaw(c.filename, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
			return 1
		}

		entry, err := helpers.ParseConfigEntry(data)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to decode config entry input: %v", err))
			return 1
		}
		kind = entry.GetKind()
		name = entry.GetName()
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}
	entries := client.ConfigEntries()

	var deleted bool
	if c.cas {
		deleted, _, err = entries.DeleteCAS(kind, name, c.modifyIndex, nil)
	} else {
		_, err = entries.Delete(kind, name, nil)
		deleted = err == nil
	}

	if err != nil {
		c.UI.Error(fmt.Sprintf("Error deleting config entry %s/%s: %v", kind, name, err))
		return 1
	}

	if !deleted {
		c.UI.Error(fmt.Sprintf("Config entry not deleted: %s/%s", kind, name))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Config entry deleted: %s/%s", kind, name))
	return 0
}

func (c *cmd) validateArgs() error {
	count := 0
	if c.filename != "" {
		count++
	}

	if c.kind != "" {
		count++
	}

	if c.name != "" {
		count++
	}

	if count >= 3 {
		return errors.New("filename can't be used with kind or name")
	} else if count == 0 {
		return errors.New("Must specify the -kind or -filename parameter")
	}

	if c.filename != "" {
		if count == 2 {
			return errors.New("filename can't be used with kind or name")
		}
	} else {
		if c.kind == "" {
			return errors.New("Must specify the -kind parameter")
		}

		if c.name == "" {
			return errors.New("Must specify the -name parameter")
		}
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
Usage: consul config delete [options] ([-kind <config kind> -name <config name>] | [-f FILENAME])

  Deletes the configuration entry specified by the kind and name.

  Example:

    $ consul config delete -kind service-defaults -name web
    $ consul config delete -filename service-defaults-web.hcl
`
)
