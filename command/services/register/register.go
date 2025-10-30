// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package register

import (
	"flag"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/services"
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
	flagKind            string
	flagId              string
	flagName            string
	flagAddress         string
	flagPort            int
	flagSocketPath      string
	flagTags            []string
	flagMeta            map[string]string
	flagTaggedAddresses map[string]string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.flagId, "id", "",
		"ID of the service to register for arg-based registration. If this "+
			"isn't set, it will default to the -name value.")
	c.flags.StringVar(&c.flagName, "name", "",
		"Name of the service to register for arg-based registration.")
	c.flags.StringVar(&c.flagAddress, "address", "",
		"Address of the service to register for arg-based registration.")
	c.flags.IntVar(&c.flagPort, "port", 0,
		"Port of the service to register for arg-based registration.")
	c.flags.StringVar(&c.flagSocketPath, "socket", "",
		"Path to the Unix domain socket to register for arg-based registration (conflicts with address and port).")
	c.flags.Var((*flags.FlagMapValue)(&c.flagMeta), "meta",
		"Metadata to set on the service, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple meta fields.")
	c.flags.Var((*flags.AppendSliceValue)(&c.flagTags), "tag",
		"Tag to add to the service. This flag can be specified multiple "+
			"times to set multiple tags.")
	c.flags.Var((*flags.FlagMapValue)(&c.flagTaggedAddresses), "tagged-address",
		"Tagged address to set on the service, formatted as key=value. This flag "+
			"may be specified multiple times to set multiple addresses.")
	c.flags.StringVar(&c.flagKind, "kind", "", "The services 'kind'")

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

	// Validate service address if provided
	if c.flagAddress != "" {
		if err := validateServiceAddressWithPortCheck(c.flagAddress, false); err != nil {
			c.UI.Error(fmt.Sprintf("Invalid Service address when using CLI flags. Use -port flag instead: %v", err))
			return 1
		}
	}

	var taggedAddrs map[string]api.ServiceAddress
	if len(c.flagTaggedAddresses) > 0 {
		taggedAddrs = make(map[string]api.ServiceAddress)
		for k, v := range c.flagTaggedAddresses {
			addr, err := api.ParseServiceAddr(v)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Invalid Tagged address: %v", err))
				return 1
			}
			// Validate the address part of the tagged address
			if err := validateServiceAddressWithPortCheck(addr.Address, true); err != nil {
				c.UI.Error(fmt.Sprintf("Invalid Tagged address for tagged address '%s': %v", k, err))
				return 1
			}
			taggedAddrs[k] = addr
		}
	}

	svcs := []*api.AgentServiceRegistration{{
		Kind:            api.ServiceKind(c.flagKind),
		ID:              c.flagId,
		Name:            c.flagName,
		Address:         c.flagAddress,
		Port:            c.flagPort,
		SocketPath:      c.flagSocketPath,
		Tags:            c.flagTags,
		Meta:            c.flagMeta,
		TaggedAddresses: taggedAddrs,
	}}

	// Check for arg validation
	args = c.flags.Args()
	if len(args) == 0 && c.flagName == "" {
		c.UI.Error("Service registration requires at least one argument or flags.")
		return 1
	} else if len(args) > 0 && c.flagName != "" {
		c.UI.Error("Service registration requires arguments or -id, not both.")
		return 1
	}

	if len(args) > 0 {
		var err error
		svcs, err = services.ServicesFromFiles(c.UI, args)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error: %s", err))
			return 1
		}
		// Validate addresses in services loaded from files
		for _, svc := range svcs {
			if svc.Address != "" {
				if err := validateServiceAddressWithPortCheck(svc.Address, false); err != nil {
					c.UI.Error(fmt.Sprintf("Invalid Service address for service '%s'. Use port field instead: %v", svc.Name, err))
					return 1
				}
			}
			// Validate tagged addresses
			for tag, addr := range svc.TaggedAddresses {
				if err := validateServiceAddressWithPortCheck(addr.Address, true); err != nil {
					c.UI.Error(fmt.Sprintf("Invalid Tagged address for tagged address '%s' in service '%s': %v", tag, svc.Name, err))
					return 1
				}
			}

			if len(svc.Ports) > 0 && svc.Port != 0 {
				c.UI.Error(fmt.Sprintf("Service '%s' has both 'port' and 'ports' fields set; only one is allowed", svc.Name))
				return 1
			}

			if len(svc.Ports) > 0 && svc.IsConnectEnabled() {
				c.UI.Error("Cannot use 'ports' with Consul Connect. Use 'port' instead.")
				return 1
			}

			if err := svc.Ports.Validate(); err != nil {
				c.UI.Error(fmt.Sprintf("Invalid ports configuration for service '%s': %v", svc.Name, err))
				return 1
			}
		}
	}

	// Create and test the HTTP client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	//Create existingsvcs to test for existing services
	existingsvcs, err := client.Agent().Services()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error checking for existing services %s", err))
	}

	// Create all the services
	for _, svc := range svcs {
		//iterate through existing services to check for duplicate sidecar ports
		for _, existingsvc := range existingsvcs {
			if svc.Connect.SidecarService.Port > 0 && existingsvc.Port > 0 {
				if svc.Connect.SidecarService.Port == existingsvc.Port {
					c.UI.Output(fmt.Sprintf("Error registering service %q. Service %q is using the same sidecar port %d. Please make sidecar ports unique to avoid address binding issues in Envoy",
						svc.Name, existingsvc.Service, svc.Connect.SidecarService.Port))
					return 1
				}
			}
		}
		if err := client.Agent().ServiceRegister(svc); err != nil {
			c.UI.Error(fmt.Sprintf("Error registering service %q: %s",
				svc.Name, err))
			return 1
		}

		c.UI.Output(fmt.Sprintf("Registered service: %s", svc.Name))
	}

	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	synopsis = "Register services with the local agent"
	help     = `
Usage: consul services register [options] [FILE...]

  Register one or more services using the local agent API. Services can
  be registered from standard Consul configuration files (HCL or JSON) or
  using flags. The service is registered and the command returns. The caller
  must remember to call "consul services deregister" or a similar API to
  deregister the service when complete.

      $ consul services register web.json

  Additional flags and more advanced use cases are detailed below.
`
)

