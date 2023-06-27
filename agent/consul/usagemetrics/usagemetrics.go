// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package usagemetrics

import (
	"context"
	"errors"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/logging"
)

var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"consul", "state", "nodes"},
		Help: "Deprecated - please use state_nodes instead.",
	},
	{
		Name: []string{"state", "nodes"},
		Help: "Measures the current number of nodes registered with Consul. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "state", "peerings"},
		Help: "Deprecated - please use state_peerings instead.",
	},
	{
		Name: []string{"state", "peerings"},
		Help: "Measures the current number of peerings registered with Consul. It is only emitted by Consul servers. Added in v1.13.0.",
	},
	{
		Name: []string{"consul", "state", "services"},
		Help: "Deprecated - please use state_services instead.",
	},
	{
		Name: []string{"state", "services"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "state", "service_instances"},
		Help: "Deprecated - please use state_service_instances instead.",
	},
	{
		Name: []string{"state", "service_instances"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "members", "clients"},
		Help: "Deprecated - please use members_clients instead.",
	},
	{
		Name: []string{"members", "clients"},
		Help: "Measures the current number of client agents registered with Consul. It is only emitted by Consul servers. Added in v1.9.6.",
	},
	{
		Name: []string{"consul", "members", "servers"},
		Help: "Deprecated - please use members_servers instead.",
	},
	{
		Name: []string{"members", "servers"},
		Help: "Measures the current number of server agents registered with Consul. It is only emitted by Consul servers. Added in v1.9.6.",
	},
	{
		Name: []string{"consul", "state", "kv_entries"},
		Help: "Deprecated - please use kv_entries instead.",
	},
	{
		Name: []string{"state", "kv_entries"},
		Help: "Measures the current number of entries in the Consul KV store. It is only emitted by Consul servers. Added in v1.10.3.",
	},
	{
		Name: []string{"consul", "state", "connect_instances"},
		Help: "Deprecated - please use state_connect_instances instead.",
	},
	{
		Name: []string{"state", "connect_instances"},
		Help: "Measures the current number of unique connect service instances registered with Consul, labeled by Kind. It is only emitted by Consul servers. Added in v1.10.4.",
	},
	{
		Name: []string{"consul", "state", "config_entries"},
		Help: "Deprecated - please use state_config_entries instead.",
	},
	{
		Name: []string{"state", "config_entries"},
		Help: "Measures the current number of unique configuration entries registered with Consul, labeled by Kind. It is only emitted by Consul servers. Added in v1.10.4.",
	},
	{
		Name: []string{"state", "billable_service_instances"},
		Help: "Total number of billable service instances in the local datacenter.",
	},
}

type getMembersFunc func() []serf.Member

// Config holds the settings for various parameters for the
// UsageMetricsReporter
type Config struct {
	logger         hclog.Logger
	metricLabels   []metrics.Label
	stateProvider  StateProvider
	tickerInterval time.Duration
	getMembersFunc getMembersFunc
}

// WithDatacenter adds the datacenter as a label to all metrics emitted by the
// UsageMetricsReporter
func (c *Config) WithDatacenter(dc string) *Config {
	c.metricLabels = append(c.metricLabels, metrics.Label{Name: "datacenter", Value: dc})
	return c
}

// WithLogger takes a logger and creates a new, named sub-logger to use when
// running
func (c *Config) WithLogger(logger hclog.Logger) *Config {
	c.logger = logger.Named(logging.UsageMetrics)
	return c
}

// WithReportingInterval specifies the interval on which UsageMetricsReporter
// should emit metrics
func (c *Config) WithReportingInterval(dur time.Duration) *Config {
	c.tickerInterval = dur
	return c
}

func (c *Config) WithStateProvider(sp StateProvider) *Config {
	c.stateProvider = sp
	return c
}

// WithGetMembersFunc specifies the function used to identify cluster members
func (c *Config) WithGetMembersFunc(fn getMembersFunc) *Config {
	c.getMembersFunc = fn
	return c
}

// StateProvider defines an inteface for retrieving a state.Store handle. In
// non-test code, this is satisfied by the fsm.FSM struct.
type StateProvider interface {
	State() *state.Store
}

// UsageMetricsReporter provides functionality for emitting usage metrics into
// the metrics stream. This makes it essentially a translation layer
// between the state store and metrics stream.
type UsageMetricsReporter struct {
	logger         hclog.Logger
	metricLabels   []metrics.Label
	stateProvider  StateProvider
	tickerInterval time.Duration
	getMembersFunc getMembersFunc
}

func NewUsageMetricsReporter(cfg *Config) (*UsageMetricsReporter, error) {
	if cfg.stateProvider == nil {
		return nil, errors.New("must provide a StateProvider to usage reporter")
	}

	if cfg.getMembersFunc == nil {
		return nil, errors.New("must provide a getMembersFunc to usage reporter")
	}

	if cfg.logger == nil {
		cfg.logger = hclog.NewNullLogger()
	}

	if cfg.tickerInterval == 0 {
		// Metrics are aggregated every 10 seconds, so we default to that.
		cfg.tickerInterval = 10 * time.Second
	}

	u := &UsageMetricsReporter{
		logger:         cfg.logger,
		stateProvider:  cfg.stateProvider,
		metricLabels:   cfg.metricLabels,
		tickerInterval: cfg.tickerInterval,
		getMembersFunc: cfg.getMembersFunc,
	}

	return u, nil
}

// Run must be run in a goroutine, and can be stopped by closing or sending
// data to the passed in shutdownCh
func (u *UsageMetricsReporter) Run(ctx context.Context) {
	ticker := time.NewTicker(u.tickerInterval)
	for {
		select {
		case <-ctx.Done():
			u.logger.Debug("usage metrics reporter shutting down")
			ticker.Stop()
			return
		case <-ticker.C:
			u.runOnce()
		}
	}
}

func (u *UsageMetricsReporter) runOnce() {
	u.logger.Trace("Starting usage run")
	state := u.stateProvider.State()

	_, nodeUsage, err := state.NodeUsage()
	if err != nil {
		u.logger.Warn("failed to retrieve nodes from state store", "error", err)
	}

	u.emitNodeUsage(nodeUsage)

	_, peeringUsage, err := state.PeeringUsage()
	if err != nil {
		u.logger.Warn("failed to retrieve peerings from state store", "error", err)
	}

	u.emitPeeringUsage(peeringUsage)

	_, serviceUsage, err := state.ServiceUsage(nil)
	if err != nil {
		u.logger.Warn("failed to retrieve services from state store", "error", err)
	}

	u.emitServiceUsage(serviceUsage)

	members := u.memberUsage()
	u.emitMemberUsage(members)

	_, kvUsage, err := state.KVUsage()
	if err != nil {
		u.logger.Warn("failed to retrieve kv entry usage from state store", "error", err)
	}

	u.emitKVUsage(kvUsage)

	_, configUsage, err := state.ConfigEntryUsage()
	if err != nil {
		u.logger.Warn("failed to retrieve config usage from state store", "error", err)
	}

	u.emitConfigEntryUsage(configUsage)
}

func (u *UsageMetricsReporter) memberUsage() []serf.Member {
	if u.getMembersFunc == nil {
		return nil
	}

	mems := u.getMembersFunc()
	if len(mems) <= 0 {
		u.logger.Warn("cluster reported zero members")
	}

	out := make([]serf.Member, 0, len(mems))
	for _, m := range mems {
		if m.Status != serf.StatusAlive {
			continue
		}
		out = append(out, m)
	}

	return out
}
