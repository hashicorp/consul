package usagemetrics

import (
	"context"
	"errors"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
)

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

// UsageMetricsReporter provides functionality for emitting usage metrics into
// the metrics stream. This makes it essentially a translation layer
// between the state store and metrics stream.
type UsageMetricsReporter struct {
	logger         hclog.Logger
	metricLabels   []metrics.Label
	stateProvider  StateProvider
	tickerInterval time.Duration
}

func NewUsageMetricsReporter(cfg *Config) (*UsageMetricsReporter, error) {
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

	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "services"},
		float32(serviceUsage.Services),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "service_instances"},
		float32(serviceUsage.ServiceInstances),
		u.metricLabels,
	)

	u.emitEnterpriseUsage(serviceUsage)
}
