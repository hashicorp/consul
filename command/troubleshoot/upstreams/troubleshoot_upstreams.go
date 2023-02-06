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
	adminBind string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	defaultAdminBind := "localhost:19000"
	if adminBind := os.Getenv("ADMIN_BIND"); adminBind != "" {
		defaultAdminBind = adminBind
	}
	c.flags.StringVar(&c.adminBind, "admin-bind", defaultAdminBind, "The address:port that envoy's admin endpoint is on.")

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

	adminAddr, adminPort, err := net.SplitHostPort(c.adminBind)
	if err != nil {
		c.UI.Error("Invalid Envoy Admin endpoint: " + err.Error())
		return 1
	}

	// Envoy requires IP addresses to bind too when using static so resolve DNS or
	// localhost here.
	adminBindIP, err := net.ResolveIPAddr("ip", adminAddr)
	if err != nil {
		c.UI.Error("Failed to resolve admin bind address: " + err.Error())
		return 1
	}

	t, err := troubleshoot.NewTroubleshoot(adminBindIP, adminPort)
	if err != nil {
		c.UI.Error("error generating troubleshoot client: " + err.Error())
		return 1
	}
	upstreams, err := t.GetUpstreams()
	if err != nil {
		c.UI.Error("error calling GetUpstreams: " + err.Error())
		return 1
	}

	for _, u := range upstreams {
		c.UI.Output(u)
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
