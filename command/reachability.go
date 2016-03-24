package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/serf/serf"
	"github.com/mitchellh/cli"
	"strings"
	"time"
)

const (
	nonLiveNodeAcks     = `This could mean Serf is detecting false-failures due to a misconfiguration or network issue.`
	liveNodeMissingAcks = `This could mean Serf gossip packets are being lost due to a misconfiguration or network issue.`
	duplicateResponses  = `Duplicate responses means there is a misconfiguration. Verify that node names are unique.`
	troubleshooting     = `
Troubleshooting tips:
* Ensure that the bind addr:port is accessible by all other nodes
* If an advertise address is set, ensure it routes to the bind address
* Check that no nodes are behind a NAT
* If nodes are behind firewalls or iptables, check that Serf traffic is permitted (UDP and TCP)
* Verify networking equipment is functional`
)

// ReachabilityCommand is a Command implementation that is used to trigger
// a new reachability test
type ReachabilityCommand struct {
	ShutdownCh <-chan struct{}
	Ui         cli.Ui
}

func (c *ReachabilityCommand) Help() string {
	helpText := `
Usage: consul reachability [options]

  Tests the network reachability of this node

Options:

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
  -verbose                  Verbose mode
`
	return strings.TrimSpace(helpText)
}

func (c *ReachabilityCommand) Run(args []string) int {
	var verbose bool
	cmdFlags := flag.NewFlagSet("reachability", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&verbose, "verbose", false, "verbose mode")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	cl, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer cl.Close()

	ackCh := make(chan string, 128)

	// Get the list of LAN members
	var members []agent.Member
	members, err = cl.LANMembers()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting members: %s", err))
		return 1
	}

	// Get only the live members
	liveMembers := make(map[string]struct{})
	for _, m := range members {
		if m.Status == "alive" {
			liveMembers[m.Name] = struct{}{}
		}
	}
	c.Ui.Output(fmt.Sprintf("Total members: %d, live members: %d", len(members), len(liveMembers)))

	// Start the query
	params := consul.SerfQueryParam{
		RequestAck: true,
		Name:       serf.InternalQueryPrefix + "ping",
		AckCh:      ackCh,
	}
	if err := cl.SerfQuery(&params); err != nil {
		c.Ui.Error(fmt.Sprintf("Error sending serf query: %s", err))
		return 1
	}
	c.Ui.Output("Starting reachability test...")
	start := time.Now()
	last := time.Now()

	// Track responses and acknowledgements
	exit := 0
	dups := false
	numAcks := 0
	acksFrom := make(map[string]struct{}, len(members))

OUTER:
	for {
		select {
		case a := <-ackCh:
			if a == "" {
				break OUTER
			}
			if verbose {
				c.Ui.Output(fmt.Sprintf("\tAck from '%s'", a))
			}
			numAcks++
			if _, ok := acksFrom[a]; ok {
				dups = true
				c.Ui.Output(fmt.Sprintf("Duplicate response from '%v'", a))
			}
			acksFrom[a] = struct{}{}
			last = time.Now()

		case <-c.ShutdownCh:
			c.Ui.Error("Test interrupted")
			return 1
		}
	}

	if verbose {
		total := float64(time.Now().Sub(start)) / float64(time.Second)
		timeToLast := float64(last.Sub(start)) / float64(time.Second)
		c.Ui.Output(fmt.Sprintf("Query time: %0.2f sec, time to last response: %0.2f sec", total, timeToLast))
	}

	// Print troubleshooting info for duplicate responses
	if dups {
		c.Ui.Output(duplicateResponses)
		exit = 1
	}

	// Ensure all live members responded.
	liveNotResponding := make(map[string]bool)
	for m := range liveMembers {
		if _, ok := acksFrom[m]; !ok {
			c.Ui.Output(fmt.Sprintf("Missing ack from: %s", m))
			liveNotResponding[m] = true
		}
	}

	// Ensure that no responses came from non-live nodes.
	nonliveResponding := make(map[string]bool)
	for m := range acksFrom {
		if _, ok := liveMembers[m]; !ok {
			c.Ui.Output(fmt.Sprintf("Received ack from non-live node: %s", m))
			nonliveResponding[m] = true
		}
	}

	if (len(liveNotResponding) == 0) && (len(nonliveResponding) == 0) {
		c.Ui.Output("Successfully contacted all live nodes")

	} else {
		if len(nonliveResponding) != 0 {
			c.Ui.Output("Acks from non-live nodes:")
			for m := range nonliveResponding {
				c.Ui.Output(fmt.Sprintf("\t%s", m))
			}
			c.Ui.Output(nonLiveNodeAcks)
		}
		if len(liveNotResponding) != 0 {
			c.Ui.Output("Missing acks from:")
			for m := range liveNotResponding {
				c.Ui.Output(fmt.Sprintf("\t%s", m))
			}
			c.Ui.Output(liveNodeMissingAcks)
		}
		c.Ui.Output(troubleshooting)
		exit = 1
	}
	return exit
}

func (c *ReachabilityCommand) Synopsis() string {
	return "Test network reachability"
}
