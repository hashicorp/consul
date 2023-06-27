// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // Expose pprof if configured
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	proxyImpl "github.com/hashicorp/consul/connect/proxy"
	"github.com/hashicorp/consul/logging"
)

func New(ui cli.Ui, shutdownCh <-chan struct{}) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

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

	logger hclog.Logger

	// flags
	logLevel    string
	logJSON     bool
	cfgFile     string
	proxyID     string
	sidecarFor  string
	pprofAddr   string
	service     string
	serviceAddr string
	upstreams   map[string]proxyImpl.UpstreamConfig
	listen      string
	register    bool
	registerId  string

	// test flags
	testNoStart bool // don't start the proxy, just exit 0
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.proxyID, "proxy-id", "",
		"The proxy's ID on the local agent.")

	c.flags.StringVar(&c.sidecarFor, "sidecar-for", "",
		"The ID of a service instance on the local agent that this proxy should "+
			"become a sidecar for. It requires that the proxy service is registered "+
			"with the agent as a connect-proxy with Proxy.DestinationServiceID set "+
			"to this value. If more than one such proxy is registered it will fail.")

	c.flags.StringVar(&c.logLevel, "log-level", "INFO",
		"Specifies the log level.")

	c.flags.BoolVar(&c.logJSON, "log-json", false,
		"Output logs in JSON format.")

	c.flags.StringVar(&c.pprofAddr, "pprof-addr", "",
		"Enable debugging via pprof. Providing a host:port (or just ':port') "+
			"enables profiling HTTP endpoints on that address.")

	c.flags.StringVar(&c.service, "service", "",
		"Name of the service this proxy is representing.")

	c.flags.Var((*FlagUpstreams)(&c.upstreams), "upstream",
		"Upstream service to support connecting to. The format should be "+
			"'name:addr', such as 'db:8181'. This will make 'db' available "+
			"on port 8181. This can be repeated multiple times.")

	c.flags.StringVar(&c.serviceAddr, "service-addr", "",
		"Address of the local service to proxy. Only useful if -listen "+
			"and -service are both set.")

	c.flags.StringVar(&c.listen, "listen", "",
		"Address to listen for inbound connections to the proxied service. "+
			"Must be specified with -service and -service-addr.")

	c.flags.BoolVar(&c.register, "register", false,
		"Self-register with the local Consul agent. Only useful with "+
			"-listen.")

	c.flags.StringVar(&c.registerId, "register-id", "",
		"ID suffix for the service. Use this to disambiguate with other proxies.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	if len(c.flags.Args()) > 0 {
		c.UI.Error(fmt.Sprintf("Should have no non-flag arguments."))
		return 1
	}

	// Load the proxy ID and token from env vars if they're set
	if c.proxyID == "" {
		c.proxyID = os.Getenv("CONNECT_PROXY_ID")
	}
	if c.sidecarFor == "" {
		c.sidecarFor = os.Getenv("CONNECT_SIDECAR_FOR")
	}

	// Setup the log outputs
	logConfig := logging.Config{
		LogLevel: c.logLevel,
		Name:     logging.Proxy,
		LogJSON:  c.logJSON,
	}

	logGate := logging.GatedWriter{Writer: &cli.UiWriter{Ui: c.UI}}

	logger, err := logging.Setup(logConfig, &logGate)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.logger = logger

	// Enable Pprof if needed
	if c.pprofAddr != "" {
		go func() {
			c.UI.Output(fmt.Sprintf("Starting pprof HTTP endpoints on "+
				"http://%s/debug/pprof", c.pprofAddr))
			log.Fatal(http.ListenAndServe(c.pprofAddr, nil))
		}()
	}

	// Setup Consul client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// Output this first since the config watcher below will output
	// other information.
	c.UI.Output("Consul Connect proxy starting...")

	// Get the proper configuration watcher
	cfgWatcher, err := c.configWatcher(client)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error preparing configuration: %s", err))
		return 1
	}

	p, err := proxyImpl.New(client, cfgWatcher, c.logger)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed initializing proxy: %s", err))
		return 1
	}

	// Hook the shutdownCh up to close the proxy
	go func() {
		<-c.shutdownCh
		p.Close()
	}()

	// Register the service if we requested it
	if c.register {
		monitor, err := c.registerMonitor(client)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed initializing registration: %s", err))
			return 1
		}

		go monitor.Run()
		defer monitor.Close()
	}

	c.UI.Info("")
	c.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Run the proxy unless our tests require we don't
	if !c.testNoStart {
		if err := p.Serve(); err != nil {
			c.UI.Error(fmt.Sprintf("Failed running proxy: %s", err))
		}
	}

	c.UI.Output("Consul Connect proxy shutdown")
	return 0
}