// This function validates that a service address is properly formatted
// and catches common malformed IP patterns
func validateServiceAddress(addr string) error {
	if addr == "" {
		return nil // Empty addresses are allowed
	}

	// Parse the address to separate host and port if present
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If SplitHostPort fails, treat the whole string as host
		host = addr
	}

	// Check if it's a valid IP address
	if ip := net.ParseIP(host); ip != nil {
		// Valid IP - allow all valid IPs including ANY addresses
		return nil
	}

	// If not an IP, it might be a hostname or malformed IP
	// Check for common malformed IP patterns
	if looksLikeIP(host) {
		return fmt.Errorf("malformed IP address: %s", host)
	}

	// If not an IP, assume it's a hostname - validate it's not empty
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("address cannot be empty")
	}

	return nil
}

// This function validates a service address and optionally checks for port presence
func validateServiceAddressWithPortCheck(addr string, allowPort bool) error {

	// Validate the basic address format
	if err := validateServiceAddress(addr); err != nil {
		return err
	}

	// Check for port presence if not allowed
	if !allowPort {
		if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
			return fmt.Errorf("address should not contain port")
		}
	}

	return nil
}

// This function returns true if the string appears to be an IP address
// but fails to parse correctly (indicating it's malformed)
func looksLikeIP(addr string) bool {
	// Check for obviously malformed IP patterns
	if strings.Contains(addr, "..") || strings.Contains(addr, ":::") {
		return true
	}

	// Check for multiple :: sequences (IPv6 can have at most one ::)
	if strings.Count(addr, "::") > 1 {
		return true
	}

	// Check for too many colons (IPv6 can have at most 7)
	if strings.Count(addr, ":") > 7 {
		return true
	}

	// Check for IPv4-like patterns with too many dots
	if strings.Count(addr, ".") > 3 {
		// Check if most segments are numeric, which may indicate a malformed IP
		parts := strings.Split(addr, ".")
		numericParts := 0
		for _, part := range parts {
			if part != "" {
				isNumeric := true
				for _, r := range part {
					if r < '0' || r > '9' {
						isNumeric = false
						break
					}
				}
				if isNumeric {
					numericParts++
				}
			}
		}
		// If most parts are numeric, it's likely a malformed IP
		return numericParts > 2
	}

	return false
}
