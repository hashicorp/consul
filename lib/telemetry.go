package lib

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	prometheuscore "github.com/prometheus/client_golang/prometheus"

	"github.com/hashicorp/consul/lib/retry"
)

// TelemetryConfig is embedded in config.RuntimeConfig and holds the
// configuration variables for go-metrics. It is a separate struct to allow it
// to be exported as JSON and passed to other process like managed connect
// proxies so they can inherit the agent's telemetry config.
//
// It is in lib package rather than agent/config because we need to use it in
// the shared InitTelemetry functions below, but we can't import agent/config
// due to a dependency cycle.
type TelemetryConfig struct {
	// Disable may be set to true to have InitTelemetry to skip initialization
	// and return a nil MetricsSink.
	Disable bool

	// Circonus*: see https://github.com/circonus-labs/circonus-gometrics
	// for more details on the various configuration options.
	// Valid configuration combinations:
	//    - CirconusAPIToken
	//      metric management enabled (search for existing check or create a new one)
	//    - CirconusSubmissionUrl
	//      metric management disabled (use check with specified submission_url,
	//      broker must be using a public SSL certificate)
	//    - CirconusAPIToken + CirconusCheckSubmissionURL
	//      metric management enabled (use check with specified submission_url)
	//    - CirconusAPIToken + CirconusCheckID
	//      metric management enabled (use check with specified id)

	// CirconusAPIApp is an app name associated with API token.
	// Default: "consul"
	//
	// hcl: telemetry { circonus_api_app = string }
	CirconusAPIApp string `json:"circonus_api_app,omitempty" mapstructure:"circonus_api_app"`

	// CirconusAPIToken is a valid API Token used to create/manage check. If provided,
	// metric management is enabled.
	// Default: none
	//
	// hcl: telemetry { circonus_api_token = string }
	CirconusAPIToken string `json:"circonus_api_token,omitempty" mapstructure:"circonus_api_token"`

	// CirconusAPIURL is the base URL to use for contacting the Circonus API.
	// Default: "https://api.circonus.com/v2"
	//
	// hcl: telemetry { circonus_api_url = string }
	CirconusAPIURL string `json:"circonus_apiurl,omitempty" mapstructure:"circonus_apiurl"`

	// CirconusBrokerID is an explicit broker to use when creating a new check. The numeric portion
	// of broker._cid. If metric management is enabled and neither a Submission URL nor Check ID
	// is provided, an attempt will be made to search for an existing check using Instance ID and
	// Search Tag. If one is not found, a new HTTPTRAP check will be created.
	// Default: use Select Tag if provided, otherwise, a random Enterprise Broker associated
	// with the specified API token or the default Circonus Broker.
	// Default: none
	//
	// hcl: telemetry { circonus_broker_id = string }
	CirconusBrokerID string `json:"circonus_broker_id,omitempty" mapstructure:"circonus_broker_id"`

	// CirconusBrokerSelectTag is a special tag which will be used to select a broker when
	// a Broker ID is not provided. The best use of this is to as a hint for which broker
	// should be used based on *where* this particular instance is running.
	// (e.g. a specific geo location or datacenter, dc:sfo)
	// Default: none
	//
	// hcl: telemetry { circonus_broker_select_tag = string }
	CirconusBrokerSelectTag string `json:"circonus_broker_select_tag,omitempty" mapstructure:"circonus_broker_select_tag"`

	// CirconusCheckDisplayName is the name for the check which will be displayed in the Circonus UI.
	// Default: value of CirconusCheckInstanceID
	//
	// hcl: telemetry { circonus_check_display_name = string }
	CirconusCheckDisplayName string `json:"circonus_check_display_name,omitempty" mapstructure:"circonus_check_display_name"`

	// CirconusCheckForceMetricActivation will force enabling metrics, as they are encountered,
	// if the metric already exists and is NOT active. If check management is enabled, the default
	// behavior is to add new metrics as they are encountered. If the metric already exists in the
	// check, it will *NOT* be activated. This setting overrides that behavior.
	// Default: "false"
	//
	// hcl: telemetry { circonus_check_metrics_activation = (true|false)
	CirconusCheckForceMetricActivation string `json:"circonus_check_force_metric_activation,omitempty" mapstructure:"circonus_check_force_metric_activation"`

	// CirconusCheckID is the check id (not check bundle id) from a previously created
	// HTTPTRAP check. The numeric portion of the check._cid field.
	// Default: none
	//
	// hcl: telemetry { circonus_check_id = string }
	CirconusCheckID string `json:"circonus_check_id,omitempty" mapstructure:"circonus_check_id"`

	// CirconusCheckInstanceID serves to uniquely identify the metrics coming from this "instance".
	// It can be used to maintain metric continuity with transient or ephemeral instances as
	// they move around within an infrastructure.
	// Default: hostname:app
	//
	// hcl: telemetry { circonus_check_instance_id = string }
	CirconusCheckInstanceID string `json:"circonus_check_instance_id,omitempty" mapstructure:"circonus_check_instance_id"`

	// CirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:consul)
	//
	// hcl: telemetry { circonus_check_search_tag = string }
	CirconusCheckSearchTag string `json:"circonus_check_search_tag,omitempty" mapstructure:"circonus_check_search_tag"`

	// CirconusCheckSearchTag is a special tag which, when coupled with the instance id, helps to
	// narrow down the search results when neither a Submission URL or Check ID is provided.
	// Default: service:app (e.g. service:consul)
	//
	// hcl: telemetry { circonus_check_tags = string }
	CirconusCheckTags string `json:"circonus_check_tags,omitempty" mapstructure:"circonus_check_tags"`

	// CirconusSubmissionInterval is the interval at which metrics are submitted to Circonus.
	// Default: 10s
	//
	// hcl: telemetry { circonus_submission_interval = "duration" }
	CirconusSubmissionInterval string `json:"circonus_submission_interval,omitempty" mapstructure:"circonus_submission_interval"`

	// CirconusCheckSubmissionURL is the check.config.submission_url field from a
	// previously created HTTPTRAP check.
	// Default: none
	//
	// hcl: telemetry { circonus_submission_url = string }
	CirconusSubmissionURL string `json:"circonus_submission_url,omitempty" mapstructure:"circonus_submission_url"`

	// DisableHostname will disable hostname prefixing for all metrics.
	//
	// hcl: telemetry { disable_hostname = (true|false)
	DisableHostname bool `json:"disable_hostname,omitempty" mapstructure:"disable_hostname"`

	// DogStatsdAddr is the address of a dogstatsd instance. If provided,
	// metrics will be sent to that instance
	//
	// hcl: telemetry { dogstatsd_addr = string }
	DogstatsdAddr string `json:"dogstatsd_addr,omitempty" mapstructure:"dogstatsd_addr"`

	// DogStatsdTags are the global tags that should be sent with each packet to dogstatsd
	// It is a list of strings, where each string looks like "my_tag_name:my_tag_value"
	//
	// hcl: telemetry { dogstatsd_tags = []string }
	DogstatsdTags []string `json:"dogstatsd_tags,omitempty" mapstructure:"dogstatsd_tags"`

	// RetryFailedConfiguration retries transient errors when setting up sinks (e.g. network errors when connecting to telemetry backends).
	//
	// hcl: telemetry { retry_failed_connection = (true|false) }
	RetryFailedConfiguration bool `json:"retry_failed_connection,omitempty" mapstructure:"retry_failed_connection"`

	// FilterDefault is the default for whether to allow a metric that's not
	// covered by the filter.
	//
	// hcl: telemetry { filter_default = (true|false) }
	FilterDefault bool `json:"filter_default,omitempty" mapstructure:"filter_default"`

	// AllowedPrefixes is a list of filter rules to apply for allowing metrics
	// by prefix. Use the 'prefix_filter' option and prefix rules with '+' to be
	// included.
	//
	// hcl: telemetry { prefix_filter = []string{"+<expr>", "+<expr>", ...} }
	AllowedPrefixes []string `json:"allowed_prefixes,omitempty" mapstructure:"allowed_prefixes"`

	// BlockedPrefixes is a list of filter rules to apply for blocking metrics
	// by prefix. Use the 'prefix_filter' option and prefix rules with '-' to be
	// excluded.
	//
	// hcl: telemetry { prefix_filter = []string{"-<expr>", "-<expr>", ...} }
	BlockedPrefixes []string `json:"blocked_prefixes,omitempty" mapstructure:"blocked_prefixes"`

	// MetricsPrefix is the prefix used to write stats values to.
	// Default: "consul."
	//
	// hcl: telemetry { metrics_prefix = string }
	MetricsPrefix string `json:"metrics_prefix,omitempty" mapstructure:"metrics_prefix"`

	// StatsdAddr is the address of a statsd instance. If provided,
	// metrics will be sent to that instance.
	//
	// hcl: telemetry { statsd_address = string }
	StatsdAddr string `json:"statsd_address,omitempty" mapstructure:"statsd_address"`

	// StatsiteAddr is the address of a statsite instance. If provided,
	// metrics will be streamed to that instance.
	//
	// hcl: telemetry { statsite_address = string }
	StatsiteAddr string `json:"statsite_address,omitempty" mapstructure:"statsite_address"`

	// PrometheusOpts provides configuration for the PrometheusSink. Currently the only configuration
	// we acquire from hcl is the retention time. We also use definition slices that are set in agent setup
	// before being passed to InitTelemmetry.
	//
	// hcl: telemetry { prometheus_retention_time = "duration" }
	PrometheusOpts prometheus.PrometheusOpts
}