func (c *cmd) lookupProxyIDForSidecar(client *api.Client) (string, error) {
	return LookupProxyIDForSidecar(client, c.sidecarFor)
}

// LookupProxyIDForSidecar finds candidate local proxy registrations that are a
// sidecar for the given service. It will return an ID if and only if there is
// exactly one registered connect proxy with `Proxy.DestinationServiceID` set to
// the specified service ID.
//
// This is exported to share it with the connect envoy command.
func LookupProxyIDForSidecar(client *api.Client, sidecarFor string) (string, error) {
	svcs, err := client.Agent().Services()
	if err != nil {
		return "", fmt.Errorf("Failed looking up sidecar proxy info for %s: %s",
			sidecarFor, err)
	}

	var proxyIDs []string
	for _, svc := range svcs {
		if svc.Kind == api.ServiceKindConnectProxy && svc.Proxy != nil &&
			strings.EqualFold(svc.Proxy.DestinationServiceID, sidecarFor) {
			proxyIDs = append(proxyIDs, svc.ID)
		}
	}

	if len(proxyIDs) == 0 {
		return "", fmt.Errorf("No sidecar proxy registered for %s", sidecarFor)
	}
	if len(proxyIDs) > 1 {
		return "", fmt.Errorf("More than one sidecar proxy registered for %s.\n"+
			"    Start proxy with -proxy-id and one of the following IDs: %s",
			sidecarFor, strings.Join(proxyIDs, ", "))
	}
	return proxyIDs[0], nil
}

// LookupGatewayProxy finds the gateway service registered with the local
// agent. If exactly one gateway exists it will be returned, otherwise an error
// is returned.
func LookupGatewayProxy(client *api.Client, kind api.ServiceKind) (*api.AgentService, error) {
	svcs, err := client.Agent().ServicesWithFilter(fmt.Sprintf("Kind == `%s`", kind))
	if err != nil {
		return nil, fmt.Errorf("Failed looking up %s instances: %v", kind, err)
	}

	switch len(svcs) {
	case 0:
		return nil, fmt.Errorf("No %s services registered with this agent", kind)
	case 1:
		for _, svc := range svcs {
			return svc, nil
		}
		return nil, fmt.Errorf("This should be unreachable")
	default:
		return nil, fmt.Errorf("Cannot lookup the %s's proxy ID because multiple are registered with the agent", kind)
	}
}

