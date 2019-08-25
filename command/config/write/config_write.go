package write

import (
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
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

	cas         bool
	modifyIndex uint64
	testStdin   io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.http = &flags.HTTPFlags{}
	c.flags.BoolVar(&c.cas, "cas", false,
		"Perform a Check-And-Set operation. Specifying this value also "+
			"requires the -modify-index flag to be set. The default value "+
			"is false.")
	c.flags.Uint64Var(&c.modifyIndex, "modify-index", 0,
		"Unsigned integer representing the ModifyIndex of the config entry. "+
			"This is used in combination with the -cas flag.")
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	args = c.flags.Args()
	if len(args) != 1 {
		c.UI.Error("Must provide exactly one positional argument to specify the config entry to write")
		return 1
	}

	data, err := helpers.LoadDataSourceNoRaw(args[0], c.testStdin)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
		return 1
	}

	entry, err := parseConfigEntry(string(data))
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to decode config entry input: %v", err))
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	entries := client.ConfigEntries()

	written := false
	if c.cas {
		written, _, err = entries.CAS(entry, c.modifyIndex, nil)
	} else {
		written, _, err = entries.Set(entry, nil)
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing config entry %q / %q: %v", entry.GetKind(), entry.GetName(), err))
		return 1
	}

	if !written {
		c.UI.Error(fmt.Sprintf("Config entry %q / %q not updated", entry.GetKind(), entry.GetName()))
		return 1
	}

	// TODO (mkeeler) should we output anything when successful
	return 0
}

func parseConfigEntry(data string) (api.ConfigEntry, error) {
	// parse the data
	var raw map[string]interface{}
	if err := hclDecode(&raw, data); err != nil {
		return nil, fmt.Errorf("Failed to decode config entry input: %v", err)
	}

	return newDecodeConfigEntry(raw)
}

// There is a 'structs' variation of this in
// agent/structs/config_entry.go:DecodeConfigEntry
func newDecodeConfigEntry(raw map[string]interface{}) (api.ConfigEntry, error) {
	var entry api.ConfigEntry

	kindVal, ok := raw["Kind"]
	if !ok {
		kindVal, ok = raw["kind"]
	}
	if !ok {
		return nil, fmt.Errorf("Payload does not contain a kind/Kind key at the top level")
	}

	if kindStr, ok := kindVal.(string); ok {
		newEntry, err := api.MakeConfigEntry(kindStr, "")
		if err != nil {
			return nil, err
		}
		entry = newEntry
	} else {
		return nil, fmt.Errorf("Kind value in payload is not a string")
	}

	skipWhenPatching, translateKeysDict, err := structs.ConfigEntryDecodeRulesForKind(entry.GetKind())
	if err != nil {
		return nil, err
	}

	// lib.TranslateKeys doesn't understand []map[string]interface{} so we have
	// to do this part first.
	raw = lib.PatchSliceOfMaps(raw, skipWhenPatching, nil)

	// CamelCase is the canonical form for these, since this translation
	// happens in the `consul config write` command and the JSON form is sent
	// off to the server.
	lib.TranslateKeys(raw, translateKeysDict)

	var md mapstructure.Metadata
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		Metadata:         &md,
		Result:           &entry,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, err
	}

	for _, k := range md.Unused {
		err = multierror.Append(err, fmt.Errorf("invalid config key %q", k))
	}
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Create or update a centralized config entry"
const help = `
Usage: consul config write [options] <configuration>

  Request a config entry to be created or updated. The configuration
  argument is either a file path or '-' to indicate that the config
  should be read from stdin. The data should be either in HCL or
  JSON form.

  Example (from file):

    $ consul config write web.service.hcl

  Example (from stdin):

    $ consul config write -
`
