package envoy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"text/template"
)

// BootstrapConfig is the set of keys we care about in a Connect.Proxy.Config
// map. Note that this only includes config keys that affects Envoy bootstrap
// generation. For Envoy config keys that affect runtime xDS behavior see
// agent/xds/config.go.
type BootstrapConfig struct {
	// StatsdURL allows simple configuration of the statsd metrics sink. If
	// tagging is required, use DogstatsdURL instead. The URL must be in one of
	// the following forms:
	//   - udp://<ip>:<port>
	//   - $ENV_VAR_NAME        in this case the ENV var named will have it's
	//                          value taken and is expected to contain a URL in
	//									 				one of the supported forms above.
	StatsdURL string `mapstructure:"envoy_statsd_url"`

	// DogstatsdURL allows simple configuration of the dogstatsd metrics sink
	// which allows tags and Unix domain sockets. The URL must be in one of the
	// following forms:
	//   - udp://<ip>:<port>
	//   - unix:///full/path/to/unix.sock
	//   - $ENV_VAR_NAME        in this case the ENV var named will have it's
	//                          value taken and is expected to contain a URL in
	//									 				one of the supported forms above.
	DogstatsdURL string `mapstructure:"envoy_dogstatsd_url"`

	// StatsTags is a slice of string values that will be added as tags to
	// metrics. They are used to configure
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/metrics/v2/stats.proto#envoy-api-msg-config-metrics-v2-statsconfig
	// and add to the basic tags Consul adds by default like the local_cluster
	// name. Only exact values are supported here. Full configuration of
	// stats_config.stats_tags can be made by overriding envoy_stats_config_json.
	StatsTags []string `mapstructure:"envoy_stats_tags"`

	// PrometheusBindAddr configures an <ip>:<port> on which the Envoy will listen
	// and expose a single /metrics HTTP endpoint for Prometheus to scrape. It
	// does this by proxying that URL to the internal admin server's prometheus
	// endpoint which allows exposing metrics on the network without the security
	// risk of exposing the full admin server API. Any other URL requested will be
	// a 404.
	//
	// Note that as of Envoy 1.9.0, the built in Prometheus endpoint only exports
	// counters and gauges but not timing information via histograms. This is
	// fixed in 1.10-dev currently in Envoy master. Other changes since 1.9.0 make
	// master incompatible with the current release of Consul Connect. This will
	// be fixed in a future Consul version as Envoy 1.10 reaches stable release.
	PrometheusBindAddr string `mapstructure:"envoy_prometheus_bind_addr"`

	// OverrideJSONTpl allows replacing the base template used to render the
	// bootstrap. This is an "escape hatch" allowing arbitrary control over the
	// proxy's configuration but will the most effort to maintain and correctly
	// configure the aspects that Connect relies upon to work. It's recommended
	// that this only be used if necessary, and that it be based on the default
	// template in
	// https://github.com/hashicorp/consul/blob/master/command/connect/envoy/bootstrap_tpl.go
	// for the correct version of Consul and Envoy being used.
	OverrideJSONTpl string `mapstructure:"envoy_bootstrap_json_tpl"`

	// StaticClustersJSON is a JSON string containing zero or more Cluster
	// definitions. They are appended to the "static_resources.clusters" list. A
	// single cluster should be given as a plain object, if more than one is to be
	// added, they should be separated by a comma suitable for direct injection
	// into a JSON array.
	//
	// Note that cluster names should be chosen in such a way that they won't
	// collide with service names since we use plain service names as cluster
	// names in xDS to make metrics population simpler and cluster names mush be
	// unique.
	//
	// This is mostly intended for providing clusters for tracing or metrics
	// services.
	//
	// See https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/api/v2/cds.proto.
	StaticClustersJSON string `mapstructure:"envoy_extra_static_clusters_json"`

	// StaticListenersJSON is a JSON string containing zero or more Listener
	// definitions. They are appended to the "static_resources.listeners" list. A
	// single listener should be given as a plain object, if more than one is to
	// be added, they should be separated by a comma suitable for direct injection
	// into a JSON array.
	//
	// See https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/api/v2/lds.proto.
	StaticListenersJSON string `mapstructure:"envoy_extra_static_listeners_json"`

	// StatsSinksJSON is a JSON string containing zero or more StatsSink
	// definititions. They are appended to the `stats_sinks` array at the top
	// level of the bootstrap config. A single sink should be given as a plain
	// object, if more than one is to be added, they should be separated by a
	// comma suitable for direct injection into a JSON array.
	//
	// See
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/metrics/v2/stats.proto#config-metrics-v2-statssink.
	//
	// If this is non-empty then it will override anything configured in
	// StatsTags.
	StatsSinksJSON string `mapstructure:"envoy_extra_stats_sinks_json"`

	// StatsConfigJSON is a JSON string containing an object in the right format
	// to be rendered as the body of the `stats_config` field at the top level of
	// the bootstrap config. It's format may vary based on Envoy version used. See
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/metrics/v2/stats.proto#envoy-api-msg-config-metrics-v2-statsconfig.
	//
	// If this is non-empty then it will override anything configured in
	// StatsdURL or DogstatsdURL.
	StatsConfigJSON string `mapstructure:"envoy_stats_config_json"`

	// StatsFlushInterval is the time duration between Envoy stats flushes. It is
	// in proto3 "duration" string format for example "1.12s" See
	// https://developers.google.com/protocol-buffers/docs/proto3#json and
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/bootstrap/v2/bootstrap.proto#bootstrap
	StatsFlushInterval string `mapstructure:"envoy_stats_flush_interval"`

	// TracingConfigJSON is a JSON string containing an object in the right format
	// to be rendered as the body of the `tracing` field at the top level of
	// the bootstrap config. It's format may vary based on Envoy version used.
	// See https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/trace/v2/trace.proto.
	TracingConfigJSON string `mapstructure:"envoy_tracing_json"`
}

