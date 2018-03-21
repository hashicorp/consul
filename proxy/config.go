package proxy

import (
	"io/ioutil"

	"github.com/hashicorp/consul/api"
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

	// ProxiedServiceName is the name of the service this proxy is representing.
	ProxiedServiceName string `json:"proxied_service_name" hcl:"proxied_service_name"`

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

// ParseConfigFile parses proxy configuration form a file for local dev.
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

	return &cfg, nil
}

// AgentConfigWatcher watches the local Consul agent for proxy config changes.
type AgentConfigWatcher struct {
	client *api.Client
}

// Watch implements ConfigWatcher.
func (w *AgentConfigWatcher) Watch() <-chan *Config {
	watch := make(chan *Config)
	// TODO implement me
	return watch
}
