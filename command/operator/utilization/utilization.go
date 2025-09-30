// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utilization

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	commandcli "github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/version"
	mcli "github.com/mitchellh/cli"
)

func New(ui mcli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    mcli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	message    string
	todayOnly  bool
	assumeSend bool
	outputPath string
	clientFn   func() (*api.Client, error)
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.message, "message", "", "Optional context that will be logged with the utilization export.")
	c.flags.BoolVar(&c.todayOnly, "today-only", false, "Include only the most recent utilization snapshot.")
	c.flags.BoolVar(&c.assumeSend, "y", false, "Automatically send the utilization report to HashiCorp.")
	c.flags.StringVar(&c.outputPath, "output", "", "Path to write the utilization bundle JSON. Defaults to consul-utilization-<timestamp>.json in the current directory.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	flags.Merge(c.flags, c.http.AddPeerName())

	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if !version.IsEnterprise() {
		c.UI.Error("operator utilization requires Consul Enterprise")
		return 1
	}

	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if len(c.flags.Args()) > 0 {
		c.UI.Error("Too many arguments (expected 0)")
		return 1
	}

	clientFactory := c.clientFn
	if clientFactory == nil {
		clientFactory = func() (*api.Client, error) {
			return c.http.APIClient()
		}
	}
	client, err := clientFactory()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	sendReport := c.assumeSend
	if !c.assumeSend {
		if c.canPrompt() {
			answer, err := c.UI.Ask("Send usage report to HashiCorp? [y/N]:")
			if err != nil {
				c.UI.Error(fmt.Sprintf("Prompt failed: %s", err))
				return 1
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			sendReport = answer == "y" || answer == "yes"
		} else {
			c.UI.Warn("Input is not interactive; skipping send prompt. Use -y to send automatically.")
		}
	}

	query := &api.QueryOptions{
		Datacenter: c.http.Datacenter(),
		Namespace:  c.http.Namespace(),
		Partition:  c.http.Partition(),
		Peer:       c.http.PeerName(),
		AllowStale: c.http.Stale(),
	}

	req := &api.UtilizationBundleRequest{
		Message:    c.message,
		TodayOnly:  c.todayOnly,
		SendReport: sendReport,
	}

	bundle, _, err := client.Census().Utilization(req, query)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error generating utilization bundle: %s", err))
		return 1
	}

	if len(bundle) == 0 {
		c.UI.Output("No utilization data available.")
		return 0
	}

	path := c.outputPath
	if path == "" {
		path = fmt.Sprintf("consul-utilization-%s.json", time.Now().UTC().Format("20060102-150405"))
	}

	if err := ensureParentDir(path); err != nil {
		c.UI.Error(fmt.Sprintf("Unable to prepare output path: %s", err))
		return 1
	}

	if err := os.WriteFile(path, bundle, 0o600); err != nil {
		c.UI.Error(fmt.Sprintf("Error writing utilization bundle: %s", err))
		return 1
	}

	c.UI.Output(fmt.Sprintf("Utilization bundle written to %s", path))
	if sendReport {
		c.UI.Output("Usage report sent to HashiCorp.")
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (c *cmd) canPrompt() bool {
	resolved := unwrapUi(c.UI)
	switch v := resolved.(type) {
	case *commandcli.BasicUI:
		return ensureBasicReader(&v.BasicUi)
	case *mcli.BasicUi:
		return ensureBasicReader(v)
	case *mcli.MockUi:
		return v.InputReader != nil
	default:
		return true
	}
}

func ensureBasicReader(ui *mcli.BasicUi) bool {
	if ui.Reader == nil {
		if os.Stdin == nil {
			return false
		}
		ui.Reader = os.Stdin
	}
	return true
}

func unwrapUi(ui mcli.Ui) mcli.Ui {
	current := ui
	for i := 0; i < 10; i++ {
		if current == nil {
			return nil
		}
		rv := reflect.ValueOf(current)
		if !rv.IsValid() {
			return current
		}
		if rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				return nil
			}
			rv = rv.Elem()
		}
		if rv.Kind() != reflect.Struct {
			return current
		}
		if field := rv.FieldByName("Ui"); field.IsValid() && field.CanInterface() {
			if inner, ok := field.Interface().(mcli.Ui); ok {
				current = inner
				continue
			}
		}
		if field := rv.FieldByName("UI"); field.IsValid() && field.CanInterface() {
			if inner, ok := field.Interface().(mcli.Ui); ok {
				current = inner
				continue
			}
		}
		return current
	}
	return current
}

const synopsis = "Generate a Consul utilization bundle for license reporting"

const help = `
Usage: consul operator utilization [options]

  Generate a license utilization bundle that can be shared with HashiCorp. The
  bundle is written to a JSON file and can optionally be sent automatically to
  HashiCorp during generation.

  Examples:
    Export all snapshots (in a bundle):
      $ consul operator utilization -message "Change Control 12345"

    Export today only:
      $ consul operator utilization --today-only

    Custom file path for bundle output:
      $ consul operator utilization -output "/reports/latest.json"
`
