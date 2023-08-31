// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package get

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

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
	UI           cli.Ui
	flags        *flag.FlagSet
	http         *flags.HTTPFlags
	help         string
	base64encode bool
	detailed     bool
	keys         bool
	recurse      bool
	separator    string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.base64encode, "base64", false,
		"Base64 encode the value. The default value is false.")
	c.flags.BoolVar(&c.detailed, "detailed", false,
		"Provide additional metadata about the key in addition to the value such "+
			"as the ModifyIndex and any flags that may have been set on the key. "+
			"The default value is false.")
	c.flags.BoolVar(&c.keys, "keys", false,
		"List keys which start with the given prefix, but not their values. "+
			"This is especially useful if you only need the key names themselves. "+
			"This option is commonly combined with the -separator option. The default "+
			"value is false.")
	c.flags.BoolVar(&c.recurse, "recurse", false,
		"Recursively look at all keys prefixed with the given path. The default "+
			"value is false.")
	c.flags.StringVar(&c.separator, "separator", "/",
		"String to use as a separator between keys. The default value is \"/\", "+
			"but this option is only taken into account when paired with the -keys flag.")

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

	// If the key is empty and we are not doing a recursive or key-based lookup,
	// this is an error.
	if key == "" && !(c.recurse || c.keys) {
		c.UI.Error("Error! Missing KEY argument")
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	switch {
	case c.keys && c.recurse:
		pairs, _, err := client.KV().List(key, &api.QueryOptions{
			AllowStale: c.http.Stale(),
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
			return 1
		}

		for i, pair := range pairs {
			if c.detailed {
				var b bytes.Buffer
				if err := prettyKVPair(&b, pair, false, true); err != nil {
					c.UI.Error(fmt.Sprintf("Error rendering KV key: %s", err))
					return 1
				}
				c.UI.Info(b.String())

				if i < len(pairs)-1 {
					c.UI.Info("")
				}
			} else {
				c.UI.Info(fmt.Sprintf("%s", pair.Key))
			}
		}
		return 0
	case c.keys:
		keys, _, err := client.KV().Keys(key, c.separator, &api.QueryOptions{
			AllowStale: c.http.Stale(),
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
			return 1
		}

		for _, k := range keys {
			c.UI.Info(k)
		}

		return 0
	case c.recurse:
		pairs, _, err := client.KV().List(key, &api.QueryOptions{
			AllowStale: c.http.Stale(),
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
			return 1
		}

		for i, pair := range pairs {
			if c.detailed {
				var b bytes.Buffer
				if err := prettyKVPair(&b, pair, c.base64encode, false); err != nil {
					c.UI.Error(fmt.Sprintf("Error rendering KV pair: %s", err))
					return 1
				}

				c.UI.Info(b.String())

				if i < len(pairs)-1 {
					c.UI.Info("")
				}
			} else {
				if c.base64encode {
					c.UI.Info(fmt.Sprintf("%s:%s", pair.Key, base64.StdEncoding.EncodeToString(pair.Value)))
				} else {
					c.UI.Info(fmt.Sprintf("%s:%s", pair.Key, pair.Value))
				}
			}
		}

		return 0
	default:
		pair, _, err := client.KV().Get(key, &api.QueryOptions{
			AllowStale: c.http.Stale(),
		})
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
			return 1
		}

		if pair == nil {
			c.UI.Error(fmt.Sprintf("Error! No key exists at: %s", key))
			return 1
		}

		if c.detailed {
			var b bytes.Buffer
			if err := prettyKVPair(&b, pair, c.base64encode, false); err != nil {
				c.UI.Error(fmt.Sprintf("Error rendering KV pair: %s", err))
				return 1
			}

			c.UI.Info(b.String())
			return 0
		}

		if c.base64encode {
			c.UI.Info(base64.StdEncoding.EncodeToString(pair.Value))
		} else {
			c.UI.Info(string(pair.Value))
		}
		return 0
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

func prettyKVPair(w io.Writer, pair *api.KVPair, base64EncodeValue bool, keysOnly bool) error {
	tw := tabwriter.NewWriter(w, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "CreateIndex\t%d\n", pair.CreateIndex)
	fmt.Fprintf(tw, "Flags\t%d\n", pair.Flags)
	fmt.Fprintf(tw, "Key\t%s\n", pair.Key)
	fmt.Fprintf(tw, "LockIndex\t%d\n", pair.LockIndex)
	fmt.Fprintf(tw, "ModifyIndex\t%d\n", pair.ModifyIndex)
	if pair.Session == "" {
		fmt.Fprint(tw, "Session\t-\n")
	} else {
		fmt.Fprintf(tw, "Session\t%s\n", pair.Session)
	}
	if pair.Partition != "" {
		fmt.Fprintf(tw, "Partition\t%s\n", pair.Partition)
	}
	if pair.Namespace != "" {
		fmt.Fprintf(tw, "Namespace\t%s\n", pair.Namespace)
	}
	if !keysOnly && base64EncodeValue {
		fmt.Fprintf(tw, "Value\t%s", base64.StdEncoding.EncodeToString(pair.Value))
	} else if !keysOnly {
		fmt.Fprintf(tw, "Value\t%s", pair.Value)
	}
	return tw.Flush()
}

const (
	synopsis = "Retrieves or lists data from the KV store"
	help     = `
Usage: consul kv get [options] [KEY_OR_PREFIX]

  Retrieves the value from Consul's key-value store at the given key name. If no
  key exists with that name, an error is returned. If a key exists with that
  name but has no data, nothing is returned. If the name or prefix is omitted,
  it defaults to "" which is the root of the key-value store.

  To retrieve the value for the key named "foo" in the key-value store:

      $ consul kv get foo

  This will return the original, raw value stored in Consul. To view detailed
  information about the key, specify the "-detailed" flag. This will output all
  known metadata about the key including ModifyIndex and any user-supplied
  flags:

      $ consul kv get -detailed foo

  To treat the path as a prefix and list all keys which start with the given
  prefix, specify the "-recurse" flag:

      $ consul kv get -recurse foo

  This will return all key-value pairs. To just list the keys which start with
  the specified prefix, use the "-keys" option instead:

      $ consul kv get -keys foo

  For a full list of options and examples, please see the Consul documentation.
`
)
