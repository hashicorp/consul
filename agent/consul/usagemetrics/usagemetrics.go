// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package usagemetrics

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/go-metrics/prometheus"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/version"
)

var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"state", "nodes"},
		Help: "Measures the current number of nodes registered with Consul. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"state", "peerings"},
		Help: "Measures the current number of peerings registered with Consul. It is only emitted by Consul servers. Added in v1.13.0.",
	},
	{
		Name: []string{"state", "services"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"state", "service_instances"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"members", "clients"},
		Help: "Measures the current number of client agents registered with Consul. It is only emitted by Consul servers. Added in v1.9.6.",
	},
	{
		Name: []string{"members", "servers"},
		Help: "Measures the current number of server agents registered with Consul. It is only emitted by Consul servers. Added in v1.9.6.",
	},
	{
		Name: []string{"state", "kv_entries"},
		Help: "Measures the current number of entries in the Consul KV store. It is only emitted by Consul servers. Added in v1.10.3.",
	},
	{
		Name: []string{"state", "connect_instances"},
		Help: "Measures the current number of unique connect service instances registered with Consul, labeled by Kind. It is only emitted by Consul servers. Added in v1.10.4.",
	},
	{
		Name: []string{"state", "config_entries"},
		Help: "Measures the current number of unique configuration entries registered with Consul, labeled by Kind. It is only emitted by Consul servers. Added in v1.10.4.",
	},
	{
		Name: []string{"state", "billable_service_instances"},
		Help: "Total number of billable service instances in the local datacenter.",
	},
	{
		Name: []string{"version"},
		Help: "Represents the Consul version.",
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
	excludeTenancy bool
}

// WithDatacenter adds the datacenter as a label to all metrics emitted by the
// UsageMetricsReporter
func (c *Config) WithDatacenter(dc string) *Config {
	c.metricLabels = append(c.metricLabels, metrics.Label{Name: "datacenter", Value: dc})
	return c
}

// WithDisabledTenancyMetrics opts the user out of specifying usage metrics for each tenancy.
func (c *Config) WithDisabledTenancyMetrics(disabled bool) *Config {
	c.excludeTenancy = disabled
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
	excludeTenancy bool

	usageReporter
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
		excludeTenancy: cfg.excludeTenancy,
	}

	u.usageReporter = newTenancyUsageReporter(u)

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

	_, serviceUsage, err := state.ServiceUsage(nil, !u.excludeTenancy)
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
	u.emitVersion()
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

func (u *UsageMetricsReporter) emitVersion() {
	// consul version metric with labels
	metrics.SetGaugeWithLabels(
		[]string{"version"},
		1,
		[]metrics.Label{
			{Name: "version", Value: versionWithMetadata()},
			{Name: "pre_release", Value: version.VersionPrerelease},
		},
	)
}

func versionWithMetadata() string {
	vsn := version.Version
	metadata := version.VersionMetadata

	if metadata != "" {
		vsn += "+" + metadata
	}

	return vsn
}

type usageReporter interface {
	emitNodeUsage(nodeUsage state.NodeUsage)
	emitPeeringUsage(peeringUsage state.PeeringUsage)
	emitMemberUsage(members []serf.Member)
	emitServiceUsage(serviceUsage structs.ServiceUsage)
	emitKVUsage(kvUsage state.KVUsage)
	emitConfigEntryUsage(configUsage state.ConfigEntryUsage)
}

type baseUsageReporter struct {
	metricLabels []metrics.Label
}

var _ usageReporter = (*baseUsageReporter)(nil)

func newBaseUsageReporter(u *UsageMetricsReporter) *baseUsageReporter {
	return &baseUsageReporter{
		metricLabels: u.metricLabels,
	}
}

func (u *baseUsageReporter) emitNodeUsage(nodeUsage state.NodeUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"state", "nodes"},
		float32(nodeUsage.Nodes),
		u.metricLabels,
	)
}

func (u *baseUsageReporter) emitPeeringUsage(peeringUsage state.PeeringUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"state", "peerings"},
		float32(peeringUsage.Peerings),
		u.metricLabels,
	)
}

func (u *baseUsageReporter) emitMemberUsage(members []serf.Member) {
	var (
		servers int
		clients int
	)
	for _, m := range members {
		switch m.Tags["role"] {
		case "node":
			clients++
		case "consul":
			servers++
		}
	}

	metrics.SetGaugeWithLabels(
		[]string{"members", "clients"},
		float32(clients),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"members", "servers"},
		float32(servers),
		u.metricLabels,
	)
}

func (u *baseUsageReporter) emitServiceUsage(serviceUsage structs.ServiceUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"state", "services"},
		float32(serviceUsage.Services),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"state", "service_instances"},
		float32(serviceUsage.ServiceInstances),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "billable_service_instances"},
		float32(serviceUsage.BillableServiceInstances),
		u.metricLabels,
	)

	for k, i := range serviceUsage.ConnectServiceInstances {
		metrics.SetGaugeWithLabels(
			[]string{"state", "connect_instances"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
	}
}

func (u *baseUsageReporter) emitKVUsage(kvUsage state.KVUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"state", "kv_entries"},
		float32(kvUsage.KVCount),
		u.metricLabels,
	)
}

func (u *baseUsageReporter) emitConfigEntryUsage(configUsage state.ConfigEntryUsage) {
	for k, i := range configUsage.ConfigByKind {
		metrics.SetGaugeWithLabels(
			[]string{"state", "config_entries"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
	}
}
