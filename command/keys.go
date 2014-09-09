package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

// KeysCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type KeysCommand struct {
	Ui cli.Ui
}

func (c *KeysCommand) Run(args []string) int {
	var installKey, useKey, removeKey string
	var listKeys, wan bool

	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.StringVar(&installKey, "install", "", "install key")
	cmdFlags.StringVar(&useKey, "use", "", "use key")
	cmdFlags.StringVar(&removeKey, "remove", "", "remove key")
	cmdFlags.BoolVar(&listKeys, "list", false, "list keys")
	cmdFlags.BoolVar(&wan, "wan", false, "operate on wan keys")

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

	var out []string
	var failures map[string]string
	var err error

	// Only accept a single argument
	found := listKeys
	for _, arg := range []string{installKey, useKey, removeKey} {
		if found && len(arg) > 0 {
			c.Ui.Error("Only one of -list, -install, -use, or -remove allowed")
			return 1
		}
		found = found || len(arg) > 0
	}

	// Fail fast if no actionable args were passed
	if !found {
		c.Ui.Error(c.Help())
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	defer client.Close()

	if listKeys {
		var keys map[string]int
		var numNodes int

		if wan {
			c.Ui.Info("Asking all WAN members for installed keys...")
			keys, numNodes, failures, err = client.ListKeysWAN()
		} else {
			c.Ui.Info("Asking all LAN members for installed keys...")
			keys, numNodes, failures, err = client.ListKeysLAN()
		}

		if err != nil {
			if len(failures) > 0 {
				for node, msg := range failures {
					out = append(out, fmt.Sprintf("failed: %s | %s", node, msg))
				}
				c.Ui.Error(columnize.SimpleFormat(out))
			}
			c.Ui.Error("")
			c.Ui.Error(fmt.Sprintf("Failed gathering member keys: %s", err))
			return 1
		}

		c.Ui.Info("Keys gathered, listing cluster keys...")
		c.Ui.Output("")

		for key, num := range keys {
			out = append(out, fmt.Sprintf("%s | [%d/%d]", key, num, numNodes))
		}
		c.Ui.Output(columnize.SimpleFormat(out))

		return 0
	}

	if installKey != "" {
		if wan {
			c.Ui.Info("Installing new WAN gossip encryption key...")
			failures, err = client.InstallKeyWAN(installKey)
		} else {
			c.Ui.Info("Installing new LAN gossip encryption key...")
			failures, err = client.InstallKeyLAN(installKey)
		}

		if err != nil {
			if len(failures) > 0 {
				for node, msg := range failures {
					out = append(out, fmt.Sprintf("failed: %s | %s", node, msg))
				}
				c.Ui.Error(columnize.SimpleFormat(out))
			}
			c.Ui.Error("")
			c.Ui.Error(fmt.Sprintf("Error installing key: %s", err))
			return 1
		}

		c.Ui.Info("Successfully installed key!")
		return 0
	}

	if useKey != "" {
		if wan {
			c.Ui.Info("Changing primary encryption key on WAN members...")
			failures, err = client.UseKeyWAN(useKey)
		} else {
			c.Ui.Info("Changing primary encryption key on LAN members...")
			failures, err = client.UseKeyLAN(useKey)
		}

		if err != nil {
			if len(failures) > 0 {
				for node, msg := range failures {
					out = append(out, fmt.Sprintf("failed: %s | %s", node, msg))
				}
				c.Ui.Error(columnize.SimpleFormat(out))
			}
			c.Ui.Error("")
			c.Ui.Error(fmt.Sprintf("Error changing primary key: %s", err))
			return 1
		}

		c.Ui.Info("Successfully changed primary key!")

		return 0
	}

	if removeKey != "" {
		return 0
	}

	// Should never make it here
	return 0
}

func (c *KeysCommand) Help() string {
	helpText := `
Usage: consul keys [options]

  Manages encryption keys used for gossip messages. Gossip encryption is
  optional. When enabled, this command may be used to examine active encryption
  keys in the cluster, add new keys, and remove old ones. When combined, this
  functionality provides the ability to perform key rotation cluster-wide,
  without disrupting the cluster.

Options:

  -install=<key>            Install a new encryption key. This will broadcast
                            the new key to all members in the cluster.
  -use=<key>                Change the primary encryption key, which is used to
                            encrypt messages. The key must already be installed
                            before this operation can succeed.
  -remove=<key>             Remove the given key from the cluster. This
                            operation may only be performed on keys which are
                            not currently the primary key.
  -list                     List all keys currently in use within the cluster.
  -wan                      If talking with a server node, this flag can be used
                            to operate on the WAN gossip layer.
  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *KeysCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}
