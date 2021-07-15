package reachability

import (
	"bytes"
	"flag"
	"fmt"
	"time"

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
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	client *api.Client

	verbose bool
	wan     bool
	segment string
	timeout time.Duration
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.verbose, "verbose", false, "verbose mode")
	c.flags.BoolVar(&c.wan, "wan", false,
		"If the agent is in server mode, this can be used to test the reachability "+
			"of the WAN pool from this node.")
	c.flags.StringVar(&c.segment, "segment", api.AllSegments,
		"(Enterprise-only) If provided, output is filtered to only nodes in"+
			"the given segment.")
	c.flags.DurationVar(&c.timeout, "timeout", 0,
		"Maximum amount of time to let the serf query run for, specified as a "+
			"duration like \"1s\" or \"3h\". The default value is 0 which "+
			"means to use a heuristic value derived from pool size.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	// flags.Merge(c.flags, c.http.NamespaceFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	if c.timeout < 0 {
		c.UI.Error("Timeout must be positive")
		return 1
	}

	c.UI = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.UI,
	}

	// Setup Consul client
	var err error
	c.client, err = c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	c.UI.Info("Starting reachability test...")

	opts := api.ReachabilityOpts{
		Segment: c.segment,
		WAN:     c.wan,
	}
	responses, _, err := c.client.Agent().ReachabilityProbe(opts, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("error: %s", err))
		return 1
	}

	var failed bool
	for i, response := range responses.Responses {
		if i != 0 {
			c.UI.Output("")
		}
		c.UI.Output(c.formatResponse(response, &failed))
	}

	if failed {
		c.UI.Output("\n" + troubleshooting)
		return 1
	}

	return 0
}

func (c *cmd) formatResponse(response *api.ReachabilityResponse, failed *bool) string {
	var buf bytes.Buffer

	liveMembers := make(map[string]struct{})
	for _, member := range response.LiveMembers {
		liveMembers[member] = struct{}{}
	}

	poolName := fmt.Sprintf("%s in %s (LAN)", response.Node, response.Datacenter)
	if response.WAN {
		poolName = "WAN"
	} else if response.Segment != "" {
		poolName = fmt.Sprintf("%s [%s]", poolName, response.Segment)
	}

	buf.WriteString(fmt.Sprintf("%s:\n", poolName))
	buf.WriteString(fmt.Sprintf("    Total Members: %d\n", response.NumNodes))
	buf.WriteString(fmt.Sprintf("    Live Members:  %d\n", len(liveMembers)))

	var (
		dups     = false
		acksFrom = make(map[string]struct{}, response.NumNodes)
	)
	for _, ack := range response.Acks {
		if c.verbose {
			buf.WriteString(fmt.Sprintf("    Ack from: %q\n", ack))
		}
		if _, ok := acksFrom[ack]; ok {
			dups = true
			buf.WriteString(fmt.Sprintf("    Duplicate response from: %q\n", ack))
		}
		acksFrom[ack] = struct{}{}
	}

	if c.verbose {
		buf.WriteString(fmt.Sprintf("    Query timeout: %s\n", response.QueryTimeout))
		buf.WriteString(fmt.Sprintf("    Query time: %s\n", response.QueryTime))
		buf.WriteString(fmt.Sprintf("    Time to last response: %s\n", response.TimeToLastResponse))
	}

	// Print troubleshooting info for duplicate responses
	if dups {
		buf.WriteString(fmt.Sprintf("    %s\n", duplicateResponses))
		*failed = true
	}

	var (
		numAcks = len(response.Acks)
		n       = len(liveMembers)
	)
	if numAcks == n {
		buf.WriteString("    OK: Successfully contacted all live nodes\n")

	} else if numAcks > n {
		buf.WriteString("    FAIL: Received more acks than live nodes!\n")
		buf.WriteString("    Acks from non-live nodes:\n")
		for m := range acksFrom {
			if _, ok := liveMembers[m]; !ok {
				buf.WriteString(fmt.Sprintf("        %s\n", m))
			}
		}
		buf.WriteString(fmt.Sprintf("    %s\n", tooManyAcks))
		*failed = true

	} else if numAcks < n {
		buf.WriteString("    FAIL: Received fewer acks than live nodes!\n")
		buf.WriteString("    Missing acks from:\n")
		for m := range liveMembers {
			if _, ok := acksFrom[m]; !ok {
				buf.WriteString(fmt.Sprintf("        %s\n", m))
			}
		}
		buf.WriteString(fmt.Sprintf("    %s\n", tooFewAcks))
		*failed = true
	}

	return buf.String()
}

const (
	tooManyAcks        = `This could mean Serf is detecting false-failures due to a misconfiguration or network issue.`
	tooFewAcks         = `This could mean Serf gossip packets are being lost due to a misconfiguration or network issue.`
	duplicateResponses = `Duplicate responses means there is a misconfiguration. Verify that node names are unique.`
	troubleshooting    = `
Troubleshooting tips:
* Ensure that the serf bind addr:port is accessible by all other nodes
* If an advertise address is set, ensure it routes to the serf bind address
* Check that no nodes are behind a NAT
* If nodes are behind firewalls or iptables, check that Serf traffic is permitted (UDP and TCP)
* Verify networking equipment is functional`
)

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Tests the network reachability of this node."
const help = `
Usage: consul reachability [options]

  Tests the network reachability of this node.

`
