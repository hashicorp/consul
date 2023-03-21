// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package monitor

import (
	"flag"
	"fmt"
	"sync"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
)

// cmd is a Command implementation that queries a running
// Consul agent what members are part of the cluster currently.
type cmd struct {
	UI    cli.Ui
	help  string
	flags *flag.FlagSet
	http  *flags.HTTPFlags

	shutdownCh <-chan struct{}

	lock     sync.Mutex
	quitting bool

	// flags
	logLevel     string
	logJSON      bool
	logSublevels []string
}

func New(ui cli.Ui, shutdownCh <-chan struct{}) *cmd {
	c := &cmd{UI: ui, shutdownCh: shutdownCh}
	c.init()
	return c
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.logLevel, "log-level", "INFO",
		"Log level of the agent.")
	c.flags.BoolVar(&c.logJSON, "log-json", false,
		"Output logs in JSON format.")
	c.flags.Var((*flags.AppendSliceValue)(&c.logSublevels), "log-sublevels",
		"Sets the log level of a subsystem in `<subsystem>:<log-level>` format (e.g. `agent.leader:warn`). Can be specified multiple times.")
	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	var client *api.Client

	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	opts := api.MonitorOptions{
		LogLevel:    c.logLevel,
		LogJson:     c.logJSON,
		LogSublevel: c.logSublevels,
	}
	eventDoneCh := make(chan struct{})
	logCh, err := client.Agent().MonitorWithOpts(eventDoneCh, opts, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error starting monitor: %s", err))
		return 1
	}

	go func() {
		defer close(eventDoneCh)
	OUTER:
		for {
			select {
			case log := <-logCh:
				if log == "" {
					break OUTER
				}
				c.UI.Info(log)
			}
		}

		c.lock.Lock()
		defer c.lock.Unlock()
		if !c.quitting {
			c.UI.Info("")
			c.UI.Output("Remote side ended the monitor! This usually means that the\n" +
				"remote side has exited or crashed.")
		}
	}()

	select {
	case <-eventDoneCh:
		return 1
	case <-c.shutdownCh:
		c.lock.Lock()
		c.quitting = true
		c.lock.Unlock()
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Stream logs from a Consul agent"
const help = `
Usage: consul monitor [options]

  Shows recent log messages of a Consul agent, and attaches to the agent,
  outputting log messages as they occur in real time. The monitor lets you
  listen for log levels that may be filtered out of the Consul agent. For
  example your agent may only be logging at INFO level, but with the monitor
  you can see the DEBUG level logs.
`
