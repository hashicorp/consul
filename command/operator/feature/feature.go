// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package feature

import (
	"encoding/json"
	"fmt"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
)

const (
	PrettyFormat = "pretty"
	JSONFormat   = "json"
)

func New() *cmd { return &cmd{} }

type cmd struct{}

func (c *cmd) Run([]string) int { return cli.RunResultHelp }
func (c *cmd) Synopsis() string { return "Inspect and update cluster feature gates" }
func (c *cmd) Help() string     { return flags.Usage(help, nil) }

const help = `
Usage: consul operator feature <subcommand> [options]

  Inspect or update cluster-wide feature gates. A token with operator
  privileges is required when ACLs are enabled.
`

// Format renders feature-gate state in Consul's conventional pretty or JSON
// formats.
func Format(features []api.FeatureGate, format string) (string, error) {
	switch format {
	case PrettyFormat:
		rows := []string{"Name|Stage|Desired|Effective|Eligible|Source|Reason|Min Version|Policy Index|Status Index"}
		for _, f := range features {
			rows = append(rows, fmt.Sprintf(
				"%s|%s|%t|%t|%t|%s|%s|%s|%d|%d",
				f.Name, f.Stage, f.DesiredEnabled, f.EffectiveEnabled, f.Eligible,
				f.Source, f.Reason, f.MinVersion, f.PolicyIndex, f.StatusIndex,
			))
		}
		return columnize.SimpleFormat(rows), nil
	case JSONFormat:
		out, err := json.MarshalIndent(features, "", "  ")
		return string(out), err
	default:
		return "", fmt.Errorf("unknown format %q (expected %s or %s)", format, PrettyFormat, JSONFormat)
	}
}
