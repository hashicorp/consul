package command

import (
	"fmt"
	"sync"
)

// MonitorCommand is a Command implementation that queries a running
// Consul agent what members are part of the cluster currently.
type MonitorCommand struct {
	BaseCommand

	ShutdownCh <-chan struct{}

	lock     sync.Mutex
	quitting bool

	// flags
	logLevel string
}

func (c *MonitorCommand) initFlags() {
	c.InitFlagSet()
	c.FlagSet.StringVar(&c.logLevel, "log-level", "INFO",
		"Log level of the agent.")
}

func (c *MonitorCommand) Help() string {
	c.initFlags()
	return c.HelpCommand(`
Usage: consul monitor [options]

  Shows recent log messages of a Consul agent, and attaches to the agent,
  outputting log messages as they occur in real time. The monitor lets you
  listen for log levels that may be filtered out of the Consul agent. For
  example your agent may only be logging at INFO level, but with the monitor
  you can see the DEBUG level logs.

`)
}

func (c *MonitorCommand) Run(args []string) int {
	c.initFlags()
	if err := c.FlagSet.Parse(args); err != nil {
		return 1
	}

	client, err := c.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	eventDoneCh := make(chan struct{})
	logCh, err := client.Agent().Monitor(c.logLevel, eventDoneCh, nil)
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
	case <-c.ShutdownCh:
		c.lock.Lock()
		c.quitting = true
		c.lock.Unlock()
	}

	return 0
}

func (c *MonitorCommand) Synopsis() string {
	return "Stream logs from a Consul agent"
}
