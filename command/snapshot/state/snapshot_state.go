package state

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/raftutil"
	"github.com/mitchellh/cli"
)

// Define the set of valid keys
var validKeys = map[string]bool{
	"Nodes":              true,
	"Coordinates":        true,
	"Services":           true,
	"GatewayServices":    true,
	"ServiceIntentions":  true,
	"ACLTokens":          true,
	"ACLRoles":           true,
	"ACLPolicies":        true,
	"ACLAuthMethods":     true,
	"ACLBindingRules":    true,
	"KVs":                true,
	"ConfigEntries":      true,
	"ConnectCAConfig":    true,
	"ConnectCARoots":     true,
	"ConnectCALeafCerts": true,
}

// Run (snapshot state):
// Dump state obtained from StateAsMap into a JSON format and print to stdout
func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	filters := strings.Split(c.filterExpr, ",")
	// Validate filters
	if len(filters) > 1 {
		for _, filter := range filters {
			if _, ok := validKeys[filter]; !ok {
				c.UI.Error(fmt.Sprintf("Invalid filter parameter passed: %q | Valid Filters: %s", filter, stringValidKeys()))
				return 1
			}
		}
	} else {
		if _, ok := validKeys[filters[0]]; !ok {
			c.UI.Error(fmt.Sprintf("Invalid filter parameter passed: %q | Valid Filters: %s", filters, stringValidKeys()))
			return 1
		}
	}

	// Check that we either got no filename or exactly one.
	if len(c.flags.Args()) != 1 {
		c.UI.Error("This command takes one argument: <file>")
		return 1
	}

	path := c.flags.Args()[0]
	snapshotFile, err := os.Open(path)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer snapshotFile.Close()

	state, meta, err := raftutil.RestoreFromArchive(snapshotFile)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to read archive file: %s", err))
		return 1
	}

	sm := raftutil.StateAsMap(state, filters...)
	sm["SnapshotMeta"] = []interface{}{meta}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(sm); err != nil {
		c.UI.Error(fmt.Sprintf("Failed to encode output: %v", err))
		return 1
	}

	return 0
}

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	help  string

	// flags
	format     string
	filterExpr string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.format, "format", PrettyFormat, fmt.Sprintf("Output format {%s}", strings.Join(GetSupportedFormats(), "|")))
	c.flags.StringVar(
		&c.filterExpr,
		"filter",
		"", "pass in filter string(s) for parsing snapshot state store")
	c.help = flags.Usage(help, c.flags)
}

func stringValidKeys() string {
	keys := make([]string, 0, len(validKeys))
	for k := range validKeys {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Displays information about a Consul snapshot file captured state"
const help = `
Usage: consul snapshot state [options] <file>

  Displays a JSON representation of state in the snapshot.

  To inspect the file "backup.snap":

    $ consul snapshot state backup.snap

`
