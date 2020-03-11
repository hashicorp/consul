package envoy

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	proxyCmd "github.com/hashicorp/consul/command/connect/proxy"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/go-sockaddr/template"

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

const DefaultAdminAccessLogPath = "/dev/null"

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	http   *flags.HTTPFlags
	help   string
	client *api.Client

	// flags
	meshGateway          bool
	proxyID              string
	sidecarFor           string
	adminAccessLogPath   string
	adminBind            string
	envoyBin             string
	bootstrap            bool
	disableCentralConfig bool
	grpcAddr             string
	envoyVersion         string

	// mesh gateway registration information
	register           bool
	address            string
	wanAddress         string
	deregAfterCritical string
	bindAddresses      map[string]string
	exposeServers      bool

	meshGatewaySvcName string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.proxyID, "proxy-id", "",
		"The proxy's ID on the local agent.")

	c.flags.BoolVar(&c.meshGateway, "mesh-gateway", false,
		"Configure Envoy as a Mesh Gateway.")

	c.flags.StringVar(&c.sidecarFor, "sidecar-for", "",
		"The ID of a service instance on the local agent that this proxy should "+
			"become a sidecar for. It requires that the proxy service is registered "+
			"with the agent as a connect-proxy with Proxy.DestinationServiceID set "+
			"to this value. If more than one such proxy is registered it will fail.")

	c.flags.StringVar(&c.envoyBin, "envoy-binary", "",
		"The full path to the envoy binary to run. By default will just search "+
			"$PATH. Ignored if -bootstrap is used.")

	c.flags.StringVar(&c.adminAccessLogPath, "admin-access-log-path", DefaultAdminAccessLogPath,
		fmt.Sprintf("The path to write the access log for the administration server. If no access "+
			"log is desired specify %q. By default it will use %q.",
			DefaultAdminAccessLogPath, DefaultAdminAccessLogPath))

	c.flags.StringVar(&c.adminBind, "admin-bind", "localhost:19000",
		"The address:port to start envoy's admin server on. Envoy requires this "+
			"but care must be taken to ensure it's not exposed to an untrusted network "+
			"as it has full control over the secrets and config of the proxy.")

	c.flags.BoolVar(&c.bootstrap, "bootstrap", false,
		"Generate the bootstrap.json but don't exec envoy")

	c.flags.BoolVar(&c.disableCentralConfig, "no-central-config", false,
		"By default the proxy's bootstrap configuration can be customized "+
			"centrally. This requires that the command run on the same agent as the "+
			"proxy will and that the agent is reachable when the command is run. In "+
			"cases where either assumption is violated this flag will prevent the "+
			"command attempting to resolve config from the local agent.")

	c.flags.StringVar(&c.grpcAddr, "grpc-addr", "",
		"Set the agent's gRPC address and port (in http(s)://host:port format). "+
			"Alternatively, you can specify CONSUL_GRPC_ADDR in ENV.")

	c.flags.StringVar(&c.envoyVersion, "envoy-version", "1.13.0",
		"Sets the envoy-version that the envoy binary has.")

	c.flags.BoolVar(&c.register, "register", false,
		"Register a new Mesh Gateway service before configuring and starting Envoy")

	c.flags.StringVar(&c.address, "address", "",
		"LAN address to advertise in the Mesh Gateway service registration")

	c.flags.StringVar(&c.wanAddress, "wan-address", "",
		"WAN address to advertise in the Mesh Gateway service registration")

	c.flags.Var((*flags.FlagMapValue)(&c.bindAddresses), "bind-address", "Bind "+
		"address to use instead of the default binding rules given as `<name>=<ip>:<port>` "+
		"pairs. This flag may be specified multiple times to add multiple bind addresses.")

	c.flags.StringVar(&c.meshGatewaySvcName, "service", "mesh-gateway",
		"Service name to use for the registration")

	c.flags.BoolVar(&c.exposeServers, "expose-servers", false,
		"Expose the servers for WAN federation via this mesh gateway")

	c.flags.StringVar(&c.deregAfterCritical, "deregister-after-critical", "6h",
		"The amount of time the gateway services health check can be failing before being deregistered")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.NamespaceFlags())
	c.help = flags.Usage(help, c.flags)
}

