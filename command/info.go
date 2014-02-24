package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"sort"
	"strings"
)

// InfoCommand is a Command implementation that queries a running
// Consul agent for various debugging statistics for operators
type InfoCommand struct {
	Ui cli.Ui
}

func (i *InfoCommand) Help() string {
	helpText := `
Usage: consul info [options]

	Provides debugging information for operators

Options:

  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (i *InfoCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("info", flag.ContinueOnError)
	cmdFlags.Usage = func() { i.Ui.Output(i.Help()) }
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		i.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	stats, err := client.Stats()
	if err != nil {
		i.Ui.Error(fmt.Sprintf("Error querying agent: %s", err))
		return 1
	}

	// Get the keys in sorted order
	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate over each top-level key
	for _, key := range keys {
		i.Ui.Output(key + ":")

		// Sort the sub-keys
		subvals := stats[key]
		subkeys := make([]string, 0, len(subvals))
		for k := range subvals {
			subkeys = append(subkeys, k)
		}
		sort.Strings(subkeys)

		// Iterate over the subkeys
		for _, subkey := range subkeys {
			val := subvals[subkey]
			i.Ui.Output(fmt.Sprintf("\t%s = %s", subkey, val))
		}
	}
	return 0
}

func (i *InfoCommand) Synopsis() string {
	return "Provides debugging information for operators"
}
