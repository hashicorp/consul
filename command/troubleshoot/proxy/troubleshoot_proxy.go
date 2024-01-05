// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxy

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/flags"
	troubleshoot "github.com/hashicorp/consul/troubleshoot/proxy"
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
	upstreamEnvoyID    string
	upstreamIP         string
	envoyAdminEndpoint string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.upstreamEnvoyID, "upstream-envoy-id", os.Getenv("UPSTREAM_ENVOY_ID"), "The envoy identifier of the upstream service that receives the communication. (explicit upstreams only)")
	c.flags.StringVar(&c.upstreamIP, "upstream-ip", os.Getenv("UPSTREAM_IP"), "The IP address of the upstream service that receives the communication. (transparent proxy only) ")

	defaultEnvoyAdminEndpoint := "localhost:19000"
	if envoyAdminEndpoint := os.Getenv("ENVOY_ADMIN_ENDPOINT"); envoyAdminEndpoint != "" {
		defaultEnvoyAdminEndpoint = envoyAdminEndpoint
	}
	c.flags.StringVar(&c.envoyAdminEndpoint, "envoy-admin-endpoint", defaultEnvoyAdminEndpoint, "The address:port that envoy's admin endpoint is on.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {

	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	if c.upstreamEnvoyID == "" && c.upstreamIP == "" {
		c.UI.Error("-upstream-envoy-id OR -upstream-ip is required.")
		c.UI.Error("Please run `consul troubleshoot upstreams` to find the corresponding upstream.")
		return 1
	}

	adminAddr, adminPort, err := net.SplitHostPort(c.envoyAdminEndpoint)
	if err != nil {
		c.UI.Error("Invalid Envoy Admin endpoint: " + err.Error())
		return 1
	}

	// Envoy requires IP addresses to bind too when using static so resolve DNS or
	// localhost here.
	adminBindIP, err := net.ResolveIPAddr("ip", adminAddr)
	if err != nil {
		c.UI.Error("Failed to resolve Envoy admin endpoint: " + err.Error())
		c.UI.Error("Please make sure Envoy's Admin API is enabled.")
		return 1
	}

	t, err := troubleshoot.NewTroubleshoot(adminBindIP, adminPort)
	if err != nil {
		c.UI.Error("Error generating troubleshoot client: " + err.Error())
		return 1
	}
	messages, err := t.RunAllTests(c.upstreamEnvoyID, c.upstreamIP)
	if err != nil {
		c.UI.Error("Error running the tests: " + err.Error())
		return 1
	}

	c.UI.HeaderOutput("Validation")
	for _, o := range messages {
		if o.Success {
			c.UI.SuccessOutput(o.Message)
		} else {
			c.UI.ErrorOutput(o.Message)
			for _, action := range o.PossibleActions {
				c.UI.UnchangedOutput("-> " + action)
			}
		}
	}
	if messages.Success() {
		c.UI.UnchangedOutput("If you are still experiencing issues, you can:")
		c.UI.UnchangedOutput("-> Check intentions to ensure the upstream allows traffic from this source")
		c.UI.UnchangedOutput("-> If using transparent proxy, ensure DNS resolution is to the same IP you have verified here")
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
	synopsis = "Troubleshoots service mesh issues from the current envoy instance"
	help     = `
Usage: consul troubleshoot proxy [options]
  
  Connects to local envoy proxy and troubleshoots service mesh communication issues.
  Requires an upstream service identifier. When debugging explicitly configured upstreams,
  use -upstream-envoy-id, when debugging transparent proxy upstreams use -upstream-ip.
  Examples:
    (explicit upstreams only)
      $ consul troubleshoot proxy -upstream-envoy-id foo
    (transparent proxy only)
      $ consul troubleshoot proxy -upstream-ip 240.0.0.1
 
    where 'foo' is the upstream envoy identifier and '240.0.0.1' is an upstream ip which
    can be obtained by running:
    $ consul troubleshoot upstreams [options]
`
)
