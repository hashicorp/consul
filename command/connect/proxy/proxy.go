package proxy

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	// Expose pprof if configured
	_ "net/http/pprof"

	"github.com/hashicorp/consul/command/flags"
	proxyImpl "github.com/hashicorp/consul/proxy"

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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.cfgFile, "insecure-dev-config", "",
		"If set, proxy config is read on startup from this file (in HCL or JSON"+
			"format). If a config file is given, the proxy will use that instead of "+
			"querying the local agent for it's configuration. It will not reload it "+
			"except on startup. In this mode the proxy WILL NOT authorize incoming "+
			"connections with the local agent which is totally insecure. This is "+
			"ONLY for development and testing.")

	c.flags.StringVar(&c.proxyID, "proxy-id", "",
		"The proxy's ID on the local agent.")

	c.flags.StringVar(&c.logLevel, "log-level", "INFO",
		"Specifies the log level.")

	c.flags.StringVar(&c.pprofAddr, "pprof-addr", "",
		"Enable debugging via pprof. Providing a host:port (or just ':port') "+
			"enables profiling HTTP endpoints on that address.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
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

	var p *proxyImpl.Proxy
	if c.cfgFile != "" {
		c.UI.Info("Configuring proxy locally from " + c.cfgFile)

		p, err = proxyImpl.NewFromConfigFile(client, c.cfgFile, c.logger)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed configuring from file: %s", err))
			return 1
		}

	} else {
		p, err = proxyImpl.New(client, c.proxyID, c.logger)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed configuring from agent: %s", err))
			return 1
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := p.Run(ctx)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed running proxy: %s", err))
		}
		// If we exited early due to a fatal error, need to unblock the main
		// routine. But we can't close shutdownCh since it might already be closed
		// by a signal and there is no way to tell. We also can't send on it to
		// unblock main routine since it's typed as receive only. So the best thing
		// we can do is cancel the context and have the main routine select on both.
		cancel()
	}()

	c.UI.Output("Consul Connect proxy running!")

	c.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Wait for shutdown or context cancel (see Run() goroutine above)
	select {
	case <-c.shutdownCh:
		cancel()
	case <-ctx.Done():
	}
	c.UI.Output("Consul Connect proxy shutdown")
	return 0
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
