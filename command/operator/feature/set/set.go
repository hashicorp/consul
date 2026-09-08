// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package set

import (
	"flag"
	"fmt"
	"strconv"

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
	casRaw string // raw string from -cas flag; parsed in Run
	cas    uint64
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.format, "format", operfeature.PrettyFormat, "Output format {pretty|json}")
	// -cas is registered as a string so that an empty or non-numeric value
	// (e.g. shell expansion of a jq null) is handled gracefully rather than
	// producing a hard parse error. It is converted to uint64 below.
	c.flags.StringVar(&c.casRaw, "cas", "", "Only apply when the current policy index matches this value (0 = no CAS)")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	posArgs, err := parseInterspersed(c.flags, args)
	if err == flag.ErrHelp {
		return 0
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}
	// Parse -cas value now that flags are resolved. An empty, zero, or
	// non-numeric value (e.g. shell-expanded "null" from jq) means no CAS.
	if c.casRaw != "" && c.casRaw != "null" {
		c.cas, err = strconv.ParseUint(c.casRaw, 10, 64)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Invalid -cas value %q: expected a policy index integer", c.casRaw))
			return 1
		}
	}
	if len(posArgs) != 2 {
		c.UI.Error("A feature name and enabled|disabled value are required")
		return 1
	}
	enabled, err := parseBoolValue(posArgs[1])
	if err != nil {
		c.UI.Error(fmt.Sprintf("Invalid enabled value %q: expected enabled, disabled, true, or false", posArgs[1]))
		return 1
	}
	name := posArgs[0]
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}
	result, err := client.Operator().FeatureGateSet(name, enabled, c.cas, &api.WriteOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error updating feature gate: %s", err))
		return 1
	}
	if !result.Applied {
		c.UI.Error(fmt.Sprintf("Feature gate was not updated because policy index %d did not match", c.cas))
		return 1
	}
	out, err := operfeature.Format([]api.FeatureGate{result.Feature}, c.format)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Info(out)
	return 0
}

func (c *cmd) Synopsis() string { return "Set a cluster feature gate" }
func (c *cmd) Help() string     { return c.help }

const help = `
Usage: consul operator feature set [options] <name> <enabled|disabled>

  Records explicit operator intent and atomically updates the cluster-wide
  effective state. Use -cas to fence concurrent policy changes.

  Flags may appear anywhere relative to the positional arguments:

    consul operator feature set -cas=<index> <name> enabled
    consul operator feature set <name> enabled -cas=<index>
    consul operator feature set <name> enabled              (no CAS)

  The value must be "enabled" or "disabled" (or "true"/"false").

  Shell one-liner with automatic CAS (use the single-object /feature/ endpoint,
  not the /features/ list, to get a plain integer from jq):

    INDEX=$(consul operator feature get api-gateway-upstream-routing -format=json | jq .PolicyIndex)
    consul operator feature set api-gateway-upstream-routing enabled -cas=$INDEX
`

// parseInterspersed parses fs against args, allowing flags to appear anywhere
// among positional arguments. It returns the non-flag positional tokens.
//
// Go's flag.FlagSet stops at the first non-flag token. We work around that by
// repeatedly offering the remaining tokens to Parse: when Parse stops at a
// positional, we harvest it, skip over it, and continue until all tokens are
// consumed.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	remaining := args
	for len(remaining) > 0 {
		if err := fs.Parse(remaining); err != nil {
			return nil, err
		}
		// fs.Args() is what Parse left unconsumed (stopped at first positional).
		leftover := fs.Args()
		if len(leftover) == 0 {
			break
		}
		// The first leftover token is the positional that caused the stop.
		pos = append(pos, leftover[0])
		remaining = leftover[1:]
	}
	return pos, nil
}

// parseBoolValue accepts "enabled"/"disabled" per the plan's CLI contract as
// well as "true"/"false" for convenience.
func parseBoolValue(s string) (bool, error) {
	switch s {
	case "enabled":
		return true, nil
	case "disabled":
		return false, nil
	default:
		return strconv.ParseBool(s)
	}
}