// MetricsHandler provides an http.Handler for displaying metrics.
type MetricsHandler interface {
	DisplayMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error)
	Stream(ctx context.Context, encoder metrics.Encoder)
}

type MetricsConfig struct {
	Handler  MetricsHandler
	mu       sync.Mutex
	cancelFn context.CancelFunc
}

func (cfg *MetricsConfig) Cancel() {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.cancelFn != nil {
		cfg.cancelFn()
	}
}

func statsiteSink(cfg TelemetryConfig, hostname string) (metrics.MetricSink, error) {
	addr := cfg.StatsiteAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(addr)
}

func statsdSink(cfg TelemetryConfig, hostname string) (metrics.MetricSink, error) {
	addr := cfg.StatsdAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(addr)
}

func dogstatdSink(cfg TelemetryConfig, hostname string) (metrics.MetricSink, error) {
	addr := cfg.DogstatsdAddr
	if addr == "" {
		return nil, nil
	}
	sink, err := datadog.NewDogStatsdSink(addr, hostname)
	if err != nil {
		return nil, err
	}
	sink.SetTags(cfg.DogstatsdTags)
	return sink, nil
}

func prometheusSink(cfg TelemetryConfig, _ string) (metrics.MetricSink, error) {

	if cfg.PrometheusOpts.Expiration.Nanoseconds() < 1 {
		return nil, nil
	}

	sink, err := prometheus.NewPrometheusSinkFrom(cfg.PrometheusOpts)
	if err != nil {
		// During testing we may try to register the same metrics collector
		// multiple times in a single run (e.g. a metrics test fails and
		// we attempt a retry), resulting in an AlreadyRegisteredError.
		// Suppress this and move on.
		if errors.As(err, &prometheuscore.AlreadyRegisteredError{}) {
			return nil, nil
		}
		return nil, err
	}
	return sink, nil
}

