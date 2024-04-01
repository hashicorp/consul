// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package put

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
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
	cas           bool
	kvflags       uint64
	base64encoded bool
	modifyIndex   uint64
	session       string
	acquire       bool
	release       bool

	// testStdin is the input for testing.
	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.cas, "cas", false,
		"Perform a Check-And-Set operation. Specifying this value also "+
			"requires the -modify-index flag to be set. The default value "+
			"is false.")
	c.flags.Uint64Var(&c.kvflags, "flags", 0,
		"Unsigned integer value to assign to this key-value pair. This "+
			"value is not read by Consul, so clients can use this value however "+
			"makes sense for their use case. The default value is 0 (no flags).")
	c.flags.BoolVar(&c.base64encoded, "base64", false,
		"Treat the data as base 64 encoded. The default value is false.")
	c.flags.Uint64Var(&c.modifyIndex, "modify-index", 0,
		"Unsigned integer representing the ModifyIndex of the key. This is "+
			"used in combination with the -cas flag.")
	c.flags.StringVar(&c.session, "session", "",
		"User-defined identifer for this session as a string. This is commonly "+
			"used with the -acquire and -release operations to build robust locking, "+
			"but it can be set on any key. The default value is empty (no session).")
	c.flags.BoolVar(&c.acquire, "acquire", false,
		"Obtain a lock on the key. If the key does not exist, this operation "+
			"will create the key and obtain the lock. The session must already "+
			"exist and be specified via the -session flag. The default value is false.")
	c.flags.BoolVar(&c.release, "release", false,
		"Forfeit the lock on the key at the given path. This requires the "+
			"-session flag to be set. The key must be held by the session in order to "+
			"be unlocked. The default value is false.")

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

	// Check for arg validation
	args = c.flags.Args()
	key, data, err := c.dataFromArgs(args)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error! %s", err))
		return 1
	}

	dataBytes := []byte(data)
	if c.base64encoded {
		dataBytes, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error! Cannot base 64 decode data: %s", err))
		}
	}

	// Session is required for release or acquire
	if (c.release || c.acquire) && c.session == "" {
		c.UI.Error("Error! Missing -session (required with -acquire and -release)")
		return 1
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	pair := &api.KVPair{
		Key:         key,
		ModifyIndex: c.modifyIndex,
		Flags:       c.kvflags,
		Value:       dataBytes,
		Session:     c.session,
	}

	switch {
	case c.cas:
		ok, _, err := client.KV().CAS(pair, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error! Did not write to %s: %s", key, err))
			return 1
		}
		if !ok && c.modifyIndex == 0 {
			c.UI.Error(fmt.Sprintf("Error! Did not write to %s: CAS performed with index=0 and key already exists.", key))
			return 1
		}
		if !ok {
			c.UI.Error(fmt.Sprintf("Error! Did not write to %s: CAS failed", key))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Data written to: %s", key))
		return 0
	case c.acquire:
		ok, _, err := client.KV().Acquire(pair, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error! Failed writing data: %s", err))
			return 1
		}
		if !ok {
			c.UI.Error("Error! Did not acquire lock")
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Lock acquired on: %s", key))
		return 0
	case c.release:
		ok, _, err := client.KV().Release(pair, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error! Failed writing data: %s", key))
			return 1
		}
		if !ok {
			c.UI.Error("Error! Did not release lock")
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Lock released on: %s", key))
		return 0
	default:
		if _, err := client.KV().Put(pair, nil); err != nil {
			c.UI.Error(fmt.Sprintf("Error! Failed writing data: %s", err))
			return 1
		}

		c.UI.Info(fmt.Sprintf("Success! Data written to: %s", key))
		return 0
	}
}

func (c *cmd) dataFromArgs(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		return "", "", fmt.Errorf("Missing KEY argument")
	case 1:
		return args[0], "", nil
	case 2:
	default:
		return "", "", fmt.Errorf("Too many arguments (expected 1 or 2, got %d)", len(args))
	}

	key := args[0]
	data, err := helpers.LoadDataSource(args[1], c.testStdin)

	if err != nil {
		return "", "", err
	} else {
		return key, data, nil
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Sets or updates data in the KV store"
	help     = `
Usage: consul kv put [options] KEY [DATA]

  Writes the data to the given path in the key-value store. The data can be of
  any type.

      $ consul kv put config/redis/maxconns 5

  The data can also be consumed from a file on disk by prefixing with the "@"
  symbol. For example:

      $ consul kv put config/program/license @license.lic

  Or it can be read from stdin using the "-" symbol:

      $ echo "abcd1234" | consul kv put config/program/license -

  The DATA argument itself is optional. If omitted, this will create an empty
  key-value pair at the specified path:

      $ consul kv put webapp/beta/active

  If the -base64 flag is specified, the data will be treated as base 64
  encoded.

  To perform a Check-And-Set operation, specify the -cas flag with the
  appropriate -modify-index flag corresponding to the key you want to perform
  the CAS operation on:

      $ consul kv put -cas -modify-index=844 config/redis/maxconns 5

  Additional flags and more advanced use cases are detailed below.
`
)