// Template returns the bootstrap template to use as a base.
func (c *BootstrapConfig) Template() string {
	if c.OverrideJSONTpl != "" {
		return c.OverrideJSONTpl
	}
	return bootstrapTemplate
}

func (c *BootstrapConfig) GenerateJSON(args *BootstrapTplArgs) ([]byte, error) {
	if err := c.ConfigureArgs(args); err != nil {
		return nil, err
	}
	t, err := template.New("bootstrap").Parse(c.Template())
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, args)
	if err != nil {
		return nil, err
	}

	// Pretty print the JSON.
	var buf2 bytes.Buffer
	if err := json.Indent(&buf2, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}

	return buf2.Bytes(), nil
}

// ConfigureArgs takes the basic template arguments generated from the command
// arguments and environment and modifies them according to the BootstrapConfig.
func (c *BootstrapConfig) ConfigureArgs(args *BootstrapTplArgs) error {

	// Attempt to setup sink(s) from high-level config. Note the args are passed
	// by ref and modified in place.
	if err := c.generateStatsSinks(args); err != nil {
		return err
	}

	if c.StatsConfigJSON != "" {
		// StatsConfig overridden explicitly
		args.StatsConfigJSON = c.StatsConfigJSON
	} else {
		// Attempt to setup tags from high-level config. Note the args are passed by
		// ref and modified in place.
		if err := c.generateStatsConfig(args); err != nil {
			return err
		}
	}

	if c.StaticClustersJSON != "" {
		args.StaticClustersJSON = c.StaticClustersJSON
	}
	if c.StaticListenersJSON != "" {
		args.StaticListenersJSON = c.StaticListenersJSON
	}
	// Setup prometheus if needed. This MUST happen after the Static*JSON is set above
	if c.PrometheusBindAddr != "" {
		if err := c.generatePrometheusConfig(args); err != nil {
			return err
		}
	}

	if c.TracingConfigJSON != "" {
		args.TracingConfigJSON = c.TracingConfigJSON
	}

	if c.StatsFlushInterval != "" {
		args.StatsFlushInterval = c.StatsFlushInterval
	}

	return nil
}

func (c *BootstrapConfig) generateStatsSinks(args *BootstrapTplArgs) error {
	var stats_sinks []string

	if c.StatsdURL != "" {
		sinkJSON, err := c.generateStatsSinkJSON("envoy.statsd", c.StatsdURL)
		if err != nil {
			return err
		}
		stats_sinks = append(stats_sinks, sinkJSON)
	}
	if c.DogstatsdURL != "" {
		sinkJSON, err := c.generateStatsSinkJSON("envoy.dog_statsd", c.DogstatsdURL)
		if err != nil {
			return err
		}
		stats_sinks = append(stats_sinks, sinkJSON)
	}
	if c.StatsSinksJSON != "" {
		stats_sinks = append(stats_sinks, c.StatsSinksJSON)
	}

	if len(stats_sinks) > 0 {
		args.StatsSinksJSON = "[\n" + strings.Join(stats_sinks, ",\n") + "\n]"
	}
	return nil
}

func (c *BootstrapConfig) generateStatsSinkJSON(name string, addr string) (string, error) {
	// Resolve address ENV var
	if len(addr) > 2 && addr[0] == '$' {
		addr = os.Getenv(addr[1:])
	}

	u, err := url.Parse(addr)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s sink address %q", name, addr)
	}

	var addrJSON string
	switch u.Scheme {
	case "udp":
		addrJSON = `
		"socket_address": {
			"address": "` + u.Hostname() + `",
			"port_value": ` + u.Port() + `
		}
		`
	case "unix":
		addrJSON = `
		"pipe": {
			"path": "` + u.Path + `"
		}
		`
	default:
		return "", fmt.Errorf("unsupported address protocol %q for %s sink",
			u.Scheme, name)
	}

	return `{
		"name": "` + name + `",
		"config": {
			"address": {
				` + addrJSON + `
			}
		}
	}`, nil
}

var sniTagJSONs []string