func (c *cmd) configWatcher(client *api.Client) (proxyImpl.ConfigWatcher, error) {
	// Use the configured proxy ID
	if c.proxyID != "" {
		c.UI.Info("Configuration mode: Agent API")
		c.UI.Info(fmt.Sprintf("          Proxy ID: %s", c.proxyID))
		return proxyImpl.NewAgentConfigWatcher(client, c.proxyID, c.logger)
	}

	if c.sidecarFor != "" {
		// Running as a sidecar, we need to find the proxy-id for the requested
		// service
		var err error
		c.proxyID, err = c.lookupProxyIDForSidecar(client)
		if err != nil {
			return nil, err
		}

		c.UI.Info("Configuration mode: Agent API")
		c.UI.Info(fmt.Sprintf("    Sidecar for ID: %s", c.sidecarFor))
		c.UI.Info(fmt.Sprintf("          Proxy ID: %s", c.proxyID))
		return proxyImpl.NewAgentConfigWatcher(client, c.proxyID, c.logger)
	}

	// Otherwise, we're representing a manually specified service.
	if c.service == "" {
		return nil, fmt.Errorf(
			"-service or -proxy-id must be specified so that proxy can " +
				"configure itself.")
	}

	c.UI.Info("Configuration mode: Flags")
	c.UI.Info(fmt.Sprintf("           Service: %s", c.service))

	// Convert our upstreams to a slice of configurations. We do this
	// deterministically by alphabetizing the upstream keys. We do this so
	// that tests can compare the upstream values.
	upstreamKeys := make([]string, 0, len(c.upstreams))
	for k := range c.upstreams {
		upstreamKeys = append(upstreamKeys, k)
	}
	sort.Strings(upstreamKeys)
	upstreams := make([]proxyImpl.UpstreamConfig, 0, len(c.upstreams))
	for _, k := range upstreamKeys {
		config := c.upstreams[k]

		addr := config.LocalBindSocketPath
		if addr == "" {
			addr = fmt.Sprintf(
				"%s:%d",
				config.LocalBindAddress, config.LocalBindPort)
		}

		c.UI.Info(fmt.Sprintf(
			"          Upstream: %s => %s",
			k, addr))
		upstreams = append(upstreams, config)
	}

	// Parse out our listener if we have one
	var listener proxyImpl.PublicListenerConfig
	if c.listen != "" {
		host, port, err := c.listenParts()
		if err != nil {
			return nil, err
		}

		if c.serviceAddr == "" {
			return nil, fmt.Errorf(
				"-service-addr must be specified with -listen so the proxy " +
					"knows the backend service address.")
		}

		c.UI.Info(fmt.Sprintf("   Public listener: %s:%d => %s", host, port, c.serviceAddr))
		listener.BindAddress = host
		listener.BindPort = port
		listener.LocalServiceAddress = c.serviceAddr
	} else {
		c.UI.Info(fmt.Sprintf("   Public listener: Disabled"))
	}

	return proxyImpl.NewStaticConfigWatcher(&proxyImpl.Config{
		ProxiedServiceName: c.service,
		PublicListener:     listener,
		Upstreams:          upstreams,
	}), nil
}

// registerMonitor returns the registration monitor ready to be started.
func (c *cmd) registerMonitor(client *api.Client) (*RegisterMonitor, error) {
	if c.service == "" || c.listen == "" {
		return nil, fmt.Errorf("-register may only be specified with -service and -listen")
	}

	host, port, err := c.listenParts()
	if err != nil {
		return nil, err
	}

	m := NewRegisterMonitor(c.logger)
	m.Logger = c.logger
	m.Client = client
	m.Service = c.service
	m.IDSuffix = c.registerId
	m.LocalAddress = host
	m.LocalPort = port
	return m, nil
}

// listenParts returns the host and port parts of the -listen flag. The
// -listen flag must be non-empty prior to calling this.
func (c *cmd) listenParts() (string, int, error) {
	host, portRaw, err := net.SplitHostPort(c.listen)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.ParseInt(portRaw, 0, 0)
	if err != nil {
		return "", 0, err
	}

	return host, int(port), nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Runs a Consul Connect proxy"
const help = `
Usage: consul connect proxy [options]

  Starts a Consul Connect proxy and runs until an interrupt is received.
  The proxy can be used to accept inbound connections for a service,
  wrap outbound connections to upstream services, or both. This enables
  a non-Connect-aware application to use Connect.

  The proxy requires service:write permissions for the service it represents.
  The token may be passed via the CLI or the CONSUL_HTTP_TOKEN environment
  variable.

  Consul can automatically start and manage this proxy by specifying the
  "proxy" configuration within your service definition.

  The example below shows how to start a local proxy for establishing outbound
  connections to "db" representing the frontend service. Once running, any
  process that creates a TCP connection to the specified port (8181) will
  establish a mutual TLS connection to "db" identified as "frontend".

    $ consul connect proxy -service frontend -upstream db:8181

  The next example starts a local proxy that also accepts inbound connections
  on port 8443, authorizes the connection, then proxies it to port 8080:

    $ consul connect proxy \
        -service frontend \
        -service-addr 127.0.0.1:8080 \
        -listen ':8443'

  A proxy can accept both inbound connections as well as proxy to upstream
  services by specifying both the "-listen" and "-upstream" flags.

`
