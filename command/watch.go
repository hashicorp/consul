package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/watch"
)

// WatchCommand is a Command implementation that is used to setup
// a "watch" which uses a sub-process
type WatchCommand struct {
	BaseCommand
	ShutdownCh <-chan struct{}

	// flags
	watchType   string
	key         string
	prefix      string
	service     string
	tag         string
	passingOnly string
	state       string
	name        string
	shell       bool
}

func (c *WatchCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.StringVar(&c.watchType, "type", "",
		"Specifies the watch type. One of key, keyprefix, services, nodes, "+
			"service, checks, or event.")
	c.FlagSet.StringVar(&c.key, "key", "",
		"Specifies the key to watch. Only for 'key' type.")
	c.FlagSet.StringVar(&c.prefix, "prefix", "",
		"Specifies the key prefix to watch. Only for 'keyprefix' type.")
	c.FlagSet.StringVar(&c.service, "service", "",
		"Specifies the service to watch. Required for 'service' type, "+
			"optional for 'checks' type.")
	c.FlagSet.StringVar(&c.tag, "tag", "",
		"Specifies the service tag to filter on. Optional for 'service' type.")
	c.FlagSet.StringVar(&c.passingOnly, "passingonly", "",
		"Specifies if only hosts passing all checks are displayed. "+
			"Optional for 'service' type, must be one of `[true|false]`. Defaults false.")
	c.FlagSet.BoolVar(&c.shell, "shell", true,
		"Use a shell to run the command (can set a custom shell via the SHELL "+
			"environment variable).")
	c.FlagSet.StringVar(&c.state, "state", "",
		"Specifies the states to watch. Optional for 'checks' type.")
	c.FlagSet.StringVar(&c.name, "name", "",
		"Specifies an event name to watch. Only for 'event' type.")
}

func (c *WatchCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
Usage: consul watch [options] [child...]

  Watches for changes in a given data view from Consul. If a child process
  is specified, it will be invoked with the latest results on changes. Otherwise,
  the latest values are dumped to stdout and the watch terminates.

  Providing the watch type is required, and other parameters may be required
  or supported depending on the watch type.

`)
}

func (c *WatchCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}

	// Check for a type
	if c.watchType == "" {
		c.UI.Error("Watch type must be specified")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Compile the watch parameters
	params := make(map[string]interface{})
	if c.watchType != "" {
		params["type"] = c.watchType
	}
	if c.HTTPDatacenter() != "" {
		params["datacenter"] = c.HTTPDatacenter()
	}
	if c.HTTPToken() != "" {
		params["token"] = c.HTTPToken()
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
	if c.tag != "" {
		params["tag"] = c.tag
	}
	if c.HTTPStale() {
		params["stale"] = c.HTTPStale()
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
	wp, err := watch.Parse(params)
	if err != nil {
		c.UI.Error(fmt.Sprintf("%s", err))
		return 1
	}

	// Create and test the HTTP client
	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Agent().NodeName()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	// Setup handler

	// errExit:
	//	0: false
	//	1: true
	errExit := 0
	if len(c.FlagSet.Args()) == 0 {
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
			var cmd *exec.Cmd
			if !c.shell {
				cmd, err = agent.ExecSubprocess(c.FlagSet.Args())
			} else {
				cmd, err = agent.ExecScript(strings.Join(c.FlagSet.Args(), " "))
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
		<-c.ShutdownCh
		wp.Stop()
		os.Exit(0)
	}()

	// Run the watch
	if err := wp.Run(c.HTTPAddr()); err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	return errExit
}

func (c *WatchCommand) Synopsis() string {
	return "Watch for changes in Consul"
}