const (
	DefaultMeshGatewayPort int = 443
)

func parseAddress(addrStr string) (string, int, error) {
	if addrStr == "" {
		// defaulting the port to 443
		return "", DefaultMeshGatewayPort, nil
	}

	x, err := template.Parse(addrStr)
	if err != nil {
		return "", DefaultMeshGatewayPort, fmt.Errorf("Error parsing address %q: %v", addrStr, err)
	}

	addr, portStr, err := net.SplitHostPort(x)
	if err != nil {
		return "", DefaultMeshGatewayPort, fmt.Errorf("Error parsing address %q: %v", x, err)
	}

	port := DefaultMeshGatewayPort

	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", DefaultMeshGatewayPort, fmt.Errorf("Error parsing port %q: %v", portStr, err)
		}
	}

	return addr, port, nil
}

// canBindInternal is here mainly so we can unit test this with a constant net.Addr list
func canBindInternal(addr string, ifAddrs []net.Addr) bool {
	if addr == "" {
		return false
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}

	ipStr := ip.String()

	for _, addr := range ifAddrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if v.IP.String() == ipStr {
				return true
			}
		default:
			if addr.String() == ipStr {
				return true
			}
		}
	}

	return false
}

func canBind(addr string) bool {
	ifAddrs, err := net.InterfaceAddrs()

	if err != nil {
		return false
	}

	return canBindInternal(addr, ifAddrs)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}
	passThroughArgs := c.flags.Args()

	// Load the proxy ID and token from env vars if they're set
	if c.proxyID == "" {
		c.proxyID = os.Getenv("CONNECT_PROXY_ID")
	}
	if c.sidecarFor == "" {
		c.sidecarFor = os.Getenv("CONNECT_SIDECAR_FOR")
	}
	if c.grpcAddr == "" {
		c.grpcAddr = os.Getenv(api.GRPCAddrEnvName)
	}

	// Setup Consul client
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	c.client = client

	if c.exposeServers {
		if !c.meshGateway {
			c.UI.Error("'-expose-servers' can only be used for mesh gateways")
			return 1
		}
		if !c.register {
			c.UI.Error("'-expose-servers' requires '-register'")
			return 1
		}
	}

	if c.register {
		if !c.meshGateway {
			c.UI.Error("Auto-Registration can only be used for mesh gateways")
			return 1
		}

		lanAddr, lanPort, err := parseAddress(c.address)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to parse the -address parameter: %v", err))
			return 1
		}

		taggedAddrs := make(map[string]api.ServiceAddress)

		if lanAddr != "" {
			taggedAddrs[structs.TaggedAddressLAN] = api.ServiceAddress{Address: lanAddr, Port: lanPort}
		}

		wanAddr := ""
		wanPort := lanPort
		if c.wanAddress != "" {
			wanAddr, wanPort, err = parseAddress(c.wanAddress)
			if err != nil {
				c.UI.Error(fmt.Sprintf("Failed to parse the -wan-address parameter: %v", err))
				return 1
			}
			taggedAddrs[structs.TaggedAddressWAN] = api.ServiceAddress{Address: wanAddr, Port: wanPort}
		}

		tcpCheckAddr := lanAddr
		if tcpCheckAddr == "" {
			// fallback to localhost as the gateway has to reside in the same network namespace
			// as the agent
			tcpCheckAddr = "127.0.0.1"
		}

		var proxyConf *api.AgentServiceConnectProxyConfig

		if len(c.bindAddresses) > 0 {
			// override all default binding rules and just bind to the user-supplied addresses
			bindAddresses := make(map[string]api.ServiceAddress)

			for addrName, addrStr := range c.bindAddresses {
				addr, port, err := parseAddress(addrStr)
				if err != nil {
					c.UI.Error(fmt.Sprintf("Failed to parse the bind address: %s=%s: %v", addrName, addrStr, err))
					return 1
				}

				bindAddresses[addrName] = api.ServiceAddress{Address: addr, Port: port}
			}

			proxyConf = &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"envoy_mesh_gateway_no_default_bind": true,
					"envoy_mesh_gateway_bind_addresses":  bindAddresses,
				},
			}
		} else if canBind(lanAddr) && canBind(wanAddr) {
			// when both addresses are bindable then we bind to the tagged addresses
			// for creating the envoy listeners
			proxyConf = &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"envoy_mesh_gateway_no_default_bind":       true,
					"envoy_mesh_gateway_bind_tagged_addresses": true,
				},
			}
		} else if !canBind(lanAddr) && lanAddr != "" {
			c.UI.Error(fmt.Sprintf("The LAN address %q will not be bindable. Either set a bindable address or override the bind addresses with -bind-address", lanAddr))
			return 1
		}

		var meta map[string]string
		if c.exposeServers {
			meta = map[string]string{structs.MetaWANFederationKey: "1"}
		}

		svc := api.AgentServiceRegistration{
			Kind:            api.ServiceKindMeshGateway,
			Name:            c.meshGatewaySvcName,
			Address:         lanAddr,
			Port:            lanPort,
			Meta:            meta,
			TaggedAddresses: taggedAddrs,
			Proxy:           proxyConf,
			Check: &api.AgentServiceCheck{
				Name:                           "Mesh Gateway Listening",
				TCP:                            ipaddr.FormatAddressPort(tcpCheckAddr, lanPort),
				Interval:                       "10s",
				DeregisterCriticalServiceAfter: c.deregAfterCritical,
			},
		}

		if err := client.Agent().ServiceRegister(&svc); err != nil {
			c.UI.Error(fmt.Sprintf("Error registering service %q: %s", svc.Name, err))
			return 1
		}

		c.UI.Output(fmt.Sprintf("Registered service: %s", svc.Name))
	}

	// See if we need to lookup proxyID
	if c.proxyID == "" && c.sidecarFor != "" {
		proxyID, err := c.lookupProxyIDForSidecar()
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.proxyID = proxyID
	} else if c.proxyID == "" && c.meshGateway {
		gatewaySvc, err := c.lookupGatewayProxy()
		if err != nil {
			c.UI.Error(err.Error())
			return 1
		}
		c.proxyID = gatewaySvc.ID
		c.meshGatewaySvcName = gatewaySvc.Service
	}

	if c.proxyID == "" {
		c.UI.Error("No proxy ID specified. One of -proxy-id or -sidecar-for/-mesh-gateway is " +
			"required")
		return 1
	}

	// See if we need to lookup grpcAddr
	if c.grpcAddr == "" {
		port, err := c.lookupGRPCPort()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		}
		if port <= 0 {
			// This is the dev mode default and recommended production setting if
			// enabled.
			port = 8502
			c.UI.Info(fmt.Sprintf("Defaulting to grpc port = %d", port))
		}
		c.grpcAddr = fmt.Sprintf("localhost:%v", port)
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

