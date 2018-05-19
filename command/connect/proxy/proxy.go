package proxy

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof" // Expose pprof if configured
	"os"

	proxyAgent "github.com/hashicorp/consul/agent/proxy"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	proxyImpl "github.com/hashicorp/consul/connect/proxy"

	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui, shutdownCh <-chan struct{}) *cmd {
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
	logLevel  string
	cfgFile   string
	proxyID   string
	pprofAddr string
	service   string
	upstreams map[string]proxyImpl.UpstreamConfig
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
			"on port 8181.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Load the proxy ID and token from env vars if they're set
	if c.proxyID == "" {
		c.proxyID = os.Getenv(proxyAgent.EnvProxyId)
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

	c.UI.Output("Consul Connect proxy starting")

	c.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Run the proxy
	err = p.Serve()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Failed running proxy: %s", err))
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
		return proxyImpl.NewStaticConfigWatcher(cfg), nil
	}

	// Use the configured proxy ID
	if c.proxyID != "" {
		return proxyImpl.NewAgentConfigWatcher(client, c.proxyID, c.logger)
	}

	// Otherwise, we're representing a manually specified service.
	if c.service == "" {
		return nil, fmt.Errorf(
			"-service or -proxy-id must be specified so that proxy can " +
				"configure itself.")
	}

	// Convert our upstreams to a slice of configurations
	upstreams := make([]proxyImpl.UpstreamConfig, 0, len(c.upstreams))
	for _, u := range c.upstreams {
		upstreams = append(upstreams, u)
	}

	return proxyImpl.NewStaticConfigWatcher(&proxyImpl.Config{
		ProxiedServiceName: c.service,
		Upstreams:          upstreams,
	}), nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Runs a Consul Connect proxy"
const help = `
Usage: consul proxy [options]

  Starts a Consul Connect proxy and runs until an interrupt is received.
`