func circonusSink(cfg TelemetryConfig, _ string) (metrics.MetricSink, error) {
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

func configureSinks(cfg TelemetryConfig, memSink metrics.MetricSink, extraSinks []metrics.MetricSink) (metrics.FanoutSink, error) {
	metricsConf := metrics.DefaultConfig(cfg.MetricsPrefix)
	metricsConf.EnableHostname = !cfg.DisableHostname
	metricsConf.FilterDefault = cfg.FilterDefault
	metricsConf.AllowedPrefixes = cfg.AllowedPrefixes
	metricsConf.BlockedPrefixes = cfg.BlockedPrefixes

	var sinks metrics.FanoutSink
	var errors error
	addSink := func(fn func(TelemetryConfig, string) (metrics.MetricSink, error)) {
		s, err := fn(cfg, metricsConf.HostName)
		if err != nil {
			errors = multierror.Append(errors, err)
			return
		}
		if s != nil {
			sinks = append(sinks, s)
		}
	}

	addSink(statsiteSink)
	addSink(statsdSink)
	addSink(dogstatdSink)
	addSink(circonusSink)
	addSink(prometheusSink)
	for _, sink := range extraSinks {
		if sink != nil {
			sinks = append(sinks, sink)
		}
	}

	if len(sinks) > 0 {
		sinks = append(sinks, memSink)
		metrics.NewGlobal(metricsConf, sinks)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, memSink)
	}
	return sinks, errors
}

// InitTelemetry configures go-metrics based on map of telemetry config
// values as returned by Runtimecfg.Config().
// InitTelemetry retries configurating the sinks in case error is retriable
// and retry_failed_connection is set to true.
func InitTelemetry(cfg TelemetryConfig, logger hclog.Logger, extraSinks ...metrics.MetricSink) (*MetricsConfig, error) {
	if cfg.Disable {
		return nil, nil
	}

	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)

	metricsConfig := &MetricsConfig{
		Handler: memSink,
	}

	var cancel context.CancelFunc
	var ctx context.Context
	retryWithBackoff := func() {
		waiter := &retry.Waiter{
			MaxWait: 5 * time.Minute,
		}
		for {
			logger.Warn("retrying configure metric sinks", "retries", waiter.Failures())
			_, err := configureSinks(cfg, memSink, extraSinks)
			if err == nil {
				logger.Info("successfully configured metrics sinks")
				return
			}
			logger.Error("failed configure sinks", "error", multierror.Flatten(err))

			if err := waiter.Wait(ctx); err != nil {
				logger.Trace("stop retrying configure metrics sinks")
			}
		}
	}

	if _, errs := configureSinks(cfg, memSink, extraSinks); errs != nil {
		if isRetriableError(errs) && cfg.RetryFailedConfiguration {
			logger.Warn("failed configure sinks", "error", multierror.Flatten(errs))
			ctx, cancel = context.WithCancel(context.Background())

			metricsConfig.mu.Lock()
			metricsConfig.cancelFn = cancel
			metricsConfig.mu.Unlock()
			go retryWithBackoff()
		} else {
			return nil, errs
		}
	}
	return metricsConfig, nil
}

func isRetriableError(errs error) bool {
	var dnsError *net.DNSError
	if errors.As(errs, &dnsError) && dnsError.IsNotFound {
		return true
	}
	return false
}
