package lib

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
)

func statsiteSink(cfg map[string]interface{}, hostname string) (metrics.MetricSink, error) {
	addr := cfgStringVal(cfg["StatsiteAddr"])
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(addr)
}

func statsdSink(cfg map[string]interface{}, hostname string) (metrics.MetricSink, error) {
	addr := cfgStringVal(cfg["StatsdAddr"])
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(addr)
}

func dogstatdSink(cfg map[string]interface{}, hostname string) (metrics.MetricSink, error) {
	addr := cfgStringVal(cfg["DogstatsdAddr"])
	if addr == "" {
		return nil, nil
	}
	sink, err := datadog.NewDogStatsdSink(addr, hostname)
	if err != nil {
		return nil, err
	}
	sink.SetTags(cfgStrSliceVal(cfg["DogstatsdTags"]))
	return sink, nil
}

func prometheusSink(cfg map[string]interface{}, hostname string) (metrics.MetricSink, error) {
	if cfgDurationVal(cfg["PrometheusRetentionTime"]).Nanoseconds() < 1 {
		return nil, nil
	}
	prometheusOpts := prometheus.PrometheusOpts{
		Expiration: cfgDurationVal(cfg["PrometheusRetentionTime"]),
	}
	sink, err := prometheus.NewPrometheusSinkFrom(prometheusOpts)
	if err != nil {
		return nil, err
	}
	return sink, nil
}

func circonusSink(cfg map[string]interface{}, hostname string) (metrics.MetricSink, error) {
	token := cfgStringVal(cfg["CirconusAPIToken"])
	url := cfgStringVal(cfg["CirconusSubmissionURL"])
	if token == "" && url == "" {
		return nil, nil
	}

	conf := &circonus.Config{}
	conf.Interval = cfgStringVal(cfg["CirconusSubmissionInterval"])
	conf.CheckManager.API.TokenKey = token
	conf.CheckManager.API.TokenApp = cfgStringVal(cfg["CirconusAPIApp"])
	conf.CheckManager.API.URL = cfgStringVal(cfg["CirconusAPIURL"])
	conf.CheckManager.Check.SubmissionURL = url
	conf.CheckManager.Check.ID = cfgStringVal(cfg["CirconusCheckID"])
	conf.CheckManager.Check.ForceMetricActivation = cfgStringVal(cfg["CirconusCheckForceMetricActivation"])
	conf.CheckManager.Check.InstanceID = cfgStringVal(cfg["CirconusCheckInstanceID"])
	conf.CheckManager.Check.SearchTag = cfgStringVal(cfg["CirconusCheckSearchTag"])
	conf.CheckManager.Check.DisplayName = cfgStringVal(cfg["CirconusCheckDisplayName"])
	conf.CheckManager.Check.Tags = cfgStringVal(cfg["CirconusCheckTags"])
	conf.CheckManager.Broker.ID = cfgStringVal(cfg["CirconusBrokerID"])
	conf.CheckManager.Broker.SelectTag = cfgStringVal(cfg["CirconusBrokerSelectTag"])

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

func cfgStringVal(i interface{}) string {
	v, ok := i.(string)
	if ok {
		return v
	}
	return ""
}
func cfgBoolVal(i interface{}) bool {
	v, ok := i.(bool)
	if ok {
		return v
	}
	return false
}
func cfgDurationVal(i interface{}) time.Duration {
	v, ok := i.(time.Duration)
	if ok {
		return v
	}
	return time.Duration(0)
}
func cfgStrSliceVal(i interface{}) []string {
	v, ok := i.([]string)
	if ok {
		return v
	}
	return nil
}

// StartupTelemetry configures go-metrics based on map of telemetry config
// values as returned by RuntimecfgStringVal(cfg["Config"])().
func StartupTelemetry(cfg map[string]interface{}) (*metrics.InmemSink, error) {
	// Setup telemetry
	// Aggregate on 10 second intervals for 1 minute. Expose the
	// metrics over stderr when there is a SIGUSR1 received.
	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)
	metricsConf := metrics.DefaultConfig(cfgStringVal(cfg["MetricsPrefix"]))
	metricsConf.EnableHostname = !cfgBoolVal(cfg["DisableHostname"])
	metricsConf.FilterDefault = cfgBoolVal(cfg["FilterDefault"])
	metricsConf.AllowedPrefixes = cfgStrSliceVal(cfg["AllowedPrefixes"])
	metricsConf.BlockedPrefixes = cfgStrSliceVal(cfg["BlockedPrefixes"])

	var sinks metrics.FanoutSink
	addSink := func(name string, fn func(map[string]interface{}, string) (metrics.MetricSink, error)) error {
		s, err := fn(cfg, metricsConf.HostName)
		if err != nil {
			return err
		}
		if s != nil {
			sinks = append(sinks, s)
		}
		return nil
	}

	if err := addSink("statsite", statsiteSink); err != nil {
		return nil, err
	}
	if err := addSink("statsd", statsdSink); err != nil {
		return nil, err
	}
	if err := addSink("dogstatd", dogstatdSink); err != nil {
		return nil, err
	}
	if err := addSink("circonus", circonusSink); err != nil {
		return nil, err
	}
	if err := addSink("prometheus", prometheusSink); err != nil {
		return nil, err
	}

	if len(sinks) > 0 {
		sinks = append(sinks, memSink)
		metrics.NewGlobal(metricsConf, sinks)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, memSink)
	}
	return memSink, nil
}
