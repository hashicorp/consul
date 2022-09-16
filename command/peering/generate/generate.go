package generate

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/peering"
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

	name              string
	externalAddresses []string
	meta              map[string]string
	format            string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.name, "name", "", "(Required) The local name assigned to the peer cluster.")

	c.flags.Var((*flags.FlagMapValue)(&c.meta), "meta",
		"Metadata to associate with the peering, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple metadata fields.")

	c.flags.Var((*flags.AppendSliceValue)(&c.externalAddresses), "server-external-addresses",
		"A list of addresses to put into the generated token, formatted as a comma-separate list. "+
			"Addresses are the form of <host or IP>:port. "+
			"This could be used to specify load balancer(s) or external IPs to reach the servers from "+
			"the dialing side, and will override any server addresses obtained from the \"consul\" service.")

	c.flags.StringVar(
		&c.format,
		"format",
		peering.PeeringFormatPretty,
		fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(peering.GetSupportedFormats(), "|"), peering.PeeringFormatPretty),
	)

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.name == "" {
		c.UI.Error("Missing the required -name flag")
		return 1
	}

	if !peering.FormatIsValid(c.format) {
		c.UI.Error(fmt.Sprintf("Invalid format, valid formats are {%s}", strings.Join(peering.GetSupportedFormats(), "|")))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	peerings := client.Peerings()

	req := api.PeeringGenerateTokenRequest{
		PeerName:                c.name,
		Partition:               c.http.Partition(),
		Meta:                    c.meta,
		ServerExternalAddresses: c.externalAddresses,
	}

	res, _, err := peerings.GenerateToken(context.Background(), req, &api.WriteOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error generating peering token for %s: %v", req.PeerName, err))
		return 1
	}

	if c.format == peering.PeeringFormatJSON {
		output, err := json.Marshal(res)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error marshalling JSON: %s", err))
			return 1
		}
		c.UI.Output(string(output))
		return 0
	}

	c.UI.Info(res.PeeringToken)
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Generate a peering token"
	help     = `
Usage: consul peering generate-token [options] -name <peer name>

  Generate a peering token. The name provided will be used locally by
  this cluster to refer to the peering connection. Re-generating a token 
  for a given name will not interrupt any active connection, but will 
  invalidate any unused token for that name.

  Example:

    $ consul peering generate-token -name west-dc

  Example using a load balancer in front of Consul servers:

    $ consul peering generate-token -name west-dc -server-external-addresses load-balancer.elb.us-west-1.amazonaws.com:8502
`
)
