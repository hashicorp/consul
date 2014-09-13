package command

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

// KeysCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type KeysCommand struct {
	Ui cli.Ui
}

func (c *KeysCommand) Run(args []string) int {
	var installKey, useKey, removeKey, init, dataDir string
	var listKeys, wan bool

	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.StringVar(&installKey, "install", "", "install key")
	cmdFlags.StringVar(&useKey, "use", "", "use key")
	cmdFlags.StringVar(&removeKey, "remove", "", "remove key")
	cmdFlags.BoolVar(&listKeys, "list", false, "list keys")
	cmdFlags.BoolVar(&wan, "wan", false, "operate on wan keys")
	cmdFlags.StringVar(&init, "init", "", "initialize keyring")
	cmdFlags.StringVar(&dataDir, "data-dir", "", "data directory")

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
	for _, arg := range []string{installKey, useKey, removeKey, init} {
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

	var out, paths []string
	var failures map[string]string
	var err error

	if init != "" {
		if dataDir == "" {
			c.Ui.Error("Must provide -data-dir")
			return 1
		}
		if _, err := base64.StdEncoding.DecodeString(init); err != nil {
			c.Ui.Error(fmt.Sprintf("Invalid key: %s", err))
			return 1
		}

		paths = append(paths, filepath.Join(dataDir, agent.SerfLANKeyring))
		if wan {
			paths = append(paths, filepath.Join(dataDir, agent.SerfWANKeyring))
		}

		keys := []string{init}
		keyringBytes, err := json.MarshalIndent(keys, "", "  ")
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return 1
		}

		for _, path := range paths {
			if err := initializeKeyring(path, keyringBytes); err != nil {
				c.Ui.Error("Error: %s", err)
				return 1
			}
		}

		return 0
	}

	// All other operations will require a client connection
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
		if wan {
			c.Ui.Info("Removing key from WAN members...")
			failures, err = client.RemoveKeyWAN(removeKey)
		} else {
			c.Ui.Info("Removing key from LAN members...")
			failures, err = client.RemoveKeyLAN(removeKey)
		}

		if err != nil {
			if len(failures) > 0 {
				for node, msg := range failures {
					out = append(out, fmt.Sprintf("failed: %s | %s", node, msg))
				}
				c.Ui.Error(columnize.SimpleFormat(out))
			}
			c.Ui.Error("")
			c.Ui.Error(fmt.Sprintf("Error removing key: %s", err))
			return 1
		}

		c.Ui.Info("Successfully removed key!")
		return 0
	}

	// Should never make it here
	return 0
}

// initializeKeyring will create a keyring file at a given path.
func initializeKeyring(path string, key []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("File already exists: %s", path)
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := fh.Write(keyringBytes); err != nil {
		os.Remove(path)
		return err
	}

	return nil
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
  -init=<key>               Create an initial keyring file for Consul to use
                            containing the provided key. By default, this option
                            will only initialize the LAN keyring. If the -wan
                            option is also passed, then the wan keyring will be
                            created as well. The -data-dir argument is required
                            with this option.
  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *KeysCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}
