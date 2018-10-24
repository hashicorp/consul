package envoy

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	proxyAgent "github.com/hashicorp/consul/agent/proxyprocess"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	proxyCmd "github.com/hashicorp/consul/command/connect/proxy"
	"github.com/hashicorp/consul/command/flags"

	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	client *api.Client

	// flags
	proxyID    string
	sidecarFor string
	adminBind  string
	envoyBin   string
	bootstrap  bool
	grpcAddr   string
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

	c.flags.StringVar(&c.envoyBin, "envoy-binary", "",
		"The full path to the envoy binary to run. By default will just search "+
			"$PATH. Ignored if -bootstrap is used.")

	c.flags.StringVar(&c.adminBind, "admin-bind", "localhost:19000",
		"The address:port to start envoy's admin server on. Envoy requires this "+
			"but care must be taked to ensure it's not exposed to untrusted network "+
			"as it has full control over the secrets and config of the proxy.")

	c.flags.BoolVar(&c.bootstrap, "bootstrap", false,
		"Generate the bootstrap.json but don't exec envoy")

	c.flags.StringVar(&c.grpcAddr, "grpc-addr", "",
		"Set the agent's gRPC address and port (in http(s)://host:port format). "+
			"Alternatively, you can specify CONSUL_GRPC_ADDR in ENV.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	passThroughArgs := c.flags.Args()

	// Load the proxy ID and token from env vars if they're set
	if c.proxyID == "" {
		c.proxyID = os.Getenv(proxyAgent.EnvProxyID)
	}
	if c.sidecarFor == "" {
		c.sidecarFor = os.Getenv(proxyAgent.EnvSidecarFor)
	}
	if c.grpcAddr == "" {
		c.grpcAddr = os.Getenv(api.GRPCAddrEnvName)
	}
	if c.grpcAddr == "" {
		// This is the dev mode default and recommended production setting if
		// enabled.
		c.grpcAddr = "localhost:8502"
	}
	if c.http.Token() == "" {
		// Extra check needed since CONSUL_HTTP_TOKEN has not been consulted yet but
		// calling SetToken with empty will force that to override the
		if proxyToken := os.Getenv(proxyAgent.EnvProxyToken); proxyToken != "" {
			c.http.SetToken(proxyToken)
		}
	}

	// Setup Consul client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	c.client = client

	// See if we need to lookup proxyID
	if c.proxyID == "" && c.sidecarFor != "" {
		proxyID, err := c.lookupProxyIDForSidecar()
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.proxyID = proxyID
	}
	if c.proxyID == "" {
		c.UI.Error("No proxy ID specified. One of -proxy-id or -sidecar-for is " +
			"required")
		return 1
	}

	// Generate config
	bootstrapJson, err := c.generateConfig()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.bootstrap {
		// Just output it and we are done
		os.Stdout.Write(bootstrapJson)
		return 0
	}

	// Find Envoy binary
	binary, err := c.findBinary()
	if err != nil {
		c.UI.Error("Couldn't find envoy binary: " + err.Error())
		return 1
	}

	err = execEnvoy(binary, nil, passThroughArgs, bootstrapJson)
	if err == errUnsupportedOS {
		c.UI.Error("Directly running Envoy is only supported on linux and macOS " +
			"since envoy itself doesn't build on other platforms currently.")
		c.UI.Error("Use the -bootstrap option to generate the JSON to use when running envoy " +
			"on a supported OS or via a container or VM.")
		return 1
	} else if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	return 0
}

var errUnsupportedOS = errors.New("envoy: not implemented on this operating system")

func (c *cmd) findBinary() (string, error) {
	if c.envoyBin != "" {
		return c.envoyBin, nil
	}
	return exec.LookPath("envoy")
}

