// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package redirecttraffic

import (
	"flag"
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"
)

func New(ui cli.Ui) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{
		UI: ui,
	}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	client *api.Client

	// Flags.
	nodeName             string
	consulDNSIP          string
	consulDNSPort        int
	proxyUID             string
	proxyID              string
	proxyInboundPort     int
	proxyOutboundPort    int
	excludeInboundPorts  []string
	excludeOutboundPorts []string
	excludeOutboundCIDRs []string
	excludeUIDs          []string
	netNS                string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.nodeName, "node-name", "",
		"The node name where the proxy service is registered. It requires proxy-id to be specified. This is needed if running in an environment without client agents.")
	c.flags.StringVar(&c.consulDNSIP, "consul-dns-ip", "", "IP used to reach Consul DNS. If provided, DNS queries will be redirected to Consul.")
	c.flags.IntVar(&c.consulDNSPort, "consul-dns-port", 0, "Port used to reach Consul DNS. If provided, DNS queries will be redirected to Consul.")
	c.flags.StringVar(&c.proxyUID, "proxy-uid", "", "The user ID of the proxy to exclude from traffic redirection.")
	c.flags.StringVar(&c.proxyID, "proxy-id", "", "The service ID of the proxy service registered with Consul.")
	c.flags.IntVar(&c.proxyInboundPort, "proxy-inbound-port", 0, "The inbound port that the proxy is listening on.")
	c.flags.IntVar(&c.proxyOutboundPort, "proxy-outbound-port", iptables.DefaultTProxyOutboundPort,
		"The outbound port that the proxy is listening on. When not provided, 15001 is used by default.")
	c.flags.Var((*flags.AppendSliceValue)(&c.excludeInboundPorts), "exclude-inbound-port",
		"Inbound port to exclude from traffic redirection. May be provided multiple times.")
	c.flags.Var((*flags.AppendSliceValue)(&c.excludeOutboundPorts), "exclude-outbound-port",
		"Outbound port to exclude from traffic redirection. May be provided multiple times.")
	c.flags.Var((*flags.AppendSliceValue)(&c.excludeOutboundCIDRs), "exclude-outbound-cidr",
		"Outbound CIDR to exclude from traffic redirection. May be provided multiple times.")
	c.flags.Var((*flags.AppendSliceValue)(&c.excludeUIDs), "exclude-uid",
		"Additional user ID to exclude from traffic redirection. May be provided multiple times.")
	c.flags.StringVar(&c.netNS, "netns", "", "The network namespace where traffic redirection rules should apply."+
		"This must be a path to the network namespace, e.g. /var/run/netns/foo.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	if c.proxyUID == "" {
		c.UI.Error("-proxy-uid is required")
		return 1
	}

	if c.proxyID == "" && c.proxyInboundPort == 0 {
		c.UI.Error("either -proxy-id or -proxy-inbound-port are required")
		return 1
	}

	if c.proxyID != "" && (c.proxyInboundPort != 0 || c.proxyOutboundPort != iptables.DefaultTProxyOutboundPort) {
		c.UI.Error("-proxy-inbound-port or -proxy-outbound-port cannot be provided together with -proxy-id. " +
			"Proxy's inbound and outbound ports are retrieved from the proxy's configuration instead.")
		return 1
	}

	cfg, err := c.generateConfigFromFlags()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed to create configuration to apply traffic redirection rules: %s", err))
		return 1
	}

	err = iptables.Setup(cfg)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error setting up traffic redirection rules: %s", err.Error()))
		return 1
	}

	c.UI.Info("Successfully applied traffic redirection rules")
	return 0
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

// trafficRedirectProxyConfig is a snippet of xds/config.go
// with only the configuration values that we need to parse from Proxy.Config
// to apply traffic redirection rules.
type trafficRedirectProxyConfig struct {
	BindPort           int    `mapstructure:"bind_port"`
	PrometheusBindAddr string `mapstructure:"envoy_prometheus_bind_addr"`
	StatsBindAddr      string `mapstructure:"envoy_stats_bind_addr"`
}

