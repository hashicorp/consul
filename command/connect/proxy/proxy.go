package proxy

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // Expose pprof if configured
	"os"
	"sort"
	"strconv"

	proxyAgent "github.com/hashicorp/consul/agent/proxy"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	proxyImpl "github.com/hashicorp/consul/connect/proxy"

	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
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

	logFilter *logutils.LevelFilter
	logOutput io.Writer
	logger    *log.Logger

	// flags
	logLevel    string
	cfgFile     string
	proxyID     string
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

	c.flags.StringVar(&c.cfgFile, "dev-config", "",
		"If set, proxy config is read on startup from this file (in HCL or JSON"+
			"format). If a config file is given, the proxy will use that instead of "+
			"querying the local agent for it's configuration. It will not reload it "+
			"except on startup. In this mode the proxy WILL NOT authorize incoming "+
			"connections with the local agent which is totally insecure. This is "+
			"ONLY for internal development and testing and will probably be removed "+
			"once proxy implementation is more complete..")

	c.flags.StringVar(&c.proxyID, "proxy-id", "",
		"The proxy's ID on the local agent.")

	c.flags.StringVar(&c.logLevel, "log-level", "INFO",
		"Specifies the log level.")

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
		c.proxyID = os.Getenv(proxyAgent.EnvProxyID)
	}
	if c.http.Token() == "" {
		c.http.SetToken(os.Getenv(proxyAgent.EnvProxyToken))
	}

	// Setup the log outputs
	logConfig := &logger.Config{
		LogLevel: c.logLevel,
	}
	logFilter, logGate, _, logOutput, ok := logger.Setup(logConfig, c.UI)
	if !ok {
		return 1
	}
	c.logFilter = logFilter
	c.logOutput = logOutput
	c.logger = log.New(logOutput, "", log.LstdFlags)

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

func (c *cmd) configWatcher(client *api.Client) (proxyImpl.ConfigWatcher, error) {
	// Manual configuration file is specified.
	if c.cfgFile != "" {
		cfg, err := proxyImpl.ParseConfigFile(c.cfgFile)
		if err != nil {
			return nil, err
		}

		c.UI.Info("Configuration mode: File")
		return proxyImpl.NewStaticConfigWatcher(cfg), nil
	}

	// Use the configured proxy ID
	if c.proxyID != "" {
		c.UI.Info("Configuration mode: Agent API")
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

		c.UI.Info(fmt.Sprintf(
			"          Upstream: %s => %s:%d",
			k, config.LocalBindAddress, config.LocalBindPort))
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

	m := NewRegisterMonitor()
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
  The token may be passed via the CLI or the CONSUL_TOKEN environment
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
