package kvexp

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.initFlags()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
}

func (c *cmd) initFlags() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
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

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	pairs, _, err := client.KV().List(key, &api.QueryOptions{
		AllowStale: c.http.Stale(),
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	exported := make([]*kvExportEntry, len(pairs))
	for i, pair := range pairs {
		exported[i] = toExportEntry(pair)
	}

	marshaled, err := json.MarshalIndent(exported, "", "\t")
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error exporting KV data: %s", err))
		return 1
	}

	c.UI.Info(string(marshaled))

	return 0
}

func (c *cmd) Synopsis() string {
	return "Exports a tree from the KV store as JSON"
}

func (c *cmd) Help() string {
	s := `Usage: consul kv export [KEY_OR_PREFIX]

  Retrieves key-value pairs for the given prefix from Consul's key-value store,
  and writes a JSON representation to stdout. This can be used with the command
  "consul kv import" to move entire trees between Consul clusters.

      $ consul kv export vault

  For a full list of options and examples, please see the Consul documentation.`

	return flags.Usage(s, c.flags, c.http.ClientFlags(), c.http.ServerFlags())
}

type kvExportEntry struct {
	Key   string `json:"key"`
	Flags uint64 `json:"flags"`
	Value string `json:"value"`
}

func toExportEntry(pair *api.KVPair) *kvExportEntry {
	return &kvExportEntry{
		Key:   pair.Key,
		Flags: pair.Flags,
		Value: base64.StdEncoding.EncodeToString(pair.Value),
	}
}
