package export

import (
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
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

	serviceName string
	peerName    string
}

func (c *cmd) init() {
	//This function defines the flags

	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.serviceName, "serviceName", "", "(Required) Specify the name of the service you want to export.")
	c.flags.StringVar(&c.peerName, "peerName", "", "(Required) Specify the name of the peer you want to export.")

	// c.flags.Var((*flags.FlagMapValue)(&c.meta), "meta",
	// 	"Metadata to associate with the peering, formatted as key=value. This flag "+
	// 		"may be specified multiple times to set multiple metadata fields.")

	// c.flags.Var((*flags.AppendSliceValue)(&c.peer), "peer",
	// 	"A list of peers where the services will be exported")

	// c.flags.StringVar(
	// 	&c.format,
	// 	"format",
	// 	peering.PeeringFormatPretty,
	// 	fmt.Sprintf("Output format {%s} (default: %s)", strings.Join(peering.GetSupportedFormats(), "|"), peering.PeeringFormatPretty),
	// )

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.PartitionFlag())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.serviceName == "" {
		c.UI.Error("Missing the required -service name flag")
		return 1
	}

	if c.peerName == "" {
		c.UI.Error("Missing the required -peer name flag")
		return 1
	}

	// //if !peering.FormatIsValid(c.format) { ?????
	// 	c.UI.Error(fmt.Sprintf("Invalid format, valid formats are {%s}", strings.Join(peering.GetSupportedFormats(), "|")))
	// 	return 1
	// }

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	cfg := api.ExportedServicesConfigEntry{
		Services: []api.ExportedService{
			{
				Name: c.serviceName,
				Consumers: []api.ServiceConsumer{
					{
						Peer: c.peerName,
					},
				},
			},
		},
	}
	client.ConfigEntries().Set(&cfg, &api.WriteOptions{})
	return 0
}

//========

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Generate a peering token"
	help     = `
Usage: consul peering export [options] -service <service name> -peers <other cluster name>

  Generate a peering token. The name provided will be used locally by
  this cluster to refer to the peering connection. Re-generating a token 
  for a given name will not interrupt any active connection, but will 
  invalidate any unused token for that name.

  Example:

	$ consul peering export -service=web -peers=other-cluster
`
)
