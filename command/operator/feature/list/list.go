// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package list

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	operfeature "github.com/hashicorp/consul/command/operator/feature"
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
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.format, "format", operfeature.PrettyFormat, "Output format {pretty|json}")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
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
	if len(c.flags.Args()) != 0 {
		c.UI.Error("This command takes no positional arguments")
		return 1
	}
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	features, _, err := client.Operator().FeatureGateList(&api.QueryOptions{AllowStale: c.http.Stale()})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying feature gates: %s", err))
		return 1
	}
	if len(features) == 0 {
		c.UI.Warn("No feature gates found. The leader may not have initialized the feature-gate policy yet; retry in a few seconds.")
		return 0
	}
	out, err := operfeature.Format(features, c.format)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(out)
	return 0
}

func (c *cmd) Synopsis() string { return "List cluster feature gates" }
func (c *cmd) Help() string     { return c.help }

const help = `
Usage: consul operator feature list [options]

  Lists desired and effective state for every registered feature gate.
`