// generateConfigFromFlags generates iptables.Config based on command flags.
func (c *cmd) generateConfigFromFlags() (iptables.Config, error) {
	cfg := iptables.Config{
		ConsulDNSIP:       c.consulDNSIP,
		ConsulDNSPort:     c.consulDNSPort,
		ProxyUserID:       c.proxyUID,
		ProxyInboundPort:  c.proxyInboundPort,
		ProxyOutboundPort: c.proxyOutboundPort,
		NetNS:             c.netNS,
	}

	// When proxyID is provided, we set up cfg with values
	// from proxy's service registration in Consul.
	if c.proxyID != "" {
		var err error
		if c.client == nil {
			c.client, err = c.http.APIClient()
			if err != nil {
				return iptables.Config{}, fmt.Errorf("error creating Consul API client: %s", err)
			}
		}

		var svc *api.AgentService
		if c.nodeName == "" {
			svc, _, err = c.client.Agent().Service(c.proxyID, nil)
			if err != nil {
				return iptables.Config{}, fmt.Errorf("failed to fetch proxy service from Consul Agent: %s", err)
			}
		} else {
			svcList, _, err := c.client.Catalog().NodeServiceList(c.nodeName, &api.QueryOptions{
				Filter:             fmt.Sprintf("ID == %q", c.proxyID),
				MergeCentralConfig: true,
			})
			if err != nil {
				return iptables.Config{}, fmt.Errorf("failed to fetch proxy service from Consul: %s", err)
			}
			if len(svcList.Services) < 1 {
				return iptables.Config{}, fmt.Errorf("proxy service with ID %q not found", c.proxyID)
			}
			if len(svcList.Services) > 1 {
				return iptables.Config{}, fmt.Errorf("expected to find only one proxy service with ID %q, but more were found", c.proxyID)
			}
			svc = svcList.Services[0]
		}

		if svc.Proxy == nil {
			return iptables.Config{}, fmt.Errorf("service %s is not a proxy service", c.proxyID)
		}

		// Decode proxy's opaque config so that we can use it later to configure
		// traffic redirection with iptables.
		var trCfg trafficRedirectProxyConfig
		if err := mapstructure.WeakDecode(svc.Proxy.Config, &trCfg); err != nil {
			return iptables.Config{}, fmt.Errorf("failed parsing Proxy.Config: %s", err)
		}

		// Set the proxy's inbound port.
		cfg.ProxyInboundPort = svc.Port
		if trCfg.BindPort != 0 {
			cfg.ProxyInboundPort = trCfg.BindPort
		}

		// Set the proxy's outbound port.
		cfg.ProxyOutboundPort = iptables.DefaultTProxyOutboundPort
		if svc.Proxy.TransparentProxy != nil && svc.Proxy.TransparentProxy.OutboundListenerPort != 0 {
			cfg.ProxyOutboundPort = svc.Proxy.TransparentProxy.OutboundListenerPort
		}

		// Exclude envoy_prometheus_bind_addr port from inbound redirection rules.
		if trCfg.PrometheusBindAddr != "" {
			_, port, err := net.SplitHostPort(trCfg.PrometheusBindAddr)
			if err != nil {
				return iptables.Config{}, fmt.Errorf("failed parsing host and port from envoy_prometheus_bind_addr: %s", err)
			}

			cfg.ExcludeInboundPorts = append(cfg.ExcludeInboundPorts, port)
		}

		// Exclude envoy_stats_bind_addr port from inbound redirection rules.
		if trCfg.StatsBindAddr != "" {
			_, port, err := net.SplitHostPort(trCfg.StatsBindAddr)
			if err != nil {
				return iptables.Config{}, fmt.Errorf("failed parsing host and port from envoy_stats_bind_addr: %s", err)
			}

			cfg.ExcludeInboundPorts = append(cfg.ExcludeInboundPorts, port)
		}

		// Exclude the ListenerPort from Expose configs from inbound traffic redirection.
		for _, exposePath := range svc.Proxy.Expose.Paths {
			if exposePath.ListenerPort != 0 {
				cfg.ExcludeInboundPorts = append(cfg.ExcludeInboundPorts, strconv.Itoa(exposePath.ListenerPort))
			}
		}

		// Exclude any exposed health check ports when Proxy.Expose.Checks is true and nodeName is not provided.
		if svc.Proxy.Expose.Checks && c.nodeName == "" {
			// Get the health checks of the destination service.
			checks, err := c.client.Agent().ChecksWithFilter(fmt.Sprintf("ServiceName == %q", svc.Proxy.DestinationServiceName))
			if err != nil {
				return iptables.Config{}, err
			}

			for _, check := range checks {
				if check.ExposedPort != 0 {
					cfg.ExcludeInboundPorts = append(cfg.ExcludeInboundPorts, strconv.Itoa(check.ExposedPort))
				}
			}
		}
	}

	for _, port := range c.excludeInboundPorts {
		cfg.ExcludeInboundPorts = append(cfg.ExcludeInboundPorts, port)
	}

	for _, port := range c.excludeOutboundPorts {
		cfg.ExcludeOutboundPorts = append(cfg.ExcludeOutboundPorts, port)
	}

	for _, cidr := range c.excludeOutboundCIDRs {
		cfg.ExcludeOutboundCIDRs = append(cfg.ExcludeOutboundCIDRs, cidr)
	}

	for _, uid := range c.excludeUIDs {
		cfg.ExcludeUIDs = append(cfg.ExcludeUIDs, uid)
	}

	return cfg, nil
}

const (
	synopsis = "Applies iptables rules for traffic redirection"
	help     = `
Usage: consul connect redirect-traffic [options]

  Applies iptables rules for inbound and outbound traffic redirection.

  Requires that the iptables command line utility is installed.

  Examples:

    $ consul connect redirect-traffic -proxy-uid 1234 -proxy-id web

    $ consul connect redirect-traffic -proxy-uid 1234 -proxy-inbound-port 20000
`
)
