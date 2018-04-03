package proxy

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/hcl"
)

// Config is the publicly configurable state for an entire proxy instance. It's
// mostly used as the format for the local-file config mode which is mostly for
// dev/testing. In normal use, different parts of this config are pulled from
// different locations (e.g. command line, agent config endpoint, agent
// certificate endpoints).
type Config struct {
	// ProxyID is the identifier for this proxy as registered in Consul. It's only
	// guaranteed to be unique per agent.
	ProxyID string `json:"proxy_id" hcl:"proxy_id"`

	// Token is the authentication token provided for queries to the local agent.
	Token string `json:"token" hcl:"token"`

	// ProxiedServiceID is the identifier of the service this proxy is representing.
	ProxiedServiceID string `json:"proxied_service_id" hcl:"proxied_service_id"`

	// ProxiedServiceNamespace is the namespace of the service this proxy is
	// representing.
	ProxiedServiceNamespace string `json:"proxied_service_namespace" hcl:"proxied_service_namespace"`

	// PublicListener configures the mTLS listener.
	PublicListener PublicListenerConfig `json:"public_listener" hcl:"public_listener"`

	// Upstreams configures outgoing proxies for remote connect services.
	Upstreams []UpstreamConfig `json:"upstreams" hcl:"upstreams"`

	// DevCAFile allows passing the file path to PEM encoded root certificate
	// bundle to be used in development instead of the ones supplied by Connect.
	DevCAFile string `json:"dev_ca_file" hcl:"dev_ca_file"`

	// DevServiceCertFile allows passing the file path to PEM encoded service
	// certificate (client and server) to be used in development instead of the
	// ones supplied by Connect.
	DevServiceCertFile string `json:"dev_service_cert_file" hcl:"dev_service_cert_file"`

	// DevServiceKeyFile allows passing the file path to PEM encoded service
	// private key to be used in development instead of the ones supplied by
	// Connect.
	DevServiceKeyFile string `json:"dev_service_key_file" hcl:"dev_service_key_file"`

	// service is a connect.Service instance representing the proxied service. It
	// is created internally by the code responsible for setting up config as it
	// may depend on other external dependencies
	service *connect.Service
}

// PublicListenerConfig contains the parameters needed for the incoming mTLS
// listener.
type PublicListenerConfig struct {
	// BindAddress is the host:port the public mTLS listener will bind to.
	BindAddress string `json:"bind_address" hcl:"bind_address"`

	// LocalServiceAddress is the host:port for the proxied application. This
	// should be on loopback or otherwise protected as it's plain TCP.
	LocalServiceAddress string `json:"local_service_address" hcl:"local_service_address"`

	// LocalConnectTimeout is the timeout for establishing connections with the
	// local backend. Defaults to 1000 (1s).
	LocalConnectTimeoutMs int `json:"local_connect_timeout_ms" hcl:"local_connect_timeout_ms"`

	// HandshakeTimeout is the timeout for incoming mTLS clients to complete a
	// handshake. Setting this low avoids DOS by malicious clients holding
	// resources open. Defaults to 10000 (10s).
	HandshakeTimeoutMs int `json:"handshake_timeout_ms" hcl:"handshake_timeout_ms"`
}

// applyDefaults sets zero-valued params to a sane default.
func (plc *PublicListenerConfig) applyDefaults() {
	if plc.LocalConnectTimeoutMs == 0 {
		plc.LocalConnectTimeoutMs = 1000
	}
	if plc.HandshakeTimeoutMs == 0 {
		plc.HandshakeTimeoutMs = 10000
	}
}

// UpstreamConfig configures an upstream (outgoing) listener.
type UpstreamConfig struct {
	// LocalAddress is the host:port to listen on for local app connections.
	LocalBindAddress string `json:"local_bind_address" hcl:"local_bind_address,attr"`

	// DestinationName is the service name of the destination.
	DestinationName string `json:"destination_name" hcl:"destination_name,attr"`

	// DestinationNamespace is the namespace of the destination.
	DestinationNamespace string `json:"destination_namespace" hcl:"destination_namespace,attr"`

	// DestinationType determines which service discovery method is used to find a
	// candidate instance to connect to.
	DestinationType string `json:"destination_type" hcl:"destination_type,attr"`

	// DestinationDatacenter is the datacenter the destination is in. If empty,
	// defaults to discovery within the same datacenter.
	DestinationDatacenter string `json:"destination_datacenter" hcl:"destination_datacenter,attr"`

	// ConnectTimeout is the timeout for establishing connections with the remote
	// service instance. Defaults to 10,000 (10s).
	ConnectTimeoutMs int `json:"connect_timeout_ms" hcl:"connect_timeout_ms,attr"`

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
}

// String returns a string that uniquely identifies the Upstream. Used for
// identifying the upstream in log output and map keys.
func (uc *UpstreamConfig) String() string {
	return fmt.Sprintf("%s->%s:%s/%s", uc.LocalBindAddress, uc.DestinationType,
		uc.DestinationNamespace, uc.DestinationName)
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
}

// Watch implements ConfigWatcher.
func (w *AgentConfigWatcher) Watch() <-chan *Config {
	watch := make(chan *Config)
	// TODO implement me, note we need to discover the Service instance to use and
	// set it on the Config we return.
	return watch
}
