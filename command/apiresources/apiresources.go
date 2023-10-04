// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apiresources

import (
	"encoding/json"
	"flag"

	"github.com/hashicorp/consul/command/cli"

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
	help  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	kindToGVKMap := resource.BuildKindToGVKMap()
	b, err := json.MarshalIndent(kindToGVKMap, "", "    ")
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(string(b))
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Reads resource map keyed with the kind and valued with the GVK"
const help = `
Usage: consul api-resources

Lists all the resources map whose keys are resource kind and values are GVK format.
User could use the kind as abbreviation to refer to the resource GVK.
`
