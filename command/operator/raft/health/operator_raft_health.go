package health

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
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

	// Set up a client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initilizing client: %s", err))
		return 1
	}

	result := []string{"Name|Status|Leader|LastContact|LastIndex|Voter|Healthy"}
	// Fetch health informations
	q := &api.QueryOptions{}
	reply, err := client.Operator().AutopilotServerHealth(q)
	if err != nil {
		// This is expected when the cluster is not healthy
		if strings.HasPrefix(err.Error(), "Unexpected response code: 429") {
			ret := columnize.SimpleFormat(result)
			ret += fmt.Sprintf("\n0 servers can fail without causing an outage")
			c.UI.Output(ret)
			return 2
		}
		c.UI.Error(fmt.Sprintf("Failed to retrieve cluster health: %v", err))
		return 1
	}

	// Format the result
	for _, s := range reply.Servers {
		result = append(result, fmt.Sprintf("%v|%v|%v|%v|%v|%v|%v",
			s.Name, s.SerfStatus, s.Leader, s.LastContact, s.LastIndex,
			s.Voter, s.Healthy,
		))
	}

	ret := columnize.SimpleFormat(result)
	ret += fmt.Sprintf("\n%v servers can fail without causing an outage", reply.FailureTolerance)

	c.UI.Output(ret)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Display health information about the raft cluster"
const help = `
Usage: consul operator raft health

  Display health information about the raft cluster.
`