func init() {
	// <subset>.<service>.<namespace>.<datacenter>.<internal|external>.<trustdomain>.consul
	// - cluster.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
	// - cluster.f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
	// - cluster.v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
	// - cluster.f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
	const PART = `[^.]+`
	rules := [][]string{
		{"consul.custom_hash",
			fmt.Sprintf(`^cluster\.((?:(%s)~)?(?:%s\.)?%s\.%s\.%s\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.service_subset",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:(%s)\.)?%s\.%s\.%s\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.service",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?(%s)\.%s\.%s\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.namespace",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.(%s)\.%s\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.datacenter",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.(%s)\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.routing_type",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.%s\.(%s)\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)}, // internal:true/false would be idea
		{"consul.trust_domain",
			fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.%s\.%s\.(%s)\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.target",
			fmt.Sprintf(`^cluster\.(((?:%s~)?(?:%s\.)?%s\.%s\.%s)\.%s\.%s\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
		{"consul.full_target",
			fmt.Sprintf(`^cluster\.(((?:%s~)?(?:%s\.)?%s\.%s\.%s\.%s\.%s)\.consul\.)`, PART, PART, PART, PART, PART, PART, PART)},
	}

	for _, rule := range rules {
		m := map[string]string{
			"tag_name": rule[0],
			"regex":    rule[1],
		}
		d, err := json.Marshal(m)
		if err != nil {
			panic("error pregenerating SNI envoy tags: " + err.Error())
		}
		sniTagJSONs = append(sniTagJSONs, string(d))
	}
}

func (c *BootstrapConfig) generateStatsConfig(args *BootstrapTplArgs) error {
	var tagJSONs []string

	// Add some default tags if not already overridden
	defaults := map[string]string{
		"local_cluster": args.ProxyCluster,
	}

	// Explode SNI portions.
	tagJSONs = append(tagJSONs, sniTagJSONs...)

	for _, tag := range c.StatsTags {
		parts := strings.SplitN(tag, "=", 2)
		// If there is no equals, treat it as a boolean tag and just assign value of
		// 1 e.g. "canary" will out put the tag "canary: 1"
		v := "1"
		if len(parts) == 2 {
			v = parts[1]
		}
		k := strings.ToLower(parts[0])
		tagJSON := `{
			"tag_name": "` + k + `",
			"fixed_value": "` + v + `"
		}`
		tagJSONs = append(tagJSONs, tagJSON)
		// Remove this in case we override a default
		delete(defaults, k)
	}

	for k, v := range defaults {
		if v == "" {
			// Skip stuff we just didn't have data for, this is only really the case
			// in tests currently.
			continue
		}
		tagJSON := `{
			"tag_name": "` + k + `",
			"fixed_value": "` + v + `"
		}`
		tagJSONs = append(tagJSONs, tagJSON)
	}

	if len(tagJSONs) > 0 {
		// use_all_default_tags is true by default but we'll make it explicit!
		args.StatsConfigJSON = `{
			"stats_tags": [
				` + strings.Join(tagJSONs, ",\n") + `
			],
			"use_all_default_tags": true
		}`
	}
	return nil
}

func (c *BootstrapConfig) generatePrometheusConfig(args *BootstrapTplArgs) error {
	host, port, err := net.SplitHostPort(c.PrometheusBindAddr)
	if err != nil {
		return fmt.Errorf("invalid prometheus_bind_addr: %s", err)
	}

	clusterJSON := `{
		"name": "self_admin",
		"connect_timeout": "5s",
		"type": "STATIC",
		"http_protocol_options": {},
		"hosts": [
			{
				"socket_address": {
					"address": "127.0.0.1",
					"port_value": ` + args.AdminBindPort + `
				}
			}
		]
	}`
	listenerJSON := `{
		"name": "envoy_prometheus_metrics_listener",
		"address": {
			"socket_address": {
				"address": "` + host + `",
				"port_value": ` + port + `
			}
		},
		"filter_chains": [
			{
				"filters": [
					{
						"name": "envoy.http_connection_manager",
						"config": {
							"stat_prefix": "envoy_prometheus_metrics",
							"codec_type": "HTTP1",
							"route_config": {
								"name": "self_admin_route",
								"virtual_hosts": [
									{
										"name": "self_admin",
										"domains": [
											"*"
										],
										"routes": [
											{
												"match": {
													"path": "/metrics"
												},
												"route": {
													"cluster": "self_admin",
													"prefix_rewrite": "/stats/prometheus"
												}
											},
											{
												"match": {
													"prefix": "/"
												},
												"direct_response": {
													"status": 404
												}
											}
										]
									}
								]
							},
							"http_filters": [
								{
									"name": "envoy.router"
								}
							]
						}
					}
				]
			}
		]
	}`

	if args.StaticClustersJSON != "" {
		clusterJSON = ",\n" + clusterJSON
	}
	args.StaticClustersJSON += clusterJSON

	if args.StaticListenersJSON != "" {
		listenerJSON = ",\n" + listenerJSON
	}
	args.StaticListenersJSON += listenerJSON
	return nil
}
