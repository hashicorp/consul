// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/api"
	consulwatch "github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/command/flags"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui, shutdownCh <-chan struct{}) *cmd {
	c := &cmd{UI: ui, shutdownCh: shutdownCh}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	shutdownCh <-chan struct{}

	// flags
	watchType   string
	key         string
	prefix      string
	service     string
	tag         []string
	passingOnly string
	state       string
	name        string
	shell       bool
	filter      string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.watchType, "type", "",
		"Specifies the watch type. One of key, keyprefix, services, nodes, "+
			"service, checks, or event.")
	c.flags.StringVar(&c.key, "key", "",
		"Specifies the key to watch. Only for 'key' type.")
	c.flags.StringVar(&c.prefix, "prefix", "",
		"Specifies the key prefix to watch. Only for 'keyprefix' type.")
	c.flags.StringVar(&c.service, "service", "",
		"Specifies the service to watch. Required for 'service' type, "+
			"optional for 'checks' type.")
	c.flags.Var((*flags.AppendSliceValue)(&c.tag), "tag", "Specifies the service tag(s) to filter on. "+
		"Optional for 'service' type. May be specified multiple times")
	c.flags.StringVar(&c.passingOnly, "passingonly", "",
		"Specifies if only hosts passing all checks are displayed. "+
			"Optional for 'service' type, must be one of `[true|false]`. Defaults false.")
	c.flags.BoolVar(&c.shell, "shell", true,
		"Use a shell to run the command (can set a custom shell via the SHELL "+
			"environment variable).")
	c.flags.StringVar(&c.state, "state", "",
		"Specifies the states to watch. Optional for 'checks' type.")
	c.flags.StringVar(&c.name, "name", "",
		"Specifies an event name to watch. Only for 'event' type.")
	c.flags.StringVar(&c.filter, "filter", "", "Filter to use with the request")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) loadToken() (string, error) {
	httpCfg := api.DefaultConfig()
	c.http.MergeOntoConfig(httpCfg)
	// Trigger the Client init to do any last-minute updates to the Config.
	if _, err := api.NewClient(httpCfg); err != nil {
		return "", err
	}

	return httpCfg.Token, nil
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Check for a type
	if c.watchType == "" {
		c.UI.Error("Watch type must be specified")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	token, err := c.loadToken()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Compile the watch parameters
	params := make(map[string]interface{})
	if c.watchType != "" {
		params["type"] = c.watchType
	}
	if c.http.Datacenter() != "" {
		params["datacenter"] = c.http.Datacenter()
	}
	if token != "" {
		params["token"] = token
	}
	if c.key != "" {
		params["key"] = c.key
	}
	if c.prefix != "" {
		params["prefix"] = c.prefix
	}
	if c.service != "" {
		params["service"] = c.service
	}
	if c.filter != "" {
		params["filter"] = c.filter
	}
	if len(c.tag) > 0 {
		params["tag"] = c.tag
	}
	if c.http.Stale() {
		params["stale"] = c.http.Stale()
	}
	if c.state != "" {
		params["state"] = c.state
	}
	if c.name != "" {
		params["name"] = c.name
	}
	if c.passingOnly != "" {
		b, err := strconv.ParseBool(c.passingOnly)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to parse passingonly flag: %s", err))
			return 1
		}
		params["passingonly"] = b
	}

	// Create the watch
	wp, err := consulwatch.Parse(params)
	if err != nil {
		c.UI.Error(fmt.Sprintf("%s", err))
		return 1
	}

	if strings.HasPrefix(wp.Type, "connect_") || strings.HasPrefix(wp.Type, "agent_") {
		c.UI.Error(fmt.Sprintf("Type %s is not supported in the CLI tool", wp.Type))
		return 1
	}

	// Create and test that the API is accessible before starting a blocking
	// loop for the watch.
	//
	// Consul does not have a /ping endpoint, so the /status/leader endpoint
	// will be used as a substitute since it does not require an ACL token to
	// query, and will always return a response to the client, unless there is a
	// network communication error.
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Status().Leader()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	// Setup handler

	// errExit:
	//	0: false
	//	1: true
	errExit := 0
	if len(c.flags.Args()) == 0 {
		wp.Handler = func(idx uint64, data interface{}) {
			defer wp.Stop()
			buf, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error encoding output: %s", err))
				errExit = 1
			}
			c.UI.Output(string(buf))
		}
	} else {
		wp.Handler = func(idx uint64, data interface{}) {
			doneCh := make(chan struct{})
			defer close(doneCh)
			logFn := func(err error) {
				c.UI.Error(fmt.Sprintf("Warning, could not forward signal: %s", err))
			}

			// Create the command
			var buf bytes.Buffer
			var err error
			var cmd *osexec.Cmd
			if !c.shell {
				cmd, err = exec.Subprocess(c.flags.Args())
			} else {
				cmd, err = exec.Script(strings.Join(c.flags.Args(), " "))
			}
			if err != nil {
				c.UI.Error(fmt.Sprintf("Error executing handler: %s", err))
				goto ERR
			}
			cmd.Env = append(os.Environ(),
				"CONSUL_INDEX="+strconv.FormatUint(idx, 10),
			)

			// Encode the input
			if err = json.NewEncoder(&buf).Encode(data); err != nil {
				c.UI.Error(fmt.Sprintf("Error encoding output: %s", err))
				goto ERR
			}
			cmd.Stdin = &buf
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Run the handler.
			if err := cmd.Start(); err != nil {
				c.UI.Error(fmt.Sprintf("Error starting handler: %s", err))
				goto ERR
			}

			// Set up signal forwarding.
			agent.ForwardSignals(cmd, logFn, doneCh)

			// Wait for the handler to complete.
			if err := cmd.Wait(); err != nil {
				c.UI.Error(fmt.Sprintf("Error executing handler: %s", err))
				goto ERR
			}
			return
		ERR:
			wp.Stop()
			errExit = 1
		}
	}

	// Watch for a shutdown
	go func() {
		<-c.shutdownCh
		wp.Stop()
		os.Exit(0)
	}()

	// Run the watch
	if err := wp.Run(c.http.Addr()); err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	return errExit
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Watch for changes in Consul"
const help = `
Usage: consul watch [options] [child...]

  Watches for changes in a given data view from Consul. If a child process
  is specified, it will be invoked with the latest results on changes. Otherwise,
  the latest values are dumped to stdout and the watch terminates.

  Providing the watch type is required, and other parameters may be required
  or supported depending on the watch type.
`
