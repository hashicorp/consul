package proxy

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/watch"
	"github.com/hashicorp/hcl"
)

// Config is the publicly configurable state for an entire proxy instance. It's
// mostly used as the format for the local-file config mode which is mostly for
// dev/testing. In normal use, different parts of this config are pulled from
// different locations (e.g. command line, agent config endpoint, agent
// certificate endpoints).
type Config struct {
	// Token is the authentication token provided for queries to the local agent.
	Token string `json:"token" hcl:"token"`

	// ProxiedServiceName is the name of the service this proxy is representing.
	// This is the service _name_ and not the service _id_. This allows the
	// proxy to represent services not present in the local catalog.
	//
	// ProxiedServiceNamespace is the namespace of the service this proxy is
	// representing.
	ProxiedServiceName      string `json:"proxied_service_name" hcl:"proxied_service_name"`
	ProxiedServiceNamespace string `json:"proxied_service_namespace" hcl:"proxied_service_namespace"`

	// PublicListener configures the mTLS listener.
	PublicListener PublicListenerConfig `json:"public_listener" hcl:"public_listener"`

	// Upstreams configures outgoing proxies for remote connect services.
	Upstreams []UpstreamConfig `json:"upstreams" hcl:"upstreams"`

	// Telemetry stores configuration for go-metrics. It is typically populated
	// from the agent's runtime config via the proxy config endpoint so that the
	// proxy will log metrics to the same location(s) as the agent.
	Telemetry lib.TelemetryConfig
}

// Service returns the *connect.Service structure represented by this config.
func (c *Config) Service(client *api.Client, logger *log.Logger) (*connect.Service, error) {
	return connect.NewServiceWithLogger(c.ProxiedServiceName, client, logger)
}

// PublicListenerConfig contains the parameters needed for the incoming mTLS
// listener.
type PublicListenerConfig struct {
	// BindAddress is the host/IP the public mTLS listener will bind to.
	//
	// BindPort is the port the public listener will bind to.
	BindAddress string `json:"bind_address" hcl:"bind_address" mapstructure:"bind_address"`
	BindPort    int    `json:"bind_port" hcl:"bind_port" mapstructure:"bind_port"`

	// LocalServiceAddress is the host:port for the proxied application. This
	// should be on loopback or otherwise protected as it's plain TCP.
	LocalServiceAddress string `json:"local_service_address" hcl:"local_service_address" mapstructure:"local_service_address"`

	// LocalConnectTimeout is the timeout for establishing connections with the
	// local backend. Defaults to 1000 (1s).
	LocalConnectTimeoutMs int `json:"local_connect_timeout_ms" hcl:"local_connect_timeout_ms" mapstructure:"local_connect_timeout_ms"`

	// HandshakeTimeout is the timeout for incoming mTLS clients to complete a
	// handshake. Setting this low avoids DOS by malicious clients holding
	// resources open. Defaults to 10000 (10s).
	HandshakeTimeoutMs int `json:"handshake_timeout_ms" hcl:"handshake_timeout_ms" mapstructure:"handshake_timeout_ms"`
}

// applyDefaults sets zero-valued params to a sane default.
func (plc *PublicListenerConfig) applyDefaults() {
	if plc.LocalConnectTimeoutMs == 0 {
		plc.LocalConnectTimeoutMs = 1000
	}
	if plc.HandshakeTimeoutMs == 0 {
		plc.HandshakeTimeoutMs = 10000
	}
	if plc.BindAddress == "" {
		plc.BindAddress = "0.0.0.0"
	}
}

// UpstreamConfig configures an upstream (outgoing) listener.
type UpstreamConfig struct {
	// LocalAddress is the host/ip to listen on for local app connections. Defaults to 127.0.0.1.
	LocalBindAddress string `json:"local_bind_address" hcl:"local_bind_address,attr" mapstructure:"local_bind_address"`

	LocalBindPort int `json:"local_bind_port" hcl:"local_bind_port,attr" mapstructure:"local_bind_port"`

	// DestinationName is the service name of the destination.
	DestinationName string `json:"destination_name" hcl:"destination_name,attr" mapstructure:"destination_name"`

	// DestinationNamespace is the namespace of the destination.
	DestinationNamespace string `json:"destination_namespace" hcl:"destination_namespace,attr" mapstructure:"destination_namespace"`

	// DestinationType determines which service discovery method is used to find a
	// candidate instance to connect to.
	DestinationType string `json:"destination_type" hcl:"destination_type,attr" mapstructure:"destination_type"`

	// DestinationDatacenter is the datacenter the destination is in. If empty,
	// defaults to discovery within the same datacenter.
	DestinationDatacenter string `json:"destination_datacenter" hcl:"destination_datacenter,attr" mapstructure:"destination_datacenter"`

	// ConnectTimeout is the timeout for establishing connections with the remote
	// service instance. Defaults to 10,000 (10s).
	ConnectTimeoutMs int `json:"connect_timeout_ms" hcl:"connect_timeout_ms,attr" mapstructure:"connect_timeout_ms"`

	// resolver is used to plug in the service discover mechanism. It can be used
	// in tests to bypass discovery. In real usage it is used to inject the
	// api.Client dependency from the remainder of the config struct parsed from
	// the user JSON using the UpstreamResolverFromClient helper.
	resolver connect.Resolver
}

// applyDefaults sets zero-valued params to a sane default.
func (uc *UpstreamConfig) applyDefaults() {
	if uc.ConnectTimeoutMs == 0 {
		uc.ConnectTimeoutMs = 10000
	}
	if uc.DestinationType == "" {
		uc.DestinationType = "service"
	}
	if uc.DestinationNamespace == "" {
		uc.DestinationNamespace = "default"
	}
	if uc.LocalBindAddress == "" {
		uc.LocalBindAddress = "127.0.0.1"
	}
}

