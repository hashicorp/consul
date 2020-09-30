package telemetry

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
)

// DefaultMetrics provides a MetricsClient implementation for armon/go-metrics.
type DefaultMetrics struct {
	client    *metrics.Metrics
	inmemSink *metrics.InmemSink
}

func (c DefaultMetrics) AddSample(key []string, start float32, labels ...Label) {
	c.client.AddSampleWithLabels(key, start, convertLabels(labels))
}

func (c DefaultMetrics) SetGauge(key []string, val float32, labels ...Label) {
	c.client.SetGaugeWithLabels(key, val, convertLabels(labels))
}

func (c DefaultMetrics) IncrCounter(key []string, val float32, labels ...Label) {
	c.client.IncrCounterWithLabels(key, val, convertLabels(labels))
}

func (c DefaultMetrics) MeasureSince(key []string, start time.Time, labels ...Label) {
	c.client.MeasureSinceWithLabels(key, start, convertLabels(labels))
}

func (c DefaultMetrics) GetInmemSink() *metrics.InmemSink {
	return c.inmemSink
}

// NoopClient does nothing. Is it pronounced like "boop" or "no op"? Up to you, friend.
type NoopClient struct{}

func (*NoopClient) SetGauge([]string, float32, ...Label)       {}
func (*NoopClient) IncrCounter([]string, float32, ...Label)    {}
func (*NoopClient) MeasureSince([]string, time.Time, ...Label) {}
func (*NoopClient) AddSample([]string, float32, ...Label)      {}
func (*NoopClient) GetInmemSink() *metrics.InmemSink           { return nil }

// Label provides a key and a value, offering an internal representation of labels used by most telemetry, tracing, and
//  event backends.
type Label struct {
	Key   string
	Value string
}

func convertLabels(labels []Label) []metrics.Label {
	if len(labels) == 0 {
		return nil
	}
	aLabels := make([]metrics.Label, len(labels))
	for i := 0; i < len(labels); i++ {
		aLabels[i] = metrics.Label{
			Name:  labels[i].Key,
			Value: labels[i].Value,
		}
	}
	return aLabels
}

// sinkFn takes Config and builds a sink to be composed in the FanOutSink
type sinkFn func(Config) (metrics.MetricSink, error)

func statsiteSink(cfg Config) (metrics.MetricSink, error) {
	addr := cfg.StatsiteAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(addr)
}

func statsdSink(cfg Config) (metrics.MetricSink, error) {
	addr := cfg.StatsdAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(addr)
}

func dogstatsdSink(cfg Config) (metrics.MetricSink, error) {
	addr := cfg.DogstatsdAddr
	if addr == "" {
		return nil, nil
	}
	sink, err := datadog.NewDogStatsdSink(addr, "")
	if err != nil {
		return nil, err
	}
	sink.SetTags(cfg.DogstatsdTags)
	return sink, nil
}

func prometheusSink(cfg Config) (metrics.MetricSink, error) {
	if cfg.PrometheusRetentionTime.Nanoseconds() < 1 {
		return nil, nil
	}
	prometheusOpts := prometheus.PrometheusOpts{
		Expiration: cfg.PrometheusRetentionTime,
	}
	sink, err := prometheus.NewPrometheusSinkFrom(prometheusOpts)
	if err != nil {
		return nil, err
	}
	return sink, nil
}

