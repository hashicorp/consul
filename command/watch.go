package command

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/watch"
	"github.com/mitchellh/cli"
)

// WatchCommand is a Command implementation that is used to setup
// a "watch" which uses a sub-process
type WatchCommand struct {
	ShutdownCh <-chan struct{}
	Ui         cli.Ui
}

func (c *WatchCommand) Help() string {
	helpText := `
Usage: consul watch [options] [child...]

  Watches for changes in a given data view from Consul. If a child process
  is specified, it will be invoked with the latest results on changes. Otherwise,
  the latest values are dumped to stdout and the watch terminates.

  Providing the watch type is required, and other parameters may be required
  or supported depending on the watch type.

Options:

  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
  -datacenter=""             Datacenter to query. Defaults to that of agent.
  -token=""                  ACL token to use. Defaults to that of agent.

Watch Specification:

  -key=val                   Specifies the key to watch. Only for 'key' type.
  -name=val                  Specifies an event name to watch. Only for 'event' type.
  -passingonly=[true|false]  Specifies if only hosts passing all checks are displayed.
                             Optional for 'service' type. Defaults false.
  -prefix=val                Specifies the key prefix to watch. Only for 'keyprefix' type.
  -service=val               Specifies the service to watch. Required for 'service' type,
                             optional for 'checks' type.
  -state=val                 Specifies the states to watch. Optional for 'checks' type.
  -tag=val                   Specifies the service tag to filter on. Optional for 'service'
                             type.
  -type=val                  Specifies the watch type. One of key, keyprefix
                             services, nodes, service, checks, or event.
`
	return strings.TrimSpace(helpText)
}

func (c *WatchCommand) Run(args []string) int {
	var watchType, datacenter, token, key, prefix, service, tag, passingOnly, state, name string
	cmdFlags := flag.NewFlagSet("watch", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&watchType, "type", "", "")
	cmdFlags.StringVar(&datacenter, "datacenter", "", "")
	cmdFlags.StringVar(&token, "token", "", "")
	cmdFlags.StringVar(&key, "key", "", "")
	cmdFlags.StringVar(&prefix, "prefix", "", "")
	cmdFlags.StringVar(&service, "service", "", "")
	cmdFlags.StringVar(&tag, "tag", "", "")
	cmdFlags.StringVar(&passingOnly, "passingonly", "", "")
	cmdFlags.StringVar(&state, "state", "", "")
	cmdFlags.StringVar(&name, "name", "", "")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Check for a type
	if watchType == "" {
		c.Ui.Error("Watch type must be specified")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	// Grab the script to execute if any
	script := strings.Join(cmdFlags.Args(), " ")

	// Compile the watch parameters
	params := make(map[string]interface{})
	if watchType != "" {
		params["type"] = watchType
	}
	if datacenter != "" {
		params["datacenter"] = datacenter
	}
	if token != "" {
		params["token"] = token
	}
	if key != "" {
		params["key"] = key
	}
	if prefix != "" {
		params["prefix"] = prefix
	}
	if service != "" {
		params["service"] = service
	}
	if tag != "" {
		params["tag"] = tag
	}
	if state != "" {
		params["state"] = state
	}
	if name != "" {
		params["name"] = name
	}
	if passingOnly != "" {
		b, err := strconv.ParseBool(passingOnly)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse passingonly flag: %s", err))
			return 1
		}
		params["passingonly"] = b
	}

	// Create the watch
	wp, err := watch.Parse(params)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("%s", err))
		return 1
	}

	// Create and test the HTTP client
	client, err := HTTPClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Agent().NodeName()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	// Setup handler

	// errExit:
	//	0: false
	//	1: true
	errExit := 0
	if script == "" {
		wp.Handler = func(idx uint64, data interface{}) {
			defer wp.Stop()
			buf, err := json.MarshalIndent(data, "", "    ")
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error encoding output: %s", err))
				errExit = 1
			}
			c.Ui.Output(string(buf))
		}
	} else {
		wp.Handler = func(idx uint64, data interface{}) {
			// Create the command
			var buf bytes.Buffer
			var err error
			cmd, err := agent.ExecScript(script)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error executing handler: %s", err))
				goto ERR
			}
			cmd.Env = append(os.Environ(),
				"CONSUL_INDEX="+strconv.FormatUint(idx, 10),
			)

			// Encode the input
			if err = json.NewEncoder(&buf).Encode(data); err != nil {
				c.Ui.Error(fmt.Sprintf("Error encoding output: %s", err))
				goto ERR
			}
			cmd.Stdin = &buf
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Run the handler
			if err := cmd.Run(); err != nil {
				c.Ui.Error(fmt.Sprintf("Error executing handler: %s", err))
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
		<-c.ShutdownCh
		wp.Stop()
		os.Exit(0)
	}()

	// Run the watch
	if err := wp.Run(*httpAddr); err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	return errExit
}

func (c *WatchCommand) Synopsis() string {
	return "Watch for changes in Consul"
}
