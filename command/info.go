package command

import (
	"fmt"
	"sort"
	"strings"
)

// InfoCommand is a Command implementation that queries a running
// Consul agent for various debugging statistics for operators
type InfoCommand struct {
	BaseCommand
}

func (c *InfoCommand) Help() string {
	helpText := `
Usage: consul info [options]

	Provides debugging information for operators

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *InfoCommand) Run(args []string) int {
	c.BaseCommand.NewFlagSet(c)

	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	self, err := client.Agent().Self()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying agent: %s", err))
		return 1
	}
	stats, ok := self["Stats"]
	if !ok {
		c.UI.Error(fmt.Sprintf("Agent response did not contain 'Stats' key: %v", self))
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
		c.UI.Output(key + ":")

		// Sort the sub-keys
		subvals, ok := stats[key].(map[string]interface{})
		if !ok {
			c.UI.Error(fmt.Sprintf("Got invalid subkey in stats: %v", subvals))
			return 1
		}
		subkeys := make([]string, 0, len(subvals))
		for k := range subvals {
			subkeys = append(subkeys, k)
		}
		sort.Strings(subkeys)

		// Iterate over the subkeys
		for _, subkey := range subkeys {
			val := subvals[subkey]
			c.UI.Output(fmt.Sprintf("\t%s = %s", subkey, val))
		}
	}
	return 0
}

func (c *InfoCommand) Synopsis() string {
	return "Provides debugging information for operators."
}