func circonusSink(cfg Config) (metrics.MetricSink, error) {
	token := cfg.CirconusAPIToken
	url := cfg.CirconusSubmissionURL
	if token == "" && url == "" {
		return nil, nil
	}

	conf := &circonus.Config{}
	conf.Interval = cfg.CirconusSubmissionInterval
	conf.CheckManager.API.TokenKey = token
	conf.CheckManager.API.TokenApp = cfg.CirconusAPIApp
	conf.CheckManager.API.URL = cfg.CirconusAPIURL
	conf.CheckManager.Check.SubmissionURL = url
	conf.CheckManager.Check.ID = cfg.CirconusCheckID
	conf.CheckManager.Check.ForceMetricActivation = cfg.CirconusCheckForceMetricActivation
	conf.CheckManager.Check.InstanceID = cfg.CirconusCheckInstanceID
	conf.CheckManager.Check.SearchTag = cfg.CirconusCheckSearchTag
	conf.CheckManager.Check.DisplayName = cfg.CirconusCheckDisplayName
	conf.CheckManager.Check.Tags = cfg.CirconusCheckTags
	conf.CheckManager.Broker.ID = cfg.CirconusBrokerID
	conf.CheckManager.Broker.SelectTag = cfg.CirconusBrokerSelectTag

	if conf.CheckManager.Check.DisplayName == "" {
		conf.CheckManager.Check.DisplayName = "Consul"
	}

	if conf.CheckManager.API.TokenApp == "" {
		conf.CheckManager.API.TokenApp = "consul"
	}

	if conf.CheckManager.Check.SearchTag == "" {
		conf.CheckManager.Check.SearchTag = "service:consul"
	}

	sink, err := circonus.NewCirconusSink(conf)
	if err != nil {
		return nil, err
	}
	sink.Start()
	return sink, nil
}

// initSinks composes all of our sink options from RuntimeCfg into a FanoutSink
func initSinks(cfg Config) (metrics.FanoutSink, error) {
	var sinks metrics.FanoutSink
	addSink := func(sinks metrics.FanoutSink, fn sinkFn, cfg Config) (metrics.MetricSink, error) {
		// Build the sink
		s, err := fn(cfg)
		if err != nil {
			return s, err
		}
		// Compose into FanoutSink
		if s != nil {
			sinks = append(sinks, s)
		}
		return s, nil
	}

	// Compose all of our external sinks with FanoutSink.
	// All sink inits must succeed - we abort setup if any of the configuration is invalid.
	if _, err := addSink(sinks, statsiteSink, cfg); err != nil {
		return sinks, err
	}

	if _, err := addSink(sinks, statsdSink, cfg); err != nil {
		return sinks, err
	}

	if _, err := addSink(sinks, dogstatsdSink, cfg); err != nil {
		return sinks, err
	}

	if _, err := addSink(sinks, circonusSink, cfg); err != nil {
		return sinks, err
	}

	if _, err := addSink(sinks, prometheusSink, cfg); err != nil {
		return sinks, err
	}
	return sinks, nil
}

// todo(kit): As a follow-up to this PR, we want to migrate away from many packages in Consul importing go-metrics.Metrics.
//  Migrating is not possible while we use the InmemSink to serve requests. First I want to get the MetricsClient to a
//  place that we're happy with it. Then we can fix Init to return a MetricsClient and replace the go-metrics dependencies
//  throughout Consul.
func Init(cfg Config) (*DefaultMetrics, error) {
	if cfg.Disable {
		return nil, nil
	}
	// Define an InmemSink so we can dump telemetry when we receive a process signal
	// Aggregate on 10 second intervals for 1 minute. Expose the
	// metrics over stderr when there is a SIGUSR1 received.
	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)

	// Union RuntimeCfg into go-metrics.Config, overriding defaults
	mCfg := metrics.DefaultConfig(cfg.MetricsPrefix)
	mCfg.EnableHostname = !cfg.DisableHostname
	mCfg.FilterDefault = cfg.FilterDefault
	mCfg.AllowedPrefixes = cfg.AllowedPrefixes
	mCfg.BlockedPrefixes = cfg.BlockedPrefixes

	sinks, err := initSinks(cfg)
	if err != nil {
		return nil, err
	}

	// client contains the API which lets us dispatch metrics events to our sinks
	var client *metrics.Metrics

	// No external sinks, we'll configure for in-process telemetry only
	if len(sinks) == 0 {
		// Hostname is irrelevant for on-host telemetry
		mCfg.EnableHostname = false
		// Store our metrics client globally and return a pointer to the client
		client, err = metrics.NewGlobal(mCfg, memSink)
		if err != nil {
			return nil, err
		}
		return &DefaultMetrics{
			client:    client,
			inmemSink: memSink,
		}, nil
	}

	// Compose our external sinks with the InmemSink
	sinks = append(sinks, memSink)
	// Store our metrics client globally and return a pointer to the client
	client, err = metrics.NewGlobal(mCfg, sinks)
	if err != nil {
		return nil, err
	}
	return &DefaultMetrics{client, memSink}, nil
}
