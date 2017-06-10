package command

import (
	"fmt"
	"regexp"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
)

// EventCommand is a Command implementation that is used to
// fire new events
type EventCommand struct {
	BaseCommand
}

func (c *EventCommand) Help() string {
	helpText := `
Usage: consul event [options] [payload]

  Dispatches a custom user event across a datacenter. An event must provide
  a name, but a payload is optional. Events support filtering using
  regular expressions on node name, service, and tag definitions.

` + c.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func (c *EventCommand) Run(args []string) int {
	var name, node, service, tag string

	f := c.BaseCommand.NewFlagSet(c)
	f.StringVar(&name, "name", "",
		"Name of the event.")
	f.StringVar(&node, "node", "",
		"Regular expression to filter on node names.")
	f.StringVar(&service, "service", "",
		"Regular expression to filter on service instances.")
	f.StringVar(&tag, "tag", "",
		"Regular expression to filter on service tags. Must be used with -service.")

	if err := c.BaseCommand.Parse(args); err != nil {
		return 1
	}

	// Check for a name
	if name == "" {
		c.UI.Error("Event name must be specified")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Validate the filters
	if node != "" {
		if _, err := regexp.Compile(node); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile node filter regexp: %v", err))
			return 1
		}
	}
	if service != "" {
		if _, err := regexp.Compile(service); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile service filter regexp: %v", err))
			return 1
		}
	}
	if tag != "" {
		if _, err := regexp.Compile(tag); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to compile tag filter regexp: %v", err))
			return 1
		}
	}
	if tag != "" && service == "" {
		c.UI.Error("Cannot provide tag filter without service filter.")
		return 1
	}

	// Check for a payload
	var payload []byte
	args = f.Args()
	switch len(args) {
	case 0:
	case 1:
		payload = []byte(args[0])
	default:
		c.UI.Error("Too many command line arguments.")
		c.UI.Error("")
		c.UI.Error(c.Help())
		return 1
	}

	// Create and test the HTTP client
	client, err := c.BaseCommand.HTTPClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Agent().NodeName()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}

	// Prepare the request
	event := client.Event()
	params := &consulapi.UserEvent{
		Name:          name,
		Payload:       payload,
		NodeFilter:    node,
		ServiceFilter: service,
		TagFilter:     tag,
	}

	// Fire the event
	id, _, err := event.Fire(params, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error firing event: %s", err))
		return 1
	}

	// Write out the ID
	c.UI.Output(fmt.Sprintf("Event ID: %s", id))
	return 0
}

func (c *EventCommand) Synopsis() string {
	return "Fire a new event"
}
