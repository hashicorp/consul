package command

import (
	"fmt"

	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

// KeyringCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type KeyringCommand struct {
	BaseCommand

	// flags
	installKey string
	useKey     string
	removeKey  string
	listKeys   bool
	relay      int
}

func (c *KeyringCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.StringVar(&c.installKey, "install", "",
		"Install a new encryption key. This will broadcast the new key to "+
			"all members in the cluster.")
	c.FlagSet.StringVar(&c.useKey, "use", "",
		"Change the primary encryption key, which is used to encrypt "+
			"messages. The key must already be installed before this operation "+
			"can succeed.")
	c.FlagSet.StringVar(&c.removeKey, "remove", "",
		"Remove the given key from the cluster. This operation may only be "+
			"performed on keys which are not currently the primary key.")
	c.FlagSet.BoolVar(&c.listKeys, "list", false,
		"List all keys currently in use within the cluster.")
	c.FlagSet.IntVar(&c.relay, "relay-factor", 0,
		"Setting this to a non-zero value will cause nodes to relay their response "+
			"to the operation through this many randomly-chosen other nodes in the "+
			"cluster. The maximum allowed value is 5.")
}

func (c *KeyringCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}

	c.UI = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.UI,
	}

	// Only accept a single argument
	found := c.listKeys
	for _, arg := range []string{c.installKey, c.useKey, c.removeKey} {
		if found && len(arg) > 0 {
			c.UI.Error("Only a single action is allowed")
			return 1
		}
		found = found || len(arg) > 0
	}

	// Fail fast if no actionable args were passed
	if !found {
		c.UI.Error(c.Help())
		return 1
	}

	// Validate the relay factor
	relayFactor, err := agent.ParseRelayFactor(c.relay)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing relay factor: %s", err))
		return 1
	}

	// All other operations will require a client connection
	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	if c.listKeys {
		c.UI.Info("Gathering installed encryption keys...")
		responses, err := client.Operator().KeyringList(&consulapi.QueryOptions{RelayFactor: relayFactor})
		if err != nil {
			c.UI.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		c.handleList(responses)
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

func (c *KeyringCommand) handleList(responses []*consulapi.KeyringResponse) {
	for _, response := range responses {
		pool := response.Datacenter + " (LAN)"
		if response.Segment != "" {
			pool += fmt.Sprintf(" [%s]", response.Segment)
		}
		if response.WAN {
			pool = "WAN"
		}

		c.UI.Output("")
		c.UI.Output(pool + ":")
		for key, num := range response.Keys {
			c.UI.Output(fmt.Sprintf("  %s [%d/%d]", key, num, response.NumNodes))
		}
	}
}

func (c *KeyringCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
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

`)
}

func (c *KeyringCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}