func (c *cmd) templateArgs() (*templateArgs, error) {
	httpCfg := api.DefaultConfig()
	c.http.MergeOntoConfig(httpCfg)

	// Decide on TLS if the scheme is provided and indicates it, if the HTTP env
	// suggests TLS is supported explicitly (CONSUL_HTTP_SSL) or implicitly
	// (CONSUL_HTTP_ADDR) is https://
	useTLS := false
	if strings.HasPrefix(strings.ToLower(c.grpcAddr), "https://") {
		useTLS = true
	} else if useSSLEnv := os.Getenv(api.HTTPSSLEnvName); useSSLEnv != "" {
		if enabled, err := strconv.ParseBool(useSSLEnv); err != nil {
			useTLS = enabled
		}
	} else if strings.HasPrefix(strings.ToLower(httpCfg.Address), "https://") {
		useTLS = true
	}

	// We want to allow grpcAddr set as host:port with no scheme but if the host
	// is an IP this will fail to parse as a URL with "parse 127.0.0.1:8500: first
	// path segment in URL cannot contain colon". On the other hand we also
	// support both http(s)://host:port and unix:///path/to/file.
	addrPort := strings.TrimPrefix(c.grpcAddr, "http://")
	addrPort = strings.TrimPrefix(c.grpcAddr, "https://")

	agentAddr, agentPort, err := net.SplitHostPort(addrPort)
	if err != nil {
		return nil, fmt.Errorf("Invalid Consul HTTP address: %s", err)
	}
	if agentAddr == "" {
		agentAddr = "127.0.0.1"
	}

	// We use STATIC for agent which means we need to resolve DNS names like
	// `localhost` ourselves. We could use STRICT_DNS or LOGICAL_DNS with envoy
	// but Envoy resolves `localhost` differently to go on macOS at least which
	// causes paper cuts like default dev agent (which binds specifically to
	// 127.0.0.1) isn't reachable since Envoy resolves localhost to `[::]` and
	// can't connect.
	agentIP, err := net.ResolveIPAddr("ip", agentAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve agent address: %s", err)
	}

	adminAddr, adminPort, err := net.SplitHostPort(c.adminBind)
	if err != nil {
		return nil, fmt.Errorf("Invalid Consul HTTP address: %s", err)
	}

	// Envoy requires IP addresses to bind too when using static so resolve DNS or
	// localhost here.
	adminBindIP, err := net.ResolveIPAddr("ip", adminAddr)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve admin bind address: %s", err)
	}

	return &templateArgs{
		ProxyCluster:          c.proxyID,
		ProxyID:               c.proxyID,
		AgentAddress:          agentIP.String(),
		AgentPort:             agentPort,
		AgentTLS:              useTLS,
		AgentCAFile:           httpCfg.TLSConfig.CAFile,
		AdminBindAddress:      adminBindIP.String(),
		AdminBindPort:         adminPort,
		Token:                 httpCfg.Token,
		LocalAgentClusterName: xds.LocalAgentClusterName,
	}, nil
}

func (c *cmd) generateConfig() ([]byte, error) {
	args, err := c.templateArgs()
	if err != nil {
		return nil, err
	}
	var t = template.Must(template.New("bootstrap").Parse(bootstrapTemplate))
	var buf bytes.Buffer
	err = t.Execute(&buf, args)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *cmd) lookupProxyIDForSidecar() (string, error) {
	return proxyCmd.LookupProxyIDForSidecar(c.client, c.sidecarFor)
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Runs or Configures Envoy as a Connect proxy"
const help = `
Usage: consul connect envoy [options]

  Generates the bootstrap configuration needed to start an Envoy proxy instance
  for use as a Connect sidecar for a particular service instance. By default it
  will generate the config and then exec Envoy directly until it exits normally.

  It will search $PATH for the envoy binary but this can be overridden with
  -envoy-binary.

  It can instead only generate the bootstrap.json based on the current ENV and
  arguments using -bootstrap.

  The proxy requires service:write permissions for the service it represents.
  The token may be passed via the CLI or the CONSUL_TOKEN environment
  variable.

  The example below shows how to start a local proxy as a sidecar to a "web"
  service instance. It assumes that the proxy was already registered with it's
  Config for example via a sidecar_service block.

    $ consul connect envoy -sidecar-for web

`
