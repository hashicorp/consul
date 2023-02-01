package envoy

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/api"
	proxyCmd "github.com/hashicorp/consul/command/connect/proxy"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/tlsutil"
)

func New(ui cli.Ui) *cmd {
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
	logger hclog.Logger

	// flags
	meshGateway           bool
	gateway               string
	proxyID               string
	nodeName              string
	sidecarFor            string
	adminAccessLogPath    string
	adminBind             string
	envoyBin              string
	bootstrap             bool
	disableCentralConfig  bool
	grpcAddr              string
	envoyVersion          string
	prometheusBackendPort string
	prometheusScrapePath  string
	prometheusCAFile      string
	prometheusCAPath      string
	prometheusCertFile    string
	prometheusKeyFile     string
	enableLogging         bool

	// mesh gateway registration information
	register           bool
	lanAddress         ServiceAddressValue
	wanAddress         ServiceAddressValue
	deregAfterCritical string
	bindAddresses      ServiceAddressMapValue
	exposeServers      bool
	omitDeprecatedTags bool

	envoyReadyBindAddress string
	envoyReadyBindPort    int

	gatewaySvcName string
	gatewayKind    api.ServiceKind
}

const meshGatewayVal = "mesh"

var defaultEnvoyVersion = proxysupport.EnvoyVersions[0]

var supportedGateways = map[string]api.ServiceKind{
	"mesh":        api.ServiceKindMeshGateway,
	"terminating": api.ServiceKindTerminatingGateway,
	"ingress":     api.ServiceKindIngressGateway,
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	c.flags.StringVar(&c.proxyID, "proxy-id", os.Getenv("CONNECT_PROXY_ID"),
		"The proxy's ID on the local agent.")

	c.flags.StringVar(&c.nodeName, "node-name", "",
		"[Experimental] The node name where the proxy service is registered. It requires proxy-id to be specified. ")

	// Deprecated in favor of `gateway`
	c.flags.BoolVar(&c.meshGateway, "mesh-gateway", false,
		"Configure Envoy as a Mesh Gateway.")

	c.flags.StringVar(&c.gateway, "gateway", "",
		"The type of gateway to register. One of: terminating, ingress, or mesh")

	c.flags.StringVar(&c.sidecarFor, "sidecar-for", os.Getenv("CONNECT_SIDECAR_FOR"),
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

	c.flags.StringVar(&c.grpcAddr, "grpc-addr", os.Getenv(api.GRPCAddrEnvName),
		"Set the agent's gRPC address and port (in http(s)://host:port format). "+
			"Alternatively, you can specify CONSUL_GRPC_ADDR in ENV.")

	// Deprecated, no longer needed, keeping it around to not break back compat
	c.flags.StringVar(&c.envoyVersion, "envoy-version", defaultEnvoyVersion,
		"This is a legacy flag that is currently not used but was formerly used to set the "+
			"version for the envoy binary that gets invoked by Consul. This is no longer "+
			"necessary as Consul will invoke the binary at a path set by -envoy-binary "+
			"or whichever envoy binary it finds in $PATH")

	c.flags.BoolVar(&c.register, "register", false,
		"Register a new gateway service before configuring and starting Envoy")

	c.flags.Var(&c.lanAddress, "address",
		"LAN address to advertise in the gateway service registration")

	c.flags.StringVar(&c.envoyReadyBindAddress, "envoy-ready-bind-address", "",
		"The address on which Envoy's readiness probe is available.")
	c.flags.IntVar(&c.envoyReadyBindPort, "envoy-ready-bind-port", 0,
		"The port on which Envoy's readiness probe is available.")

	c.flags.Var(&c.wanAddress, "wan-address",
		"WAN address to advertise in the gateway service registration. For ingress gateways, "+
			"only an IP address (without a port) is required.")

	c.flags.Var(&c.bindAddresses, "bind-address", "Bind "+
		"address to use instead of the default binding rules given as `<name>=<ip>:<port>` "+
		"pairs. This flag may be specified multiple times to add multiple bind addresses.")

	c.flags.StringVar(&c.gatewaySvcName, "service", "",
		"Service name to use for the registration")

	c.flags.BoolVar(&c.exposeServers, "expose-servers", false,
		"Expose the servers for WAN federation via this mesh gateway")

	c.flags.StringVar(&c.deregAfterCritical, "deregister-after-critical", "6h",
		"The amount of time the gateway services health check can be failing before being deregistered")

	c.flags.BoolVar(&c.omitDeprecatedTags, "omit-deprecated-tags", false,
		"In Consul 1.9.0 the format of metric tags for Envoy clusters was updated from consul.[service|dc|...] to "+
			"consul.destination.[service|dc|...]. The old tags were preserved for backward compatibility,"+
			"but can be disabled with this flag.")

	c.flags.StringVar(&c.prometheusBackendPort, "prometheus-backend-port", "",
		"Sets the backend port for the 'prometheus_backend' cluster that envoy_prometheus_bind_addr will point to. "+
			"Without this flag, envoy_prometheus_bind_addr would point to the 'self_admin' cluster where Envoy metrics are exposed. "+
			"The metrics merging feature in consul-k8s uses this to point to the merged metrics endpoint combining Envoy and service metrics. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")

	c.flags.StringVar(&c.prometheusScrapePath, "prometheus-scrape-path", "/metrics",
		"Sets the path where Envoy will expose metrics on envoy_prometheus_bind_addr listener. "+
			"For example, if envoy_prometheus_bind_addr is 0.0.0.0:20200, and this flag is "+
			"set to /scrape-metrics, prometheus metrics would be scrapeable at "+
			"0.0.0.0:20200/scrape-metrics. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")

	c.flags.StringVar(&c.prometheusCAFile, "prometheus-ca-file", "",
		"Path to a CA file for Envoy to use when serving TLS on the Prometheus metrics endpoint. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")
	c.flags.StringVar(&c.prometheusCAPath, "prometheus-ca-path", "",
		"Path to a directory of CA certificates for Envoy to use when serving the Prometheus metrics endpoint. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")
	c.flags.StringVar(&c.prometheusCertFile, "prometheus-cert-file", "",
		"Path to a certificate file for Envoy to use when serving TLS on the Prometheus metrics endpoint. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")
	c.flags.StringVar(&c.prometheusKeyFile, "prometheus-key-file", "",
		"Path to a private key file for Envoy to use when serving TLS on the Prometheus metrics endpoint. "+
			"Only applicable when envoy_prometheus_bind_addr is set in proxy config.")

	c.flags.BoolVar(&c.enableLogging, "enable-config-gen-logging", false,
		"Output debug log messages during config generation")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	c.help = flags.Usage(help, c.flags)
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

func canBind(addr api.ServiceAddress) bool {
	ifAddrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	return canBindInternal(addr.Address, ifAddrs)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		return 1
	}

	// Setup Consul client
	var err error
	c.client, err = c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}

	// TODO: refactor
	return c.run(c.flags.Args())
}

