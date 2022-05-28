package create

import (
	"flag"
	"fmt"
	"strings"

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
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	flagLockDelay     string
	flagNode          string
	flagName          string
	flagBehavior      string
	flagTTL           string
	flagNodeChecks    []string
	flagServiceChecks []string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagLockDelay, "lock-delay", "",
		"Specifies the duration for the lock delay.")
	c.flags.StringVar(&c.flagNode, "node", "",
		"Specifies the name of the node.")
	c.flags.StringVar(&c.flagName, "name", "",
		"Specifies a human-readable name for the session.")
	c.flags.StringVar(&c.flagBehavior, "behavior", "",
		"Controls the behavior to take when a session is invalidated.")
	c.flags.StringVar(&c.flagTTL, "ttl", "",
		"Specifies the duration of a session (between 10s and 86400s).")
	c.flags.Var((*flags.AppendSliceValue)(&c.flagNodeChecks), "node-check",
		"Specifies a node health check ID. May be specified multiple times. It is highly recommended that, if you override this list, you include the default `serfHealth`.")

	c.flags.Var((*flags.AppendSliceValue)(&c.flagServiceChecks), "service-check",
		"Specifies a service check. May be specified multiple times. Format is the SERVICECHECKID or SERVICECHECKID:NAMESPACE.")

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

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	serviceChecks := []api.ServiceCheck{}
	for _, sc := range c.flagServiceChecks {
		splits := strings.SplitN(sc, ":", 2)
		serviceCheck := api.ServiceCheck{
			ID: splits[0],
		}
		if len(splits) == 2 {
			serviceCheck.Namespace = splits[1]
		}
		serviceChecks = append(serviceChecks, serviceCheck)
	}

	s := &api.SessionEntry{
		Name:          c.flagName,
		Node:          c.flagNode,
		LockDelay:     0,
		Behavior:      c.flagBehavior,
		TTL:           c.flagTTL,
		NodeChecks:    c.flagNodeChecks,
		ServiceChecks: serviceChecks,
	}
	id, _, err := client.Session().Create(s, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error creating session: %s", err))
		return 1
	}

	c.UI.Info(id)

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Create a new session"
	help     = `
Usage: consul session create [options]

    This endpoint initializes a new session. Sessions must be associated with a
    node and may be associated with any number of checks.

    Create a new session:

        $ consul session create -lock-delay=15s \
                                -name=my-service-lock \
                                -node=foobar \
                                -node-check=serfHealth \
                                -node-check=a \
                                -node-check=b \
                                -node-check=c \
                                -behavior=release \
                                -ttl=30s
`
)
