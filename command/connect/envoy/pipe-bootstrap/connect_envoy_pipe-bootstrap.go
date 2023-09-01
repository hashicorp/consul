// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pipebootstrap

import (
	"bytes"
	"flag"
	"os"
	"time"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	help  string
}

func (c *cmd) init() {
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	// Read from STDIN, write to the named pipe provided in the only positional arg
	if len(args) != 1 {
		c.UI.Error("Expecting named pipe path as argument")
		return 1
	}

	// This should never be alive for very long. In case bad things happen and
	// Envoy never starts limit how long we live before just exiting so we can't
	// accumulate tons of these zombie children.
	time.AfterFunc(10*time.Second, func() {
		// Force cleanup
		os.RemoveAll(args[0])
		os.Exit(99)
	})

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(os.Stdin); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// WRONLY is very important here - the open() call will block until there is a
	// reader (Envoy) if we open it with RDWR though that counts as an opener and
	// we will just send the data to ourselves as the first opener and so only
	// valid reader.
	f, err := os.OpenFile(args[0], os.O_WRONLY|os.O_APPEND, 0700)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if _, err := buf.WriteTo(f); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if err = f.Close(); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Use Warn to send to stderr, because all logs should go to stderr.
	c.UI.Warn("Bootstrap sent, unlinking named pipe")

	// Removed named pipe now we sent it. Even if Envoy has not yet read it, we
	// know it has opened it and has the file descriptor since our write above
	// will block until there is a reader.
	os.RemoveAll(args[0])

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Internal shim for delivering Envoy bootstrap without writing to file system"
const help = `
Usage: should only be used internally by consul connect envoy
`
