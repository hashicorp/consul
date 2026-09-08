// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package get

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
	if len(c.flags.Args()) != 1 {
		c.UI.Error("Exactly one feature name is required")
		return 1
	}
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	feature, _, err := client.Operator().FeatureGateGet(c.flags.Arg(0), &api.QueryOptions{AllowStale: c.http.Stale()})
	if err != nil {
		// Surface a friendlier message when the leader has not yet initialized state.
		c.UI.Error(fmt.Sprintf("Error querying feature gate: %s", err))
		c.UI.Warn("If this is a new cluster, the feature-gate policy may not be initialized yet. Wait for the leader to complete reconciliation and retry.")
		return 1
	}
	out, err := operfeature.Format([]api.FeatureGate{*feature}, c.format)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(out)
	return 0
}

func (c *cmd) Synopsis() string { return "Get a cluster feature gate" }
func (c *cmd) Help() string     { return c.help }

const help = `
Usage: consul operator feature get [options] <name>

  Displays desired, eligibility, and effective state for one feature gate.
`