func (c *cmd) run(args []string) int {
	opts := hclog.LoggerOptions{Level: hclog.Error}
	if c.enableLogging {
		opts.Level = hclog.Debug
	}
	c.logger = hclog.New(&opts)
	c.logger.Debug("Starting Envoy config generation")

	if c.nodeName != "" && c.proxyID == "" {
		c.UI.Error("'-node-name' requires '-proxy-id'")
		return 1
	}

	// Fixup for deprecated mesh-gateway flag
	if c.meshGateway && c.gateway != "" {
		c.UI.Error("The mesh-gateway flag is deprecated and cannot be used alongside the gateway flag")
		return 1
	}

	if c.meshGateway {
		c.gateway = meshGatewayVal
	}

	if c.exposeServers {
		if c.gateway != meshGatewayVal {
			c.UI.Error("'-expose-servers' can only be used for mesh gateways")
			return 1
		}
		if !c.register {
			c.UI.Error("'-expose-servers' requires '-register'")
			return 1
		}
	}

	// Gateway kind is set so that it is available even if not auto-registering the gateway
	if c.gateway != "" {
		kind, ok := supportedGateways[c.gateway]
		if !ok {
			c.UI.Error("Gateway must be one of: terminating, mesh, or ingress")
			return 1
		}
		c.gatewayKind = kind

		if c.gatewaySvcName == "" {
			c.gatewaySvcName = string(c.gatewayKind)
		}
	}

	if c.proxyID == "" {
		switch {
		case c.sidecarFor != "":
			proxyID, err := proxyCmd.LookupProxyIDForSidecar(c.client, c.sidecarFor)
			if err != nil {
				c.UI.Error(err.Error())
				return 1
			}
			c.proxyID = proxyID

		case c.gateway != "" && !c.register:
			gatewaySvc, err := proxyCmd.LookupGatewayProxy(c.client, c.gatewayKind)
			if err != nil {
				c.UI.Error(err.Error())
				return 1
			}
			c.proxyID = gatewaySvc.ID
			c.gatewaySvcName = gatewaySvc.Service

		case c.gateway != "" && c.register:
			c.proxyID = c.gatewaySvcName

		}
		c.logger.Debug("Set Proxy ID", "proxy-id", c.proxyID)
	}
	if c.proxyID == "" {
		c.UI.Error("No proxy ID specified. One of -proxy-id, -sidecar-for, or -gateway is " +
			"required")
		return 1
	}

	// If any of CA/Cert/Key are specified, make sure they are all present.
	if c.prometheusKeyFile != "" || c.prometheusCertFile != "" || (c.prometheusCAFile != "" || c.prometheusCAPath != "") {
		if c.prometheusKeyFile == "" || c.prometheusCertFile == "" || (c.prometheusCAFile == "" && c.prometheusCAPath == "") {
			c.UI.Error("Must provide a CA (-prometheus-ca-file or -prometheus-ca-path) as well as " +
				"-prometheus-cert-file and -prometheus-key-file to enable TLS for prometheus metrics")
			return 1
		}
	}

	if c.register {
		if c.nodeName != "" {
			c.UI.Error("'-register' cannot be used with '-node-name'")
			return 1
		}
		if c.gateway == "" {
			c.UI.Error("Auto-Registration can only be used for gateways")
			return 1
		}

		taggedAddrs := make(map[string]api.ServiceAddress)
		lanAddr := c.lanAddress.Value()
		if lanAddr.Address != "" {
			taggedAddrs[structs.TaggedAddressLAN] = lanAddr
		}

		wanAddr := c.wanAddress.Value()
		if wanAddr.Address != "" {
			taggedAddrs[structs.TaggedAddressWAN] = wanAddr
		}

		tcpCheckAddr := lanAddr.Address
		if tcpCheckAddr == "" {
			// fallback to localhost as the gateway has to reside in the same network namespace
			// as the agent
			tcpCheckAddr = "127.0.0.1"
		}

		var proxyConf *api.AgentServiceConnectProxyConfig
		if len(c.bindAddresses.value) > 0 {
			// override all default binding rules and just bind to the user-supplied addresses
			proxyConf = &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"envoy_gateway_no_default_bind": true,
					"envoy_gateway_bind_addresses":  c.bindAddresses.value,
				},
			}
		} else if canBind(lanAddr) && canBind(wanAddr) {
			// when both addresses are bindable then we bind to the tagged addresses
			// for creating the envoy listeners
			proxyConf = &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"envoy_gateway_no_default_bind":       true,
					"envoy_gateway_bind_tagged_addresses": true,
				},
			}
		} else if !canBind(lanAddr) && lanAddr.Address != "" {
			c.UI.Error(fmt.Sprintf("The LAN address %q will not be bindable. Either set a bindable address or override the bind addresses with -bind-address", lanAddr.Address))
			return 1
		}

		var meta map[string]string
		if c.exposeServers {
			meta = map[string]string{structs.MetaWANFederationKey: "1"}
		}

		svc := api.AgentServiceRegistration{
			Kind:            c.gatewayKind,
			Name:            c.gatewaySvcName,
			ID:              c.proxyID,
			Address:         lanAddr.Address,
			Port:            lanAddr.Port,
			Meta:            meta,
			TaggedAddresses: taggedAddrs,
			Proxy:           proxyConf,
			Check: &api.AgentServiceCheck{
				Name:                           fmt.Sprintf("%s listening", c.gatewayKind),
				TCP:                            ipaddr.FormatAddressPort(tcpCheckAddr, lanAddr.Port),
				Interval:                       "10s",
				DeregisterCriticalServiceAfter: c.deregAfterCritical,
			},
		}

		if err := c.client.Agent().ServiceRegister(&svc); err != nil {
			c.UI.Error(fmt.Sprintf("Error registering service %q: %s", svc.Name, err))
			return 1
		}
		c.logger.Debug("Proxy registration complete")

		if !c.bootstrap {
			// We need stdout to be reserved exclusively for the JSON blob, so
			// we omit logging this to Info which also writes to stdout.
			c.UI.Info(fmt.Sprintf("Registered service: %s", svc.Name))
		}
	}

	// Generate config
	c.logger.Debug("Generating bootstrap config")
	bootstrapJson, err := c.generateConfig()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if c.bootstrap {
		// Just output it and we are done
		c.logger.Debug("Outputting bootstrap config")
		c.UI.Output(string(bootstrapJson))
		return 0
	}

	// Find Envoy binary
	c.logger.Debug("Finding envoy binary")
	binary, err := c.findBinary()
	if err != nil {
		c.UI.Error("Couldn't find envoy binary: " + err.Error())
		return 1
	}

	c.logger.Debug("Executing envoy binary")
	err = execEnvoy(binary, nil, args, bootstrapJson)
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

	// api.NewClient normalizes some values (Token, Scheme) on the Config.
	if _, err := api.NewClient(httpCfg); err != nil {
		return nil, err
	}

	xdsAddr, err := c.xdsAddress(httpCfg)
	if err != nil {
		return nil, err
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
	proxySourceService := ""
	if c.sidecarFor != "" {
		cluster = c.sidecarFor
		proxySourceService = c.sidecarFor
	} else if c.gateway != "" && c.gatewaySvcName != "" {
		cluster = c.gatewaySvcName
		proxySourceService = c.gatewaySvcName
	}

	adminAccessLogPath := c.adminAccessLogPath
	if adminAccessLogPath == "" {
		adminAccessLogPath = DefaultAdminAccessLogPath
	}

	var caPEM string
	pems, err := tlsutil.LoadCAs(httpCfg.TLSConfig.CAFile, httpCfg.TLSConfig.CAPath)
	if err != nil {
		return nil, err
	}
	caPEM = strings.Replace(strings.Join(pems, ""), "\n", "\\n", -1)

	return &BootstrapTplArgs{
		GRPC:                  xdsAddr,
		ProxyCluster:          cluster,
		ProxyID:               c.proxyID,
		NodeName:              c.nodeName,
		ProxySourceService:    proxySourceService,
		AgentCAPEM:            caPEM,
		AdminAccessLogPath:    adminAccessLogPath,
		AdminBindAddress:      adminBindIP.String(),
		AdminBindPort:         adminPort,
		Token:                 httpCfg.Token,
		LocalAgentClusterName: xds.LocalAgentClusterName,
		Namespace:             httpCfg.Namespace,
		Partition:             httpCfg.Partition,
		Datacenter:            httpCfg.Datacenter,
		PrometheusBackendPort: c.prometheusBackendPort,
		PrometheusScrapePath:  c.prometheusScrapePath,
		PrometheusCAFile:      c.prometheusCAFile,
		PrometheusCAPath:      c.prometheusCAPath,
		PrometheusCertFile:    c.prometheusCertFile,
		PrometheusKeyFile:     c.prometheusKeyFile,
	}, nil
}

