package telemetry

import (
	"reflect"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
)

// Config is embedded in config.RuntimeConfig and holds the
// configuration variables for go-metrics. It is a separate struct to allow it
// to be exported as JSON and passed to other process like managed connect
// proxies so they can inherit the agent's telemetry config.
//
// It is in lib package rather than agent/config because we need to use it in
// the shared Init functions below, but we can't import agent/config
// due to a dependency cycle.
type Config struct {
	// Disable may be set to true to have Init to skip initialization
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

	// PrometheusRetentionTime is the retention time for prometheus metrics if greater than 0.
	// A value of 0 disable Prometheus support. Regarding Prometheus, it is considered a good
	// practice to put large values here (such as a few days), and at least the interval between
	// prometheus requests.
	//
	// hcl: telemetry { prometheus_retention_time = "duration" }
	PrometheusRetentionTime time.Duration `json:"prometheus_retention_time,omitempty" mapstructure:"prometheus_retention_time"`

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
}

// sinkFn takes Config and a hostname and builds a sink to be composed in the FanOutSink
type sinkFn func(Config, string) (metrics.MetricSink, error)

// ArmonMetricsClient provides a MetricsClient implementation for armon/go-metrics.
type ArmonMetricsClient struct {
	client    *metrics.Metrics
	inmemSink *metrics.InmemSink
}

var _ MetricsClient = &ArmonMetricsClient{}

func (c *ArmonMetricsClient) GetClient() interface{} {
	return c.client
}

func (c *ArmonMetricsClient) GetInmemSink() interface{} {
	return c.inmemSink
}

func (c *ArmonMetricsClient) SetGauge(key []string, val float32, labels ...Label) {
	if 0 < len(labels) {
		aLabels := convertLabelsToArmonMetricsLabels(labels)
		c.client.SetGaugeWithLabels(key, val, aLabels)
		return
	}
	c.client.SetGauge(key, val)
}

func (c *ArmonMetricsClient) EmitKV(key []string, val float32, labels ...Label) {
	if 0 < len(labels) {
		// FIXME(Kit): Ignore labels for now. It would be ideal to provide a warning, but lacking that, continuing as
		//  usual is fine. Other backends besides go-metrics may support labelled KVs
	}
	c.client.EmitKey(key, val)
}

func (c *ArmonMetricsClient) IncCounter(key []string, val float32, labels ...Label) {
	if 0 < len(labels) {
		aLabels := convertLabelsToArmonMetricsLabels(labels)
		c.client.IncrCounterWithLabels(key, val, aLabels)
		return
	}
	c.client.IncrCounter(key, val)
}

func (c *ArmonMetricsClient) AddSample(key []string, val float32, labels ...Label) {
	if 0 < len(labels) {
		aLabels := convertLabelsToArmonMetricsLabels(labels)
		c.client.AddSampleWithLabels(key, val, aLabels)
		return
	}
	c.client.AddSample(key, val)
}

func (c *ArmonMetricsClient) MeasureSince(key []string, start time.Time, labels ...Label) {
	if 0 < len(labels) {
		aLabels := convertLabelsToArmonMetricsLabels(labels)
		c.client.MeasureSinceWithLabels(key, start, aLabels)
		return
	}
	c.client.MeasureSince(key, start)
}

func convertLabelsToArmonMetricsLabels(labels []Label) []metrics.Label {
	aLabels := make([]metrics.Label, len(labels))
	for i := 0; i < len(labels); i++ {
		aLabels[i] = metrics.Label{
			Name:  labels[i].Key,
			Value: labels[i].Value,
		}
	}
	return aLabels
}

// MergeDefaults copies any non-zero field from defaults into the current
// config.
func (c *Config) MergeDefaults(defaults *Config) {
	if defaults == nil {
		return
	}
	cfgPtrVal := reflect.ValueOf(c)
	cfgVal := cfgPtrVal.Elem()
	otherVal := reflect.ValueOf(*defaults)
	for i := 0; i < cfgVal.NumField(); i++ {
		f := cfgVal.Field(i)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		// See if the current value is a zero-value, if _not_ skip it
		//
		// No built in way to check for zero-values for all types so only
		// implementing this for the types we actually have for now. Test failure
		// should catch the case where we add new types later.
		switch f.Kind() {
		case reflect.Slice:
			if !f.IsNil() {
				continue
			}
		case reflect.Int, reflect.Int64: // time.Duration == int64
			if f.Int() != 0 {
				continue
			}
		case reflect.String:
			if f.String() != "" {
				continue
			}
		case reflect.Bool:
			if f.Bool() {
				continue
			}
		default:
			// Needs implementing, should be caught by tests.
			continue
		}

		// It's zero, copy it from defaults
		f.Set(otherVal.Field(i))
	}
}

func statsiteSink(cfg Config, hostname string) (metrics.MetricSink, error) {
	addr := cfg.StatsiteAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(addr)
}

func statsdSink(cfg Config, hostname string) (metrics.MetricSink, error) {
	addr := cfg.StatsdAddr
	if addr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(addr)
}

func dogstatsdSink(cfg Config, hostname string) (metrics.MetricSink, error) {
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

func prometheusSink(cfg Config, hostname string) (metrics.MetricSink, error) {
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

func circonusSink(cfg Config, hostname string) (metrics.MetricSink, error) {
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

// addSink takes our collection of sinks and applies configuration to its setup function, appending the new sink to the
// collection.
func addSink(sinks metrics.FanoutSink, fn sinkFn, cfg Config, hostname string) error {
	// Build the sink
	s, err := fn(cfg, hostname)
	if err != nil {
		return err
	}
	// Compose into FanoutSink
	if s != nil {
		sinks = append(sinks, s)
	}
	return nil
}

// initSinks composes all of our sink options from RuntimeCfg into a FanoutSink
func initSinks(cfg Config, hostname string) (metrics.FanoutSink, error) {
	var sinks metrics.FanoutSink
	// Compose all of our external sinks with FanoutSink.
	// All sink inits must succeed - we abort setup if any of the configuration is invalid.
	if err := addSink(sinks, statsiteSink, cfg, hostname); err != nil {
		return sinks, err
	}
	if err := addSink(sinks, statsdSink, cfg, hostname); err != nil {
		return sinks, err
	}
	if err := addSink(sinks, dogstatsdSink, cfg, hostname); err != nil {
		return sinks, err
	}
	if err := addSink(sinks, circonusSink, cfg, hostname); err != nil {
		return sinks, err
	}
	if err := addSink(sinks, prometheusSink, cfg, hostname); err != nil {
		return sinks, err
	}
	return sinks, nil
}

// initArmonMetrics configures go-metrics based on map of telemetry config
// values as returned by RuntimeCfg.Config().
func initArmonMetrics(cfg Config) (MetricsClient, error) {
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

	sinks, err := initSinks(cfg, mCfg.HostName)
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
		return &ArmonMetricsClient{
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
	return &ArmonMetricsClient{client, memSink}, nil
}
