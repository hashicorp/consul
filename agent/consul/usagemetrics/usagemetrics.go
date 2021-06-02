package usagemetrics

import (
	"context"
	"errors"
	"time"

	"github.com/armon/go-metrics/prometheus"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"consul", "state", "nodes"},
		Help: "Measures the current number of nodes registered with Consul. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "state", "services"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "state", "service_instances"},
		Help: "Measures the current number of unique services registered with Consul, based on service name. It is only emitted by Consul servers. Added in v1.9.0.",
	},
	{
		Name: []string{"consul", "state", "client_agents"},
		// TODO: Insert proper version
		Help: "Measures the current number of client agents registered with Consul. It is only emitted by Consul servers. Added in vX.X.X.",
	},
	{
		Name: []string{"consul", "state", "server_agents"},
		// TODO: Insert proper version
		Help: "Measures the current number of server agents registered with Consul. It is only emitted by Consul servers. Added in vX.X.X.",
	},
}

// Config holds the settings for various parameters for the
// UsageMetricsReporter
type Config struct {
	logger         hclog.Logger
	metricLabels   []metrics.Label
	stateProvider  StateProvider
	tickerInterval time.Duration
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

// StateProvider defines an inteface for retrieving a state.Store handle. In
// non-test code, this is satisfied by the fsm.FSM struct.
type StateProvider interface {
	State() *state.Store
}

type getMembersFunc func() []serf.Member

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

func NewUsageMetricsReporter(cfg *Config, getMembersFunc getMembersFunc) (*UsageMetricsReporter, error) {
	if cfg.stateProvider == nil {
		return nil, errors.New("must provide a StateProvider to usage reporter")
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
		getMembersFunc: getMembersFunc,
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
	state := u.stateProvider.State()
	_, nodes, err := state.NodeCount()
	if err != nil {
		u.logger.Warn("failed to retrieve nodes from state store", "error", err)
	}
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "nodes"},
		float32(nodes),
		u.metricLabels,
	)

	_, serviceUsage, err := state.ServiceUsage()
	if err != nil {
		u.logger.Warn("failed to retrieve services from state store", "error", err)
	}

	u.emitServiceUsage(serviceUsage)

	servers, clients := u.memberUsage()
	u.emitMemberUsage(servers, clients)
}

func (u *UsageMetricsReporter) memberUsage() (servers, clients int) {
	if u.getMembersFunc == nil {
		return 0, 0
	}

	members := u.getMembersFunc()

	numClients := 0
	numServers := 0

	for _, m := range members {
		if m.Status != serf.StatusAlive {
			continue
		}

		switch m.Tags["role"] {
		case "node":
			numClients++
		case "consul":
			numServers++
		}
	}

	return numServers, numClients
}

func (u *UsageMetricsReporter) emitMemberUsage(servers, clients int) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "client_agents"},
		float32(clients),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "server_agents"},
		float32(servers),
		u.metricLabels,
	)
}