func (c *cmd) generateConfig() ([]byte, error) {
	args, err := c.templateArgs()
	if err != nil {
		return nil, err
	}
	c.logger.Debug("Generated template args")

	var bsCfg BootstrapConfig

	// Fetch any customization from the registration
	var svcProxyConfig *api.AgentServiceConnectProxyConfig
	var serviceName, ns, partition, datacenter string
	if c.nodeName == "" {
		svc, _, err := c.client.Agent().Service(c.proxyID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed fetch proxy config from local agent: %s", err)
		}
		svcProxyConfig = svc.Proxy
		serviceName = svc.Service
		ns = svc.Namespace
		partition = svc.Partition
		datacenter = svc.Datacenter
	} else {
		filter := fmt.Sprintf("ID == %q", c.proxyID)
		svcList, _, err := c.client.Catalog().NodeServiceList(c.nodeName,
			&api.QueryOptions{Filter: filter, MergeCentralConfig: true})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch proxy config from catalog for node %q: %w", c.nodeName, err)
		}
		if len(svcList.Services) == 0 {
			return nil, fmt.Errorf("Proxy service with ID %q not found", c.proxyID)
		}
		if len(svcList.Services) > 1 {
			return nil, fmt.Errorf("Expected to find only one proxy service with ID %q, but more were found", c.proxyID)
		}

		svcProxyConfig = svcList.Services[0].Proxy
		serviceName = svcList.Services[0].Service
		ns = svcList.Services[0].Namespace
		partition = svcList.Services[0].Partition
		datacenter = svcList.Node.Datacenter
		c.gatewayKind = svcList.Services[0].Kind
	}
	c.logger.Debug("Fetched registration info")
	if svcProxyConfig == nil {
		return nil, errors.New("service is not a Connect proxy or gateway")
	}

	if svcProxyConfig.DestinationServiceName != "" {
		// Override cluster now we know the actual service name
		args.ProxyCluster = svcProxyConfig.DestinationServiceName
		args.ProxySourceService = svcProxyConfig.DestinationServiceName
	} else {
		// Set the source service name from the proxy's own registration
		args.ProxySourceService = serviceName
	}

	// In most cases where namespaces and partitions are enabled they will already be set
	// correctly because the http client that fetched this will provide them explicitly.
	// However, if these arguments were not provided, they will be empty even
	// though Namespaces and Partitions are actually being used.
	// Overriding them ensures that we always set the Namespace and Partition args
	// if the cluster is using them. This prevents us from defaulting to the "default"
	// when a non-default partition or namespace was inferred from the ACL token.
	if ns != "" {
		args.Namespace = ns
	}
	if partition != "" {
		args.Partition = partition
	}

	if datacenter != "" {
		// The agent will definitely have the definitive answer here.
		args.Datacenter = datacenter
	}

	// Setup ready listener for ingress gateway to pass healthcheck
	if c.gatewayKind == api.ServiceKindIngressGateway {
		lanAddr := c.lanAddress.String()
		// Deal with possibility of address not being specified and defaulting to
		// ":443"
		if strings.HasPrefix(lanAddr, ":") {
			lanAddr = "127.0.0.1" + lanAddr
		}
		bsCfg.ReadyBindAddr = lanAddr
	}

	if c.envoyReadyBindAddress != "" && c.envoyReadyBindPort != 0 {
		bsCfg.ReadyBindAddr = fmt.Sprintf("%s:%d", c.envoyReadyBindAddress, c.envoyReadyBindPort)
	}

	if !c.disableCentralConfig {
		// Parse the bootstrap config
		if err := mapstructure.WeakDecode(svcProxyConfig.Config, &bsCfg); err != nil {
			return nil, fmt.Errorf("failed parsing Proxy.Config: %s", err)
		}
	}

	return bsCfg.GenerateJSON(args, c.omitDeprecatedTags)
}