func (c *cmd) templateArgs() (*BootstrapTplArgs, error) {
	httpCfg := api.DefaultConfig()
	c.http.MergeOntoConfig(httpCfg)

	// Trigger the Client init to do any last-minute updates to the Config.
	if _, err := api.NewClient(httpCfg); err != nil {
		return nil, err
	}

	// Decide on TLS if the scheme is provided and indicates it, if the HTTP env
	// suggests TLS is supported explicitly (CONSUL_HTTP_SSL) or implicitly
	// (CONSUL_HTTP_ADDR) is https://
	useTLS := false
	if strings.HasPrefix(strings.ToLower(c.grpcAddr), "https://") {
		useTLS = true
	} else if useSSLEnv := os.Getenv(api.HTTPSSLEnvName); useSSLEnv != "" {
		if enabled, err := strconv.ParseBool(useSSLEnv); err == nil {
			useTLS = enabled
		}
	} else if strings.HasPrefix(strings.ToLower(httpCfg.Address), "https://") {
		useTLS = true
	}

	// We want to allow grpcAddr set as host:port with no scheme but if the host
	// is an IP this will fail to parse as a URL with "parse 127.0.0.1:8500: first
	// path segment in URL cannot contain colon". On the other hand we also
	// support both http(s)://host:port and unix:///path/to/file.
	var agentAddr, agentPort, agentSock string
	if grpcAddr := strings.TrimPrefix(c.grpcAddr, "unix://"); grpcAddr != c.grpcAddr {
		// Path to unix socket
		agentSock = grpcAddr
	} else {
		// Parse as host:port with option http prefix
		grpcAddr = strings.TrimPrefix(c.grpcAddr, "http://")
		grpcAddr = strings.TrimPrefix(c.grpcAddr, "https://")

		var err error
		agentAddr, agentPort, err = net.SplitHostPort(grpcAddr)
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
		agentAddr = agentIP.String()
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

	// Ideally the cluster should be the service name. We may or may not have that
	// yet depending on the arguments used so make a best effort here. In the
	// common case, even if the command was invoked with proxy-id and we don't
	// know service name yet, we will after we resolve the proxy's config in a bit
	// and will update this then.
	cluster := c.proxyID
	if c.sidecarFor != "" {
		cluster = c.sidecarFor
	} else if c.meshGateway && c.meshGatewaySvcName != "" {
		cluster = c.meshGatewaySvcName
	}

	adminAccessLogPath := c.adminAccessLogPath
	if adminAccessLogPath == "" {
		adminAccessLogPath = DefaultAdminAccessLogPath
	}

	var caPEM string
	if httpCfg.TLSConfig.CAFile != "" {
		content, err := ioutil.ReadFile(httpCfg.TLSConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("Failed to read CA file: %s", err)
		}
		caPEM = strings.Replace(string(content), "\n", "\\n", -1)
	}

	return &BootstrapTplArgs{
		ProxyCluster:          cluster,
		ProxyID:               c.proxyID,
		AgentAddress:          agentAddr,
		AgentPort:             agentPort,
		AgentSocket:           agentSock,
		AgentTLS:              useTLS,
		AgentCAPEM:            caPEM,
		AdminAccessLogPath:    adminAccessLogPath,
		AdminBindAddress:      adminBindIP.String(),
		AdminBindPort:         adminPort,
		Token:                 httpCfg.Token,
		LocalAgentClusterName: xds.LocalAgentClusterName,
		Namespace:             httpCfg.Namespace,
		EnvoyVersion:          c.envoyVersion,
	}, nil
}

func (c *cmd) generateConfig() ([]byte, error) {
	args, err := c.templateArgs()
	if err != nil {
		return nil, err
	}

	var bsCfg BootstrapConfig

	if !c.disableCentralConfig {
		// Fetch any customization from the registration
		svc, _, err := c.client.Agent().Service(c.proxyID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed fetch proxy config from local agent: %s", err)
		}

		if svc.Proxy == nil {
			return nil, errors.New("service is not a Connect proxy or mesh gateway")
		}

		// Parse the bootstrap config
		if err := mapstructure.WeakDecode(svc.Proxy.Config, &bsCfg); err != nil {
			return nil, fmt.Errorf("failed parsing Proxy.Config: %s", err)
		}

		if svc.Proxy.DestinationServiceName != "" {
			// Override cluster now we know the actual service name
			args.ProxyCluster = svc.Proxy.DestinationServiceName
		}
	}

	return bsCfg.GenerateJSON(args)
}

func (c *cmd) lookupProxyIDForSidecar() (string, error) {
	return proxyCmd.LookupProxyIDForSidecar(c.client, c.sidecarFor)
}

func (c *cmd) lookupGatewayProxy() (*api.AgentService, error) {
	return proxyCmd.LookupGatewayProxy(c.client)
}

func (c *cmd) lookupGRPCPort() (int, error) {
	self, err := c.client.Agent().Self()
	if err != nil {
		return 0, err
	}
	cfg, ok := self["DebugConfig"]
	if !ok {
		return 0, fmt.Errorf("unexpected agent response: no debug config")
	}
	port, ok := cfg["GRPCPort"]
	if !ok {
		return 0, fmt.Errorf("agent does not have grpc port enabled")
	}
	portN, ok := port.(float64)
	if !ok {
		return 0, fmt.Errorf("invalid grpc port in agent response")
	}

	return int(portN), nil
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
  The token may be passed via the CLI or the CONSUL_HTTP_TOKEN environment
  variable.

  The example below shows how to start a local proxy as a sidecar to a "web"
  service instance. It assumes that the proxy was already registered with it's
  Config for example via a sidecar_service block.

    $ consul connect envoy -sidecar-for web

`
