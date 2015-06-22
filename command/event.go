package command

import (
	"flag"
	"fmt"
	"regexp"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

// EventCommand is a Command implementation that is used to
// fire new events
type EventCommand struct {
	Ui cli.Ui
}

func (c *EventCommand) Help() string {
	helpText := `
Usage: consul event [options] [payload]

  Dispatches a custom user event across a datacenter. An event must provide
  a name, but a payload is optional. Events support filtering using
  regular expressions on node name, service, and tag definitions.

Options:

  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
  -datacenter=""             Datacenter to dispatch in. Defaults to that of agent.
  -name=""                   Name of the event.
  -node=""                   Regular expression to filter on node names
  -service=""                Regular expression to filter on service instances
  -tag=""                    Regular expression to filter on service tags. Must be used
                             with -service.
  -token=""                  ACL token to use during requests. Defaults to that
                             of the agent.
`
	return strings.TrimSpace(helpText)
}

func (c *EventCommand) Run(args []string) int {
	var datacenter, name, node, service, tag, token string
	cmdFlags := flag.NewFlagSet("event", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&datacenter, "datacenter", "", "")
	cmdFlags.StringVar(&name, "name", "", "")
	cmdFlags.StringVar(&node, "node", "", "")
	cmdFlags.StringVar(&service, "service", "", "")
	cmdFlags.StringVar(&tag, "tag", "", "")
	cmdFlags.StringVar(&token, "token", "", "")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Check for a name
	if name == "" {
		c.Ui.Error("Event name must be specified")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	// Validate the filters
	if node != "" {
		if _, err := regexp.Compile(node); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to compile node filter regexp: %v", err))
			return 1
		}
	}
	if service != "" {
		if _, err := regexp.Compile(service); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to compile service filter regexp: %v", err))
			return 1
		}
	}
	if tag != "" {
		if _, err := regexp.Compile(tag); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to compile tag filter regexp: %v", err))
			return 1
		}
	}
	if tag != "" && service == "" {
		c.Ui.Error("Cannot provide tag filter without service filter.")
		return 1
	}

	// Check for a payload
	var payload []byte
	args = cmdFlags.Args()
	switch len(args) {
	case 0:
	case 1:
		payload = []byte(args[0])
	default:
		c.Ui.Error("Too many command line arguments.")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
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

	// Prepare the request
	event := client.Event()
	params := &consulapi.UserEvent{
		Name:          name,
		Payload:       payload,
		NodeFilter:    node,
		ServiceFilter: service,
		TagFilter:     tag,
	}
	opts := &consulapi.WriteOptions{
		Datacenter: datacenter,
		Token:      token,
	}

	// Fire the event
	id, _, err := event.Fire(params, opts)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error firing event: %s", err))
		return 1
	}

	// Write out the ID
	c.Ui.Output(fmt.Sprintf("Event ID: %s", id))
	return 0
}

func (c *EventCommand) Synopsis() string {
	return "Fire a new event"
}
