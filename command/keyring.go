package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
)

// KeyringCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type KeyringCommand struct {
	Ui cli.Ui
}

func (c *KeyringCommand) Run(args []string) int {
	var installKey, useKey, removeKey, token string
	var listKeys bool

	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.StringVar(&installKey, "install", "", "install key")
	cmdFlags.StringVar(&useKey, "use", "", "use key")
	cmdFlags.StringVar(&removeKey, "remove", "", "remove key")
	cmdFlags.BoolVar(&listKeys, "list", false, "list keys")
	cmdFlags.StringVar(&token, "token", "", "acl token")

	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "",
		InfoPrefix:   "==> ",
		ErrorPrefix:  "",
		Ui:           c.Ui,
	}

	// Only accept a single argument
	found := listKeys
	for _, arg := range []string{installKey, useKey, removeKey} {
		if found && len(arg) > 0 {
			c.Ui.Error("Only a single action is allowed")
			return 1
		}
		found = found || len(arg) > 0
	}

	// Fail fast if no actionable args were passed
	if !found {
		c.Ui.Error(c.Help())
		return 1
	}

	// All other operations will require a client connection
	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	if listKeys {
		c.Ui.Info("Gathering installed encryption keys...")
		r, err := client.ListKeys(token)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		if rval := c.handleResponse(r.Info, r.Messages); rval != 0 {
			return rval
		}
		c.handleList(r.Info, r.Keys)
		return 0
	}

	if installKey != "" {
		c.Ui.Info("Installing new gossip encryption key...")
		r, err := client.InstallKey(installKey, token)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return c.handleResponse(r.Info, r.Messages)
	}

	if useKey != "" {
		c.Ui.Info("Changing primary gossip encryption key...")
		r, err := client.UseKey(useKey, token)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return c.handleResponse(r.Info, r.Messages)
	}

	if removeKey != "" {
		c.Ui.Info("Removing gossip encryption key...")
		r, err := client.RemoveKey(removeKey, token)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("error: %s", err))
			return 1
		}
		return c.handleResponse(r.Info, r.Messages)
	}

	// Should never make it here
	return 0
}

func (c *KeyringCommand) handleResponse(
	info []agent.KeyringInfo,
	messages []agent.KeyringMessage) int {

	var rval int

	for _, i := range info {
		if i.Error != "" {
			pool := i.Pool
			if pool != "WAN" {
				pool = i.Datacenter + " (LAN)"
			}

			c.Ui.Error("")
			c.Ui.Error(fmt.Sprintf("%s error: %s", pool, i.Error))

			for _, msg := range messages {
				if msg.Datacenter != i.Datacenter || msg.Pool != i.Pool {
					continue
				}
				c.Ui.Error(fmt.Sprintf("  %s: %s", msg.Node, msg.Message))
			}
			rval = 1
		}
	}

	if rval == 0 {
		c.Ui.Info("Done!")
	}

	return rval
}

func (c *KeyringCommand) handleList(
	info []agent.KeyringInfo,
	keys []agent.KeyringEntry) {

	installed := make(map[string]map[string][]int)
	for _, key := range keys {
		var nodes int
		for _, i := range info {
			if i.Datacenter == key.Datacenter && i.Pool == key.Pool {
				nodes = i.NumNodes
			}
		}

		pool := key.Pool
		if pool != "WAN" {
			pool = key.Datacenter + " (LAN)"
		}

		if _, ok := installed[pool]; !ok {
			installed[pool] = map[string][]int{key.Key: []int{key.Count, nodes}}
		} else {
			installed[pool][key.Key] = []int{key.Count, nodes}
		}
	}

	for pool, keys := range installed {
		c.Ui.Output("")
		c.Ui.Output(pool + ":")
		for key, num := range keys {
			c.Ui.Output(fmt.Sprintf("  %s [%d/%d]", key, num[0], num[1]))
		}
	}
}

func (c *KeyringCommand) Help() string {
	helpText := `
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

Options:

  -install=<key>            Install a new encryption key. This will broadcast
                            the new key to all members in the cluster.
  -list                     List all keys currently in use within the cluster.
  -remove=<key>             Remove the given key from the cluster. This
                            operation may only be performed on keys which are
                            not currently the primary key.
  -token=""                 ACL token to use during requests. Defaults to that
                            of the agent.
  -use=<key>                Change the primary encryption key, which is used to
                            encrypt messages. The key must already be installed
                            before this operation can succeed.
  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *KeyringCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}
