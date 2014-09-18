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

// KeyringCommand is a Command implementation that handles querying, installing,
// and removing gossip encryption keys from a keyring.
type KeyringCommand struct {
	Ui cli.Ui
}

func (c *KeyringCommand) Run(args []string) int {
	var installKey, useKey, removeKey, init, dataDir string
	var listKeys bool

	cmdFlags := flag.NewFlagSet("keys", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }

	cmdFlags.StringVar(&installKey, "install", "", "install key")
	cmdFlags.StringVar(&useKey, "use", "", "use key")
	cmdFlags.StringVar(&removeKey, "remove", "", "remove key")
	cmdFlags.BoolVar(&listKeys, "list", false, "list keys")
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

	if init != "" {
		if dataDir == "" {
			c.Ui.Error("Must provide -data-dir")
			return 1
		}

		fileLAN := filepath.Join(dataDir, agent.SerfLANKeyring)
		if err := initKeyring(fileLAN, init); err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return 1
		}
		fileWAN := filepath.Join(dataDir, agent.SerfWANKeyring)
		if err := initKeyring(fileWAN, init); err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return 1
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

	// For all key-related operations, we must be querying a server node. It is
	// probably better to enforce this even for LAN pool changes, because other-
	// wise, the same exact command syntax will have different results depending
	// on where it was run.
	s, err := client.Stats()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error: %s", err))
		return 1
	}
	if s["consul"]["server"] != "true" {
		c.Ui.Error("Error: Key modification can only be handled by a server")
		return 1
	}

	if listKeys {
		c.Ui.Info("Asking all WAN members for installed keys...")
		if rval := c.listKeysOperation(client.ListKeysWAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Asking all LAN members for installed keys...")
		if rval := c.listKeysOperation(client.ListKeysLAN); rval != 0 {
			return rval
		}
		return 0
	}

	if installKey != "" {
		c.Ui.Info("Installing new WAN gossip encryption key...")
		if rval := c.keyOperation(installKey, client.InstallKeyWAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Installing new LAN gossip encryption key...")
		if rval := c.keyOperation(installKey, client.InstallKeyLAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Successfully installed key!")
		return 0
	}

	if useKey != "" {
		c.Ui.Info("Changing primary WAN gossip encryption key...")
		if rval := c.keyOperation(useKey, client.UseKeyWAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Changing primary LAN gossip encryption key...")
		if rval := c.keyOperation(useKey, client.UseKeyLAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Successfully changed primary key!")
		return 0
	}

	if removeKey != "" {
		c.Ui.Info("Removing WAN gossip encryption key...")
		if rval := c.keyOperation(removeKey, client.RemoveKeyWAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Removing LAN gossip encryption key...")
		if rval := c.keyOperation(removeKey, client.RemoveKeyLAN); rval != 0 {
			return rval
		}
		c.Ui.Info("Successfully removed key!")
		return 0
	}

	// Should never make it here
	return 0
}

// keyFunc is a function which manipulates gossip encryption keyrings. This is
// used for key installation, removal, and primary key changes.
type keyFunc func(string) (map[string]string, error)

// keyOperation is a unified process for manipulating the gossip keyrings.
func (c *KeyringCommand) keyOperation(key string, fn keyFunc) int {
	var out []string

	failures, err := fn(key)

	if err != nil {
		if len(failures) > 0 {
			for node, msg := range failures {
				out = append(out, fmt.Sprintf("failed: %s | %s", node, msg))
			}
			c.Ui.Error(columnize.SimpleFormat(out))
		}
		c.Ui.Error("")
		c.Ui.Error(fmt.Sprintf("Error: %s", err))
		return 1
	}

	return 0
}

// listKeysFunc is a function which handles querying lists of gossip keys
type listKeysFunc func() (map[string]int, int, map[string]string, error)

// listKeysOperation is a unified process for querying and
// displaying gossip keys.
func (c *KeyringCommand) listKeysOperation(fn listKeysFunc) int {
	var out []string

	keys, numNodes, failures, err := fn()

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
	for key, num := range keys {
		out = append(out, fmt.Sprintf("%s | [%d/%d]", key, num, numNodes))
	}
	c.Ui.Output(columnize.SimpleFormat(out))

	c.Ui.Output("")
	return 0
}

// initKeyring will create a keyring file at a given path.
func initKeyring(path, key string) error {
	if _, err := base64.StdEncoding.DecodeString(key); err != nil {
		return fmt.Errorf("Invalid key: %s", err)
	}

	keys := []string{key}
	keyringBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}

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

func (c *KeyringCommand) Help() string {
	helpText := `
Usage: consul keyring [options]

  Manages encryption keys used for gossip messages. Gossip encryption is
  optional. When enabled, this command may be used to examine active encryption
  keys in the cluster, add new keys, and remove old ones. When combined, this
  functionality provides the ability to perform key rotation cluster-wide,
  without disrupting the cluster.

  With the exception of the -init argument, all other operations performed by
  this command can only be run against server nodes.

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
  -init=<key>               Create the initial keyring files for Consul to use
                            containing the provided key. The -data-dir argument
                            is required with this option.
  -rpc-addr=127.0.0.1:8400  RPC address of the Consul agent.
`
	return strings.TrimSpace(helpText)
}

func (c *KeyringCommand) Synopsis() string {
	return "Manages gossip layer encryption keys"
}
