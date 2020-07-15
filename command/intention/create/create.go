package create

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/intention"
	"github.com/mitchellh/cli"
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
	flagAllow   bool
	flagDeny    bool
	flagFile    bool
	flagReplace bool
	flagMeta    map[string]string

	// testStdin is the input for testing.
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.flagAllow, "allow", false,
		"Create an intention that allows when matched.")
	c.flags.BoolVar(&c.flagDeny, "deny", false,
		"Create an intention that denies when matched.")
	c.flags.BoolVar(&c.flagFile, "file", false,
		"Read intention data from one or more files.")
	c.flags.BoolVar(&c.flagReplace, "replace", false,
		"Replace matching intentions.")
	c.flags.Var((*flags.FlagMapValue)(&c.flagMeta), "meta",
		"Metadata to set on the intention, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.NamespaceFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Default to allow
	if !c.flagAllow && !c.flagDeny {
		c.flagAllow = true
	}

	// If both are specified it is an error
	if c.flagAllow && c.flagDeny {
		c.UI.Error("Only one of -allow or -deny may be specified.")
		return 1
	}

	// Check for arg validation
	args = c.flags.Args()
	ixns, err := c.ixnsFromArgs(args)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error: %s", err))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Go through and create each intention
	for _, ixn := range ixns {
		// If replace is set to true, then perform an update operation.
		if c.flagReplace {
			oldIxn, _, err := client.Connect().IntentionGetExact(
				intention.FormatSource(ixn),
				intention.FormatDestination(ixn),
				nil,
			)
			if err != nil {
				c.UI.Error(fmt.Sprintf(
					"Error looking up intention for replacement with source %q "+
						"and destination %q: %s",
					intention.FormatSource(ixn),
					intention.FormatDestination(ixn),
					err))
				return 1
			}
			if oldIxn != nil {
				// We set the ID of our intention so we overwrite it
				ixn.ID = oldIxn.ID

				if _, err := client.Connect().IntentionUpdate(ixn, nil); err != nil {
					c.UI.Error(fmt.Sprintf(
						"Error replacing intention with source %q "+
							"and destination %q: %s",
						intention.FormatSource(ixn),
						intention.FormatDestination(ixn),
						err))
					return 1
				}

				// Continue since we don't want to try to insert a new intention
				continue
			}
		}

		_, _, err := client.Connect().IntentionCreate(ixn, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error creating intention %q: %s", ixn, err))
			return 1
		}

		c.UI.Output(fmt.Sprintf("Created: %s", ixn))
	}

	return 0
}

// ParseIntentionTarget parses a target of the form <namespace>/<name> and returns
// the two distinct parts. In some cases the namespace may be elided and this function
// will return the empty string for the namespace then.
func ParseIntentionTarget(input string) (name string, namespace string, err error) {
	// Get the index to the '/'. If it doesn't exist, we have just a name
	// so just set that and return.
	idx := strings.IndexByte(input, '/')
	if idx == -1 {
		// let the agent do token based defaulting of the namespace
		return input, "", nil
	}

	namespace = input[:idx]
	name = input[idx+1:]
	if strings.IndexByte(name, '/') != -1 {
		return "", "", fmt.Errorf("target can contain at most one '/'")
	}

	return name, namespace, nil
}

// ixnsFromArgs returns the set of intentions to create based on the arguments
// given and the flags set. This will call ixnsFromFiles if the -file flag
// was set.
func (c *cmd) ixnsFromArgs(args []string) ([]*api.Intention, error) {
	// If we're in file mode, load from files
	if c.flagFile {
		return c.ixnsFromFiles(args)
	}

	// From args we require exactly two
	if len(args) != 2 {
		return nil, fmt.Errorf("Must specify two arguments: source and destination")
	}

	srcName, srcNamespace, err := ParseIntentionTarget(args[0])
	if err != nil {
		return nil, fmt.Errorf("Invalid intention source: %v", err)
	}

	dstName, dstNamespace, err := ParseIntentionTarget(args[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid intention destination: %v", err)
	}

	return []*api.Intention{{
		SourceNS:        srcNamespace,
		SourceName:      srcName,
		DestinationNS:   dstNamespace,
		DestinationName: dstName,
		SourceType:      api.IntentionSourceConsul,
		Action:          c.ixnAction(),
		Meta:            c.flagMeta,
	}}, nil
}

func (c *cmd) ixnsFromFiles(args []string) ([]*api.Intention, error) {
	var result []*api.Intention
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}

		var ixn api.Intention
		err = json.NewDecoder(f).Decode(&ixn)
		f.Close()
		if err != nil {
			return nil, err
		}

		result = append(result, &ixn)
	}

	return result, nil
}

// ixnAction returns the api.IntentionAction based on the flag set.
func (c *cmd) ixnAction() api.IntentionAction {
	if c.flagAllow {
		return api.IntentionActionAllow
	}

	return api.IntentionActionDeny
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Create intentions for service connections."
const help = `
Usage: consul intention create [options] SRC DST
Usage: consul intention create [options] -file FILE...

  Create one or more intentions. The data can be specified as a single
  source and destination pair or via a set of files when the "-file" flag
  is specified.

      $ consul intention create web db

  To consume data from a set of files:

      $ consul intention create -file one.json two.json

  When specifying the "-file" flag, "-" may be used once to read from stdin:

      $ echo "{ ... }" | consul intention create -file -

  An "allow" intention is created by default (allowlist). To create a
  "deny" intention, the "-deny" flag should be specified.

  If a conflicting intention is found, creation will fail. To replace any
  conflicting intentions, specify the "-replace" flag. This will replace any
  conflicting intentions with the intention specified in this command.
  Metadata and any other fields of the previous intention will not be
  preserved.

  Additional flags and more advanced use cases are detailed below.
`