// String returns a string that uniquely identifies the Upstream. Used for
// identifying the upstream in log output and map keys.
func (uc *UpstreamConfig) String() string {
	return fmt.Sprintf("%s:%d->%s:%s/%s", uc.LocalBindAddress, uc.LocalBindPort,
		uc.DestinationType, uc.DestinationNamespace, uc.DestinationName)
}

// UpstreamResolverFromClient returns a ConsulResolver that can resolve the
// given UpstreamConfig using the provided api.Client dependency.
func UpstreamResolverFromClient(client *api.Client,
	cfg UpstreamConfig) connect.Resolver {

	// For now default to service as it has the most natural meaning and the error
	// that the service doesn't exist is probably reasonable if misconfigured. We
	// should probably handle actual configs that have invalid types at a higher
	// level anyway (like when parsing).
	typ := connect.ConsulResolverTypeService
	if cfg.DestinationType == "prepared_query" {
		typ = connect.ConsulResolverTypePreparedQuery
	}
	return &connect.ConsulResolver{
		Client:     client,
		Namespace:  cfg.DestinationNamespace,
		Name:       cfg.DestinationName,
		Type:       typ,
		Datacenter: cfg.DestinationDatacenter,
	}
}

// ConfigWatcher is a simple interface to allow dynamic configurations from
// plugggable sources.
type ConfigWatcher interface {
	// Watch returns a channel that will deliver new Configs if something external
	// provokes it.
	Watch() <-chan *Config
}

// StaticConfigWatcher is a simple ConfigWatcher that delivers a static Config
// once and then never changes it.
type StaticConfigWatcher struct {
	ch chan *Config
}

// NewStaticConfigWatcher returns a ConfigWatcher for a config that never
// changes. It assumes only one "watcher" will ever call Watch. The config is
// delivered on the first call but will never be delivered again to allow
// callers to call repeatedly (e.g. select in a loop).
func NewStaticConfigWatcher(cfg *Config) *StaticConfigWatcher {
	sc := &StaticConfigWatcher{
		// Buffer it so we can queue up the config for first delivery.
		ch: make(chan *Config, 1),
	}
	sc.ch <- cfg
	return sc
}

// Watch implements ConfigWatcher on a static configuration for compatibility.
// It returns itself on the channel once and then leaves it open.
func (sc *StaticConfigWatcher) Watch() <-chan *Config {
	return sc.ch
}

// ParseConfigFile parses proxy configuration from a file for local dev.
func ParseConfigFile(filename string) (*Config, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config

	err = hcl.Unmarshal(bs, &cfg)
	if err != nil {
		return nil, err
	}

	cfg.PublicListener.applyDefaults()
	for idx := range cfg.Upstreams {
		cfg.Upstreams[idx].applyDefaults()
	}

	return &cfg, nil
}

// AgentConfigWatcher watches the local Consul agent for proxy config changes.
type AgentConfigWatcher struct {
	client  *api.Client
	proxyID string
	logger  *log.Logger
	ch      chan *Config
	plan    *watch.Plan
}

// NewAgentConfigWatcher creates an AgentConfigWatcher.
func NewAgentConfigWatcher(client *api.Client, proxyID string,
	logger *log.Logger) (*AgentConfigWatcher, error) {
	w := &AgentConfigWatcher{
		client:  client,
		proxyID: proxyID,
		logger:  logger,
		ch:      make(chan *Config),
	}

	// Setup watch plan for config
	plan, err := watch.Parse(map[string]interface{}{
		"type":             "connect_proxy_config",
		"proxy_service_id": w.proxyID,
	})
	if err != nil {
		return nil, err
	}
	w.plan = plan
	w.plan.HybridHandler = w.handler
	go w.plan.RunWithClientAndLogger(w.client, w.logger)
	return w, nil
}

func (w *AgentConfigWatcher) handler(blockVal watch.BlockingParamVal,
	val interface{}) {

	resp, ok := val.(*api.ConnectProxyConfig)
	if !ok {
		w.logger.Printf("[WARN] proxy config watch returned bad response: %v", val)
		return
	}

	// Create proxy config from the response
	cfg := &Config{
		// Token should be already setup in the client
		ProxiedServiceName:      resp.TargetServiceName,
		ProxiedServiceNamespace: "default",
	}

	if tRaw, ok := resp.Config["telemetry"]; ok {
		err := mapstructure.Decode(tRaw, &cfg.Telemetry)
		if err != nil {
			w.logger.Printf("[WARN] proxy telemetry config failed to parse: %s", err)
		}
	}

	// Unmarshal configs
	err := mapstructure.Decode(resp.Config, &cfg.PublicListener)
	if err != nil {
		w.logger.Printf("[ERR] proxy config watch public listener config "+
			"couldn't be parsed: %s", err)
		return
	}
	cfg.PublicListener.applyDefaults()

	err = mapstructure.Decode(resp.Config["upstreams"], &cfg.Upstreams)
	if err != nil {
		w.logger.Printf("[ERR] proxy config watch upstream listener config "+
			"couldn't be parsed: %s", err)
		return
	}
	for i := range cfg.Upstreams {
		cfg.Upstreams[i].applyDefaults()
	}

	// Parsed config OK, deliver it!
	w.ch <- cfg
}

// Watch implements ConfigWatcher.
func (w *AgentConfigWatcher) Watch() <-chan *Config {
	return w.ch
}

// Close frees watcher resources and implements io.Closer
func (w *AgentConfigWatcher) Close() error {
	if w.plan != nil {
		w.plan.Stop()
	}
	return nil
}
