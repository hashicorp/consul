package proxy

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/hashicorp/consul/command/flags"
	troubleshoot "github.com/hashicorp/consul/troubleshoot"
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
	upstream  string
	adminBind string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.upstream, "upstream", os.Getenv("TROUBLESHOOT_UPSTREAM"), "The upstream service that receives the communication. ")

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

	if c.upstream == "" {
		c.UI.Error("-upstream envoy identifier is required")
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
	output, err := t.RunAllTests(c.upstream)
	if err != nil {
		c.UI.Error("error running the tests: " + err.Error())
		return 1
	}
	c.UI.Output(output)
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
  Requires an upstream service envoy identifier.
  Examples:
    $ consul troubleshoot proxy -upstream foo
 
    where 'foo' is the upstream envoy ID which 
    can be obtained by running:
    $ consul troubleshoot upstreams [options]
`
)
