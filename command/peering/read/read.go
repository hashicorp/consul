package read

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

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

	name   string
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.name, "name", "", "(Required) The local name assigned to the peer cluster.")

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

	res, _, err := peerings.Read(context.Background(), c.name, &api.QueryOptions{})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading peering: %s", err))
		return 1
	}

	if res == nil {
		c.UI.Error(fmt.Sprintf("No peering with name %s found.", c.name))
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

	c.UI.Output(formatPeering(res))

	return 0
}

func formatPeering(peering *api.Peering) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("Name:         %s\n", peering.Name))
	buffer.WriteString(fmt.Sprintf("ID:           %s\n", peering.ID))
	if peering.Partition != "" {
		buffer.WriteString(fmt.Sprintf("Partition:    %s\n", peering.Partition))
	}
	if peering.DeletedAt != nil {
		buffer.WriteString(fmt.Sprintf("DeletedAt:    %s\n", peering.DeletedAt.Format(time.RFC3339)))
	}
	buffer.WriteString(fmt.Sprintf("State:        %s\n", peering.State))
	if peering.Meta != nil && len(peering.Meta) > 0 {
		buffer.WriteString("Meta:\n")
		for k, v := range peering.Meta {
			buffer.WriteString(fmt.Sprintf("    %s=%s\n", k, v))
		}
	}

	buffer.WriteString("\n")
	buffer.WriteString(fmt.Sprintf("Peer ID:               %s\n", peering.PeerID))
	buffer.WriteString(fmt.Sprintf("Peer Server Name:      %s\n", peering.PeerServerName))
	buffer.WriteString(fmt.Sprintf("Peer CA Pems:          %d\n", len(peering.PeerCAPems)))
	if peering.PeerServerAddresses != nil && len(peering.PeerServerAddresses) > 0 {
		buffer.WriteString("Peer Server Addresses:\n")
		for _, v := range peering.PeerServerAddresses {
			buffer.WriteString(fmt.Sprintf("    %s", v))
		}
	}

	buffer.WriteString("\n")
	buffer.WriteString(fmt.Sprintf("Imported Services: %d\n", len(peering.StreamStatus.ImportedServices)))
	buffer.WriteString(fmt.Sprintf("Exported Services: %d\n", len(peering.StreamStatus.ExportedServices)))
	buffer.WriteString("\n")
	buffer.WriteString(fmt.Sprintf("Last Heartbeat:    %v\n", peering.StreamStatus.LastHeartbeat))
	buffer.WriteString(fmt.Sprintf("Last Send:         %v\n", peering.StreamStatus.LastSend))
	buffer.WriteString(fmt.Sprintf("Last Receive:      %v\n", peering.StreamStatus.LastReceive))
	buffer.WriteString("\n")
	buffer.WriteString(fmt.Sprintf("Create Index: %d\n", peering.CreateIndex))
	buffer.WriteString(fmt.Sprintf("Modify Index: %d\n", peering.ModifyIndex))

	return buffer.String()
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const (
	synopsis = "Read a peering connection"
	help     = `
Usage: consul peering read [options] -name <peer name>

  Read a peering connection with the provided name.  If one is not found,
  the command will exit with a non-zero code. The result will be filtered according
  to ACL policy configuration.

  Example:

    $ consul peering read -name west-dc
`
)
