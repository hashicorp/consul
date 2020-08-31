package expose

import (
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/intention/create"
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
	ingressGateway string
	service        string
	port           int
	protocol       string
	hosts          flags.AppendSliceValue
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.ingressGateway, "ingress-gateway", "",
		"(Required) The name of the ingress gateway service to use. A namespace "+
			"can optionally be specified as a prefix via the 'namespace/service' format.")

	c.flags.StringVar(&c.service, "service", "",
		"(Required) The name of destination service to expose. A namespace "+
			"can optionally be specified as a prefix via the 'namespace/service' format.")

	c.flags.IntVar(&c.port, "port", 0,
		"(Required) The listener port to use for the service on the Ingress gateway.")

	c.flags.StringVar(&c.protocol, "protocol", "tcp",
		"The protocol for the service. Defaults to 'tcp'.")

	c.flags.Var(&c.hosts, "host", "Additional DNS hostname to use for routing to this service."+
		"Can be specified multiple times.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check for any missing or invalid flag values.
	if c.service == "" {
		c.UI.Error("A service name must be given via the -service flag.")
		return 1
	}
	svc, svcNamespace, err := create.ParseIntentionTarget(c.service)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Invalid service name: %s", err))
		return 1
	}

	if c.ingressGateway == "" {
		c.UI.Error("An ingress gateway service must be given via the -ingress-gateway flag.")
		return 1
	}
	gateway, gatewayNamespace, err := create.ParseIntentionTarget(c.ingressGateway)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Invalid ingress gateway name: %s", err))
		return 1
	}

	if c.port == 0 {
		c.UI.Error("A port must be provided via the -port flag.")
		return 1
	}

	// First get the config entry for the ingress gateway, if it exists. Don't error if it's a 404 as that
	// just means we'll need to create a new config entry.
	conf, _, err := client.ConfigEntries().Get(api.IngressGateway, gateway, nil)
	if err != nil && !strings.Contains(err.Error(), agent.ConfigEntryNotFoundErr) {
		c.UI.Error(fmt.Sprintf("Error fetching existing ingress gateway configuration: %s", err))
		return 1
	}
	if conf == nil {
		conf = &api.IngressGatewayConfigEntry{
			Kind:      api.IngressGateway,
			Name:      gateway,
			Namespace: gatewayNamespace,
		}
	}

	// Make sure the flags don't conflict with existing config.
	ingressConf, ok := conf.(*api.IngressGatewayConfigEntry)
	if !ok {
		// This should never happen
		c.UI.Error(fmt.Sprintf("Config entry is an invalid type: %T", conf))
		return 1
	}

	listenerIdx := -1
	serviceIdx := -1
	newService := api.IngressService{
		Name:      svc,
		Namespace: svcNamespace,
		Hosts:     c.hosts,
	}
	for i, listener := range ingressConf.Listeners {
		// Find the listener for the specified port, if one exists.
		if listener.Port != c.port {
			continue
		}

		// Make sure the given protocol matches the existing one.
		listenerIdx = i
		if listener.Protocol != c.protocol {
			c.UI.Error(fmt.Sprintf("Listener on port %d already configured with conflicting protocol %q", listener.Port, listener.Protocol))
			return 1
		}

		// Make sure the service isn't already exposed in this gateway
		for j, service := range listener.Services {
			if service.Name == svc && namespaceMatch(service.Namespace, svcNamespace) {
				serviceIdx = j
				c.UI.Output(fmt.Sprintf("Updating service definition for %q on listener with port %d", c.service, listener.Port))
				break
			}
		}
	}

	// Add a service to the existing listener for the port if one exists, or make a new listener.
	if listenerIdx >= 0 {
		if serviceIdx >= 0 {
			ingressConf.Listeners[listenerIdx].Services[serviceIdx] = newService
		} else {
			ingressConf.Listeners[listenerIdx].Services = append(ingressConf.Listeners[listenerIdx].Services, newService)
		}
	} else {
		ingressConf.Listeners = append(ingressConf.Listeners, api.IngressListener{
			Port:     c.port,
			Protocol: c.protocol,
			Services: []api.IngressService{newService},
		})
	}

	// Write the updated config entry using a check-and-set, so it fails if the entry
	// has been changed since we looked it up.
	succeeded, _, err := client.ConfigEntries().CAS(ingressConf, ingressConf.GetModifyIndex(), nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing ingress config entry: %v", err))
		return 1
	}
	if !succeeded {
		c.UI.Error("Ingress config entry was changed while attempting to update, please try again.")
		return 1
	}
	c.UI.Output(fmt.Sprintf("Successfully updated config entry for ingress service %q", gateway))

	// Check for an existing intention.
	existing, _, err := client.Connect().IntentionGetExact(c.ingressGateway, c.service, nil)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error looking up existing intention: %s", err))
		return 1
	}
	if existing != nil && existing.Action == api.IntentionActionAllow {
		c.UI.Output(fmt.Sprintf("Intention already exists for %q -> %q", c.ingressGateway, c.service))
		return 0
	}

	// Add the intention between the gateway service and the destination.
	ixn := &api.Intention{
		SourceName:      gateway,
		SourceNS:        gatewayNamespace,
		DestinationName: svc,
		DestinationNS:   svcNamespace,
		SourceType:      api.IntentionSourceConsul,
		Action:          api.IntentionActionAllow,
	}
	if existing == nil {
		_, _, err = client.Connect().IntentionCreate(ixn, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error creating intention: %s", err))
			return 1
		}
	} else {
		_, err = client.Connect().IntentionUpdate(ixn, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error updating intention: %s", err))
			return 1
		}
	}

	c.UI.Output(fmt.Sprintf("Successfully set up intention for %q -> %q", c.ingressGateway, c.service))
	return 0
}

func namespaceMatch(a, b string) bool {
	namespaceA := a
	namespaceB := b
	if namespaceA == "" {
		namespaceA = structs.IntentionDefaultNamespace
	}
	if namespaceB == "" {
		namespaceB = structs.IntentionDefaultNamespace
	}

	return namespaceA == namespaceB
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Expose a Connect-enabled service through an Ingress gateway"
const help = `
Usage: consul connect expose [options]

  Exposes a Connect-enabled service through the given ingress gateway, using the
  given protocol and port.
`
