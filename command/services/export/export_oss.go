//go:build !consulent
// +build !consulent

package export

import (
	"flag"

	"github.com/hashicorp/consul/command/flags"
)

func (c *cmd) init() {
	//This function defines the flags for OSS.
	// Flags related to namespaces and partitions are excluded.

	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.serviceName, "name", "", "(Required) Specify the name of the service you want to export.")
	c.flags.StringVar(&c.peerNames, "consumer-peers", "", "A list of peers to export the service to, formatted as a comma-separated list.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) getPartitionNames() ([]string, error) {
	return []string{}, nil
}
