package del

import (
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI          cli.Ui
	flags       *flag.FlagSet
	http        *flags.HTTPFlags
	help        string
	cas         bool
	modifyIndex uint64
	recurse     bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.cas, "cas", false,
		"Perform a Check-And-Set operation. Specifying this value also requires "+
			"the -modify-index flag to be set. The default value is false.")
	c.flags.Uint64Var(&c.modifyIndex, "modify-index", 0,
		"Unsigned integer representing the ModifyIndex of the key. This is "+
			"used in combination with the -cas flag.")
	c.flags.BoolVar(&c.recurse, "recurse", false,
		"Recursively delete all keys with the path. The default value is false.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	key := ""

	// Check for arg validation
	args = c.flags.Args()
	switch len(args) {
	case 0:
		key = ""
	case 1:
		key = args[0]
	default:
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// This is just a "nice" thing to do. Since pairs cannot start with a /, but
	// users will likely put "/" or "/foo", lets go ahead and strip that for them
	// here.
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	// If the key is empty and we are not doing a recursive delete, this is an
	// error.
	if key == "" && !c.recurse {
		c.UI.Error("Error! Missing KEY argument")
		return 1
	}

	// ModifyIndex is required for CAS
	if c.cas && c.modifyIndex == 0 {
		c.UI.Error("Cannot delete a key that does not exist. Provide a -modify-index greater than than 0.")
		return 1
	}

	// Specifying a ModifyIndex for a non-CAS operation is not possible.
	if c.modifyIndex != 0 && !c.cas {
		c.UI.Error("Cannot specify -modify-index without -cas!")
		return 1
	}

	// It is not valid to use a CAS and recurse in the same call
	if c.recurse && c.cas {
		c.UI.Error("Cannot specify both -cas and -recurse!")
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	switch {
	case c.recurse:
		if _, err := client.KV().DeleteTree(key, nil); err != nil {
			c.UI.Error(fmt.Sprintf("Error! Did not delete prefix %s: %s", key, err))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Deleted keys with prefix: %s", key))
		return 0
	case c.cas:
		pair := &api.KVPair{
			Key:         key,
			ModifyIndex: c.modifyIndex,
		}

		success, _, err := client.KV().DeleteCAS(pair, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error! Did not delete key %s: %s", key, err))
			return 1
		}
		if !success {
			c.UI.Error(fmt.Sprintf("Error! Did not delete key %s: CAS failed", key))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Deleted key: %s", key))
		return 0
	default:
		if _, err := client.KV().Delete(key, nil); err != nil {
			c.UI.Error(fmt.Sprintf("Error deleting key %s: %s", key, err))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Deleted key: %s", key))
		return 0
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Removes data from the KV store"
	help     = `
Usage: consul kv delete [options] KEY_OR_PREFIX

  Removes the value from Consul's key-value store at the given path. If no
  key exists at the path, no action is taken.

  To delete the value for the key named "foo" in the key-value store:

      $ consul kv delete foo

  To delete all keys which start with "foo", specify the -recurse option:

      $ consul kv delete -recurse foo

  This will delete the keys named "foo", "food", and "foo/bar/zip" if they
  existed.
`
)
