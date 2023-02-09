package upstreams

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/hashicorp/consul/command/flags"
	troubleshoot "github.com/hashicorp/consul/troubleshoot/proxy"
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

	// flags
	envoyAdminEndpoint string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

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

	adminAddr, adminPort, err := net.SplitHostPort(c.envoyAdminEndpoint)
	if err != nil {
		c.UI.Error("Invalid Envoy Admin endpoint: " + err.Error())
		return 1
	}

	// Envoy requires IP addresses to bind too when using static so resolve DNS or
	// localhost here.
	adminBindIP, err := net.ResolveIPAddr("ip", adminAddr)
	if err != nil {
		c.UI.Error("Failed to resolve envoy admin endpoint: " + err.Error())
		c.UI.Error("Please make sure Envoy's Admin API is enabled.")
		return 1
	}

	t, err := troubleshoot.NewTroubleshoot(adminBindIP, adminPort)
	if err != nil {
		c.UI.Error("error generating troubleshoot client: " + err.Error())
		return 1
	}
	envoyIDs, upstreamIPs, err := t.GetUpstreams()
	if err != nil {
		c.UI.Error("error calling GetUpstreams: " + err.Error())
		return 1
	}

	c.UI.Output(fmt.Sprintf("==> Upstreams (explicit upstreams only) (%v)", len(envoyIDs)))
	for _, u := range envoyIDs {
		c.UI.Output(u)
	}

	c.UI.Output(fmt.Sprintf("\n==> Upstream IPs (transparent proxy only) (%v)", len(upstreamIPs)))
	for _, u := range upstreamIPs {
		c.UI.Output(fmt.Sprintf("%+v   %v   %+v", u.IPs, u.IsVirtual, u.ClusterNames))
	}

	c.UI.Output("\nIf you don't see your upstream address or cluster for a transparent proxy upstream:")
	c.UI.Output("- Check intentions: Tproxy upstreams are configured based on intentions, make sure you " +
		"have configured intentions to allow traffic to your upstream.")
	c.UI.Output("- You can also check that the right cluster is being dialed by running a DNS lookup " +
		"for the upstream you are dialing (i.e dig backend.svc.consul). If the address you get from that is missing " +
		"from the Upstream IPs your proxy may be misconfigured.")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Get upstream envoy identifiers for the current envoy instance"
	help     = `
Usage: consul troubleshoot upstreams [options]
  
  Connects to local Envoy and lists upstream service envoy identifiers.
  This command is used in combination with 
  'consul troubleshoot proxy' to diagnose issues in Consul service mesh. 
  Examples:
    $ consul troubleshoot upstreams
`
)
