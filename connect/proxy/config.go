// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxy

import (
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/connect"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
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
func (c *Config) Service(client *api.Client, logger hclog.Logger) (*connect.Service, error) {
	return connect.NewServiceWithConfig(c.ProxiedServiceName, connect.Config{Client: client, Logger: logger, ServerNextProtos: []string{}})
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

// applyDefaults sets zero-valued params to a reasonable default.
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

// UpstreamConfig is an alias for api.Upstream so we can parse in a compatible
// way but define custom methods for accessing the opaque config metadata.
type UpstreamConfig api.Upstream

// ConnectTimeout returns the connect timeout field of the nested config struct
// or the default value.
func (uc *UpstreamConfig) ConnectTimeout() time.Duration {
	if ms, ok := uc.Config["connect_timeout_ms"].(int); ok {
		return time.Duration(ms) * time.Millisecond
	}
	return 10000 * time.Millisecond
}

// applyDefaults sets zero-valued params to a reasonable default.
func (uc *UpstreamConfig) applyDefaults() {
	if uc.DestinationType == "" {
		uc.DestinationType = "service"
	}
	if uc.DestinationNamespace == "" {
		uc.DestinationNamespace = "default"
	}
	if uc.DestinationPartition == "" {
		uc.DestinationPartition = "default"
	}
	if uc.LocalBindAddress == "" && uc.LocalBindSocketPath == "" {
		uc.LocalBindAddress = "127.0.0.1"
	}
}

// String returns a string that uniquely identifies the Upstream. Used for
// identifying the upstream in log output and map keys.
func (uc *UpstreamConfig) String() string {
	addr := uc.LocalBindSocketPath
	if addr == "" {
		addr = fmt.Sprintf(
			"%s:%d",
			uc.LocalBindAddress, uc.LocalBindPort)
	}
	return fmt.Sprintf("%s->%s:%s/%s/%s", addr,
		uc.DestinationType, uc.DestinationPartition, uc.DestinationNamespace, uc.DestinationName)
}

// UpstreamResolverFuncFromClient returns a closure that captures a consul
// client and when called provides a ConsulResolver that can resolve the given
// UpstreamConfig using the provided api.Client dependency.
func UpstreamResolverFuncFromClient(client *api.Client) func(cfg UpstreamConfig) (connect.Resolver, error) {
	return func(cfg UpstreamConfig) (connect.Resolver, error) {
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
			Partition:  cfg.DestinationPartition,
			Name:       cfg.DestinationName,
			Type:       typ,
			Datacenter: cfg.Datacenter,
		}, nil
	}
}

// ConfigWatcher is a simple interface to allow dynamic configurations from
// pluggable sources.
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

// AgentConfigWatcher watches the local Consul agent for proxy config changes.
type AgentConfigWatcher struct {
	client  *api.Client
	proxyID string
	logger  hclog.Logger
	ch      chan *Config
	plan    *watch.Plan
}

// NewAgentConfigWatcher creates an AgentConfigWatcher.
func NewAgentConfigWatcher(client *api.Client, proxyID string,
	logger hclog.Logger) (*AgentConfigWatcher, error) {
	w := &AgentConfigWatcher{
		client:  client,
		proxyID: proxyID,
		logger:  logger.With("service_id", proxyID),
		ch:      make(chan *Config),
	}

	// Setup watch plan for config
	plan, err := watch.Parse(map[string]interface{}{
		"type":       "agent_service",
		"service_id": w.proxyID,
	})
	if err != nil {
		return nil, err
	}
	w.plan = plan
	w.plan.HybridHandler = w.handler
	go w.plan.RunWithClientAndHclog(w.client, w.logger)
	return w, nil
}

func (w *AgentConfigWatcher) handler(blockVal watch.BlockingParamVal,
	val interface{}) {

	resp, ok := val.(*api.AgentService)
	if !ok {
		w.logger.Warn("proxy config watch returned bad response", "response", val)
		return
	}

	if resp.Kind != api.ServiceKindConnectProxy {
		w.logger.Error("service is not a valid connect proxy")
		return
	}

	// Create proxy config from the response
	cfg := &Config{
		// Token should be already setup in the client
		ProxiedServiceName:      resp.Proxy.DestinationServiceName,
		ProxiedServiceNamespace: "default",
	}

	if tRaw, ok := resp.Proxy.Config["telemetry"]; ok {
		err := mapstructure.Decode(tRaw, &cfg.Telemetry)
		if err != nil {
			w.logger.Warn("proxy telemetry config failed to parse", "error", err)
		}
	}

	// Unmarshal configs
	err := mapstructure.Decode(resp.Proxy.Config, &cfg.PublicListener)
	if err != nil {
		w.logger.Error("failed to parse public listener config", "error", err)
	}
	cfg.PublicListener.BindAddress = resp.Address
	cfg.PublicListener.BindPort = resp.Port
	if resp.Proxy.LocalServiceSocketPath != "" {
		w.logger.Error("Unhandled unix domain socket config %+v %+v", resp.Proxy, cfg.PublicListener)
	}
	cfg.PublicListener.LocalServiceAddress = ipaddr.FormatAddressPort(
		resp.Proxy.LocalServiceAddress, resp.Proxy.LocalServicePort)

	cfg.PublicListener.applyDefaults()

	for _, u := range resp.Proxy.Upstreams {
		uc := UpstreamConfig(u)
		uc.applyDefaults()
		cfg.Upstreams = append(cfg.Upstreams, uc)
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