// TODO: make method a function
func (c *cmd) xdsAddress(httpCfg *api.Config) (GRPC, error) {
	g := GRPC{}

	addr := c.grpcAddr
	if addr == "" {
		port, err := c.lookupXDSPort()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		}
		if port <= 0 {
			// This is the dev mode default and recommended production setting if
			// enabled.
			port = 8502
		}
		addr = fmt.Sprintf("localhost:%v", port)
	}

	// TODO: parse addr as a url instead of strings.HasPrefix/TrimPrefix

	// Decide on TLS if the scheme is provided and indicates it, if the HTTP env
	// suggests TLS is supported explicitly (CONSUL_HTTP_SSL) or implicitly
	// (CONSUL_HTTP_ADDR) is https://
	switch {
	case strings.HasPrefix(strings.ToLower(addr), "https://"):
		g.AgentTLS = true
	case httpCfg.Scheme == "https":
		g.AgentTLS = true
	}

	// We want to allow grpcAddr set as host:port with no scheme but if the host
	// is an IP this will fail to parse as a URL with "parse 127.0.0.1:8500: first
	// path segment in URL cannot contain colon". On the other hand we also
	// support both http(s)://host:port and unix:///path/to/file.
	if grpcAddr := strings.TrimPrefix(addr, "unix://"); grpcAddr != addr {
		// Path to unix socket
		g.AgentSocket = grpcAddr
	} else {
		// Parse as host:port with option http prefix
		grpcAddr = strings.TrimPrefix(addr, "http://")
		grpcAddr = strings.TrimPrefix(grpcAddr, "https://")

		var err error
		var host string
		host, g.AgentPort, err = net.SplitHostPort(grpcAddr)
		if err != nil {
			return g, fmt.Errorf("Invalid Consul HTTP address: %s", err)
		}

		// We use STATIC for agent which means we need to resolve DNS names like
		// `localhost` ourselves. We could use STRICT_DNS or LOGICAL_DNS with envoy
		// but Envoy resolves `localhost` differently to go on macOS at least which
		// causes paper cuts like default dev agent (which binds specifically to
		// 127.0.0.1) isn't reachable since Envoy resolves localhost to `[::]` and
		// can't connect.
		agentIP, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return g, fmt.Errorf("Failed to resolve agent address: %s", err)
		}
		g.AgentAddress = agentIP.String()
	}
	return g, nil
}

func (c *cmd) lookupXDSPort() (int, error) {
	self, err := c.client.Agent().Self()
	if err != nil {
		return 0, err
	}

	type response struct {
		XDS struct {
			Port int
		}
	}

	var resp response
	if err := mapstructure.Decode(self, &resp); err == nil && resp.XDS.Port != 0 {
		return resp.XDS.Port, nil
	}

	// Fallback to old API for the case where a new consul CLI is being used with
	// an older API version.
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

const (
	synopsis = "Runs or Configures Envoy as a Connect proxy"
	help     = `
Usage: consul connect envoy [options] [-- pass-through options]

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

  Additional arguments may be passed directly to Envoy by specifying a double
  dash followed by a list of options.

    $ consul connect envoy -sidecar-for web -- --log-level debug
`
)
