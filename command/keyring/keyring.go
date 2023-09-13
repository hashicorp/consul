// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package keyring

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
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

	// flags
	installKey      string
	useKey          string
	removeKey       string
	listKeys        bool
	listPrimaryKeys bool
	relay           int
	local           bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.installKey, "install", "",
		"Install a new encryption key. This will broadcast the new key to "+
			"all members in the cluster.")
	c.flags.StringVar(&c.useKey, "use", "",
		"Change the primary encryption key, which is used to encrypt "+
			"messages. The key must already be installed before this operation "+
			"can succeed.")
	c.flags.StringVar(&c.removeKey, "remove", "",
		"Remove the given key from the cluster. This operation may only be "+
			"performed on keys which are not currently the primary key.")
	c.flags.BoolVar(&c.listKeys, "list", false,
		"List all keys currently in use within the cluster.")
	c.flags.BoolVar(&c.listPrimaryKeys, "list-primary", false,
		"List all primary keys currently in use within the cluster.")
	c.flags.IntVar(&c.relay, "relay-factor", 0,
		"Setting this to a non-zero value will cause nodes to relay their response "+
			"to the operation through this many randomly-chosen other nodes in the "+
			"cluster. The maximum allowed value is 5.")
	c.flags.BoolVar(&c.local, "local-only", false,
		"Setting this to true will force the keyring query to only hit local servers "+
			"(no WAN traffic). This flag can only be set for list queries.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func numberActions(listKeys, listPrimaryKeys bool, installKey, useKey, removeKey string) int {
	count := 0
	if listKeys {
		count++
	}
	if listPrimaryKeys {
		count++
	}
	for _, arg := range []string{installKey, useKey, removeKey} {
		if len(arg) > 0 {
			count++
		}
	}
	return count
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	c.UI = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.UI,
	}

	num := numberActions(c.listKeys, c.listPrimaryKeys, c.installKey, c.useKey, c.removeKey)
	if num == 0 {
		c.UI.Error(c.Help())
		return 1
	}
	if num > 1 {
		c.UI.Error("Only a single action is allowed")
		return 1
	}

	// Validate the relay factor
	relayFactor, err := agent.ParseRelayFactor(c.relay)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing relay factor: %s", err))
		return 1
	}

	// Validate local-only
	err = agent.ValidateLocalOnly(c.local, c.listKeys)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error validating local-only: %s", err))
		return 1
	}

	// All other operations will require a client connection
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if c.listKeys {
		c.UI.Info("Gathering installed encryption keys...")
		responses, err := client.Operator().KeyringList(&consulapi.QueryOptions{RelayFactor: relayFactor, LocalOnly: c.local})
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		for _, response := range responses {
			c.UI.Output(formatResponse(response, response.Keys))
		}
		return 0
	}

	if c.listPrimaryKeys {
		c.UI.Info("Gathering installed primary encryption keys...")
		responses, err := client.Operator().KeyringList(&consulapi.QueryOptions{RelayFactor: relayFactor, LocalOnly: c.local})
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		for _, response := range responses {
			c.UI.Output(formatResponse(response, response.PrimaryKeys))
		}
		return 0
	}

	opts := &consulapi.WriteOptions{RelayFactor: relayFactor}
	if c.installKey != "" {
		c.UI.Info("Installing new gossip encryption key...")
		err := client.Operator().KeyringInstall(c.installKey, opts)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	if c.useKey != "" {
		c.UI.Info("Changing primary gossip encryption key...")
		err := client.Operator().KeyringUse(c.useKey, opts)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	if c.removeKey != "" {
		c.UI.Info("Removing gossip encryption key...")
		err := client.Operator().KeyringRemove(c.removeKey, opts)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return 0
	}

	// Should never make it here
	return 0
}

func formatResponse(response *consulapi.KeyringResponse, keys map[string]int) string {
	b := new(strings.Builder)
	b.WriteString("\n")
	b.WriteString(poolName(response.Datacenter, response.WAN, response.Partition, response.Segment))
	b.WriteString(formatMessages(response.Messages))
	b.WriteString(formatKeys(keys, response.NumNodes))
	return strings.TrimRight(b.String(), "\n")
}

func poolName(dc string, wan bool, partition, segment string) string {
	pool := fmt.Sprintf("%s (LAN)", dc)
	if wan {
		pool = "WAN"
	}

	var suffix string
	if segment != "" {
		suffix = fmt.Sprintf(" [%s]", segment)
	} else if !acl.IsDefaultPartition(partition) {
		suffix = fmt.Sprintf(" [partition: %s]", partition)
	}
	return fmt.Sprintf("%s%s:\n", pool, suffix)
}

func formatMessages(messages map[string]string) string {
	b := new(strings.Builder)
	for from, msg := range messages {
		b.WriteString(fmt.Sprintf("  ===> %s: %s\n", from, msg))
	}
	return b.String()
}

func formatKeys(keys map[string]int, total int) string {
	b := new(strings.Builder)
	for key, num := range keys {
		b.WriteString(fmt.Sprintf("  %s [%d/%d]\n", key, num, total))
	}
	return b.String()
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Manages gossip layer encryption keys"
const help = `
Usage: consul keyring [options]

  Manages encryption keys used for gossip messages. Gossip encryption is
  optional. When enabled, this command may be used to examine active encryption
  keys in the cluster, add new keys, and remove old ones. When combined, this
  functionality provides the ability to perform key rotation cluster-wide,
  without disrupting the cluster.

  All operations performed by this command can only be run against server nodes,
  and affect both the LAN and WAN keyrings in lock-step.

  All variations of the keyring command return 0 if all nodes reply and there
  are no errors. If any node fails to reply or reports failure, the exit code
  will be 1.
`
