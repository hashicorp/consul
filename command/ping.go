package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/mitchellh/cli"
	"os"
	"os/signal"
	"strings"
	"time"
)

// PingCommand is a Command implementation that is used to initiate
// a series of Serf 'ping' messages to the specified client node
type PingCommand struct {
	ShutdownCh <-chan struct{}
	Ui         cli.Ui
}

func (c *PingCommand) Help() string {
	helpText := `
Usage: consul reachability [options]

  Issues a series of Serf 'ping' messages to the specified node

Options:

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
  -count                    Number of pings to send (0 is infinite)
  -node                     Name of node to ping
`
	return strings.TrimSpace(helpText)
}

func (c *PingCommand) Run(args []string) int {
	var count int
	var nodename string
	exit := 0
	cmdFlags := flag.NewFlagSet("ping", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.IntVar(&count, "count", 0, "count")
	cmdFlags.StringVar(&nodename, "node", "*", "name of node to ping")
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

	success := 0
	latency := 0 * time.Millisecond
	var sent int
	c.Ui.Output("Starting serf ping...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		cleanup(sent, success, latency, c)
		os.Exit(exit)
	}()
	for sent = 0; (count == 0) || (sent < count); sent++ {
		if sent > 0 {
			time.Sleep(time.Second)
		}
		// Start the query
		params := consul.SerfPingParam{
			Name: nodename,
		}
		if resp, err := cl.SerfPing(&params); err != nil {
			c.Ui.Error(fmt.Sprintf("Error sending serf ping: %s", err))
			exit = 1
			break
		} else if resp.Success {
			c.Ui.Output(fmt.Sprintf("Count %d: Node %s responded in %s", sent, nodename, resp.RTT))
			latency = latency + resp.RTT
			success++
		} else {
			c.Ui.Output(fmt.Sprintf("Count %d: Node %s failed to respond in %s", sent, nodename, resp.RTT))
		}
	}

	if exit == 0 {
		cleanup(sent, success, latency, c)
	}

	return exit
}

func cleanup(sent int, success int, latency time.Duration, c *PingCommand) {
	var avgLatency string
	if success > 0 {
		avgLatency = fmt.Sprintf("%s", latency/time.Duration(success))
	} else {
		avgLatency = "N/A"
	}
	c.Ui.Output(fmt.Sprintf("Statistics: total %d, success %d%%, avg latency: %s",
		sent, (success * 100 / sent), avgLatency))
}

func (c *PingCommand) Synopsis() string {
	return "Issue Serf 'ping' messages to specified node"
}
