// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package envoy

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/hashicorp/consul/api"
)

const (
	selfAdminName = "self_admin"
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

	// TelemetryCollectorBindSocketDir is a string that configures the directory for a
	// unix socket where Envoy will forward metrics. These metrics get pushed to
	// the telemetry collector.
	TelemetryCollectorBindSocketDir string `mapstructure:"envoy_telemetry_collector_bind_socket_dir"`

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

	// StatsBindAddr configures an <ip>:<port> on which the Envoy will listen
	// and expose the /stats HTTP path prefix for any agent to access. It
	// does this by proxying that path prefix to the internal admin server
	// which allows exposing metrics on the network without the security
	// risk of exposing the full admin server API. Any other URL requested will be
	// a 404.
	StatsBindAddr string `mapstructure:"envoy_stats_bind_addr"`

	// ReadyBindAddr configures an <ip>:<port> on which Envoy will listen and
	// expose a single /ready HTTP endpoint. This is useful for checking the
	// liveness of an Envoy instance when no other listeners are garaunteed to be
	// configured, as is the case with ingress gateways.
	//
	// Note that we do not allow this to be configured via the service
	// definition config map currently.
	ReadyBindAddr string `mapstructure:"-"`

	// OverrideJSONTpl allows replacing the base template used to render the
	// bootstrap. This is an "escape hatch" allowing arbitrary control over the
	// proxy's configuration but will the most effort to maintain and correctly
	// configure the aspects that Connect relies upon to work. It's recommended
	// that this only be used if necessary, and that it be based on the default
	// template in
	// https://github.com/hashicorp/consul/blob/main/command/connect/envoy/bootstrap_tpl.go
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

func (c *BootstrapConfig) GenerateJSON(args *BootstrapTplArgs, omitDeprecatedTags bool) ([]byte, error) {
	if err := c.ConfigureArgs(args, omitDeprecatedTags); err != nil {
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
func (c *BootstrapConfig) ConfigureArgs(args *BootstrapTplArgs, omitDeprecatedTags bool) error {

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
		stats, err := generateStatsTags(args, c.StatsTags, omitDeprecatedTags)
		if err != nil {
			return err
		}
		args.StatsConfigJSON = formatStatsTags(stats)
	}

	if c.StaticClustersJSON != "" {
		args.StaticClustersJSON = c.StaticClustersJSON
	}
	if c.StaticListenersJSON != "" {
		args.StaticListenersJSON = c.StaticListenersJSON
	}
	// Setup prometheus if needed. This MUST happen after the Static*JSON is set above
	if c.PrometheusBindAddr != "" {
		if err := c.generateListenerConfig(args, c.PrometheusBindAddr, "envoy_prometheus_metrics", "path", args.PrometheusScrapePath, "/stats/prometheus", args.PrometheusBackendPort); err != nil {
			return err
		}
	}
	// Setup /stats proxy listener if needed. This MUST happen after the Static*JSON is set above
	if c.StatsBindAddr != "" {
		if err := c.generateListenerConfig(args, c.StatsBindAddr, "envoy_metrics", "prefix", "/stats", "/stats", ""); err != nil {
			return err
		}
	}
	// Setup /ready proxy listener if needed. This MUST happen after the Static*JSON is set above
	if c.ReadyBindAddr != "" {
		if err := c.generateListenerConfig(args, c.ReadyBindAddr, "envoy_ready", "path", "/ready", "/ready", ""); err != nil {
			return err
		}
	}

	if c.TracingConfigJSON != "" {
		args.TracingConfigJSON = c.TracingConfigJSON
	}

	if c.StatsFlushInterval != "" {
		args.StatsFlushInterval = c.StatsFlushInterval
	}

	// Setup telemetry collector if needed. This MUST happen after the Static*JSON is set above
	if c.TelemetryCollectorBindSocketDir != "" {
		appendTelemetryCollectorConfig(args, c.TelemetryCollectorBindSocketDir)
	}

	return nil
}

func (c *BootstrapConfig) generateStatsSinks(args *BootstrapTplArgs) error {
	var stats_sinks []string

	if c.StatsdURL != "" {
		sinkJSON, err := c.generateStatsSinkJSON(
			"envoy.stat_sinks.statsd",
			"type.googleapis.com/envoy.config.metrics.v3.StatsdSink",
			c.StatsdURL,
		)
		if err != nil {
			return err
		}
		stats_sinks = append(stats_sinks, sinkJSON)
	}
	if c.DogstatsdURL != "" {
		sinkJSON, err := c.generateStatsSinkJSON(
			"envoy.stat_sinks.dog_statsd",
			"type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink",
			c.DogstatsdURL,
		)
		if err != nil {
			return err
		}
		stats_sinks = append(stats_sinks, sinkJSON)
	}
	if c.StatsSinksJSON != "" {
		stats_sinks = append(stats_sinks, c.StatsSinksJSON)
	}

	if len(stats_sinks) > 0 {
		args.StatsSinksJSON = strings.Join(stats_sinks, ",\n")
	}
	return nil
}

func (c *BootstrapConfig) generateStatsSinkJSON(name string, typeName string, addr string) (string, error) {
	// Resolve address ENV var
	if len(addr) > 2 && addr[0] == '$' {
		addr = os.Getenv(addr[1:])
	} else {
		addr = os.Expand(addr, statsSinkEnvMapping)
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
		"typedConfig": {
			"@type": "` + typeName + `",
			"address": {
				` + addrJSON + `
			}
		}
	}`, nil
}

func statsSinkEnvMapping(s string) string {
	allowedStatsSinkEnvVars := map[string]bool{
		"HOST_IP": true,
	}

	if !allowedStatsSinkEnvVars[s] {
		// if the specified env var isn't explicitly allowed, unexpand it
		return fmt.Sprintf("${%s}", s)
	}
	return os.Getenv(s)
}

// resourceTagSpecifiers returns patterns used to generate tags from cluster and filter metric names.
func resourceTagSpecifiers(omitDeprecatedTags bool) ([]string, error) {
	const (
		reSegment = `[^.]+`
	)

	// For all rules:
	//   - The outer capture group is removed from the final metric name.
	//   - The inner capture group is extracted into labels.
	rules := [][]string{
		// Cluster metrics are prefixed by consul.destination
		//
		// Cluster metric name format:
		// <subset>.<service>.<namespace>.<partition>.<datacenter|peering>.<internal|internal-<version>|external>.<trustdomain>.consul
		//
		// Examples:
		// (default partition)
		// - cluster.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.passthrough~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// (non-default partition)
		// - cluster.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.f8f8f8f8~pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// - cluster.passthrough~pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		// (peered)
		// - cluster.pong.default.cloudpeer.external.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors: 0
		{"consul.destination.custom_hash",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:(%s)~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.service_subset",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:(%s)\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.service",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?(%s)\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.namespace",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?%s\.(%s)\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.partition",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?%s\.%s\.(?:(%s)\.)?%s\.internal[^.]*\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.datacenter",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?(%s)\.internal[^.]*\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.peer",
			fmt.Sprintf(`^cluster\.(%s\.(?:%s\.)?(%s)\.external\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.routing_type",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.(%s)\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.trust_domain",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.(%s)\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.target",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?(((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s)\.%s\.%s\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		{"consul.destination.full_target",
			fmt.Sprintf(`^cluster\.(?:passthrough~)?(((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s)\.consul\.)`,
				reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

		// Upstream listener metrics are prefixed by consul.upstream
		//
		// Listener metric name format:
		// <tcp|http>.upstream.<service>.<namespace>.<partition>.<datacenter>
		//
		// Examples:
		// - tcp.upstream.db.dc1.downstream_cx_total: 0
		// - http.upstream.web.frontend.west.dc1.downstream_cx_total: 0
		//
		// Peered Listener metric name format:
		// <tcp|http>.upstream_peered.<service>.<namespace>.peer
		//
		// Examples:
		// - http.upstream_peered.web.frontend.cloudpeer.downstream_cx_total: 0
		{"consul.upstream.service",
			fmt.Sprintf(`^(?:tcp|http)\.upstream(?:_peered)?\.((%s)(?:\.%s)?(?:\.%s)?\.%s\.)`,
				reSegment, reSegment, reSegment, reSegment)},

		{"consul.upstream.datacenter",
			fmt.Sprintf(`^(?:tcp|http)\.upstream\.(%s(?:\.%s)?(?:\.%s)?\.(%s)\.)`,
				reSegment, reSegment, reSegment, reSegment)},

		{"consul.upstream.peer",
			fmt.Sprintf(`^(?:tcp|http)\.upstream_peered\.(%s(?:\.%s)?\.(%s)\.)`,
				reSegment, reSegment, reSegment)},

		{"consul.upstream.namespace",
			fmt.Sprintf(`^(?:tcp|http)\.upstream(?:_peered)?\.(%s(?:\.(%s))?(?:\.%s)?\.%s\.)`,
				reSegment, reSegment, reSegment, reSegment)},

		{"consul.upstream.partition",
			fmt.Sprintf(`^(?:tcp|http)\.upstream\.(%s(?:\.%s)?(?:\.(%s))?\.%s\.)`,
				reSegment, reSegment, reSegment, reSegment)},
	}

	// These tags were deprecated in Consul 1.9.0
	// We are leaving them enabled by default for backwards compatibility
	if !omitDeprecatedTags {
		deprecatedRules := [][]string{
			{"consul.custom_hash",
				fmt.Sprintf(`^cluster\.((?:(%s)~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.service_subset",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:(%s)\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.service",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?(%s)\.%s\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.namespace",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.(%s)\.(?:%s\.)?%s\.%s\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.datacenter",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?(%s)\.internal[^.]*\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.routing_type",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.(%s)\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.trust_domain",
				fmt.Sprintf(`^cluster\.((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.(%s)\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.target",
				fmt.Sprintf(`^cluster\.(((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s)\.%s\.%s\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},

			{"consul.full_target",
				fmt.Sprintf(`^cluster\.(((?:%s~)?(?:%s\.)?%s\.%s\.(?:%s\.)?%s\.%s\.%s)\.consul\.)`,
					reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment, reSegment)},
		}
		rules = append(rules, deprecatedRules...)
	}

	var tags []string
	for _, rule := range rules {
		m := map[string]string{
			"tag_name": rule[0],
			"regex":    rule[1],
		}
		d, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		tags = append(tags, string(d))
	}
	return tags, nil
}

func formatStatsTags(tags []string) string {
	var output string
	if len(tags) > 0 {
		// use_all_default_tags is true by default but we'll make it explicit!
		output = `{
			"stats_tags": [
				` + strings.Join(tags, ",\n") + `
			],
			"use_all_default_tags": true
		}`
	}
	return output
}

func generateStatsTags(args *BootstrapTplArgs, initialTags []string, omitDeprecatedTags bool) ([]string, error) {
	var (
		// Track tags we are setting explicitly to exclude them from defaults
		tagNames = make(map[string]struct{})
		tagJSONs []string
	)

	for _, tag := range initialTags {
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
		tagNames[k] = struct{}{}
	}

	// Explode listener and cluster portions.
	tags, err := resourceTagSpecifiers(omitDeprecatedTags)
	if err != nil {
		return nil, fmt.Errorf("failed to generate resource-specific envoy tags: %v", err)
	}
	tagJSONs = append(tagJSONs, tags...)

	// Default the namespace and partition here since it is also done for cluster SNI
	ns := args.Namespace
	if ns == "" {
		ns = api.IntentionDefaultNamespace
	}

	ap := args.Partition
	if ap == "" {
		ap = api.IntentionDefaultNamespace
	}

	// Add some default tags if not already overridden. Note this is a slice not a
	// map since we need ordering to be deterministic.
	defaults := []struct {
		name string
		val  string
	}{
		// local_cluster is for backwards compatibility. We originally choose this
		// name as it matched a few other Envoy metrics examples given in docs but
		// it's a little confusing in context of setting up metrics dashboards.
		{
			name: "local_cluster",
			val:  args.ProxyCluster,
		},
		{
			name: "consul.source.service",
			val:  args.ProxySourceService,
		},
		{
			name: "consul.source.namespace",
			val:  ns,
		},
		{
			name: "consul.source.partition",
			val:  ap,
		},
		{
			name: "consul.source.datacenter",
			val:  args.Datacenter,
		},
	}

	for _, kv := range defaults {
		if kv.val == "" {
			// Skip stuff we just didn't have data for.
			continue
		}
		if _, ok := tagNames[kv.name]; ok {
			// Skip anything already set explicitly.
			continue
		}
		tagJSON := `{
			"tag_name": "` + kv.name + `",
			"fixed_value": "` + kv.val + `"
		}`
		tagJSONs = append(tagJSONs, tagJSON)
	}
	return tagJSONs, nil
}

func (c *BootstrapConfig) generateListenerConfig(args *BootstrapTplArgs, bindAddr, name, matchType, matchValue, prefixRewrite, prometheusBackendPort string) error {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return fmt.Errorf("invalid %s bind address: %s", name, err)
	}

	// If prometheusBackendPort is set (not empty string), create
	// "prometheus_backend" cluster with the prometheusBackendPort that the
	// listener will point to, rather than the "self_admin" cluster. This is for
	// the merged metrics feature in consul-k8s, so the
	// envoy_prometheus_bind_addr listener will point to the merged Envoy and
	// service metrics endpoint rather than the Envoy admin endpoint for
	// metrics. This cluster will only be created once since it's only created
	// when prometheusBackendPort is set, and prometheusBackendPort is only set
	// when calling this function if c.PrometheusBindAddr is set.
	clusterAddress := args.AdminBindAddress
	clusterPort := args.AdminBindPort
	clusterName := selfAdminName
	if prometheusBackendPort != "" {
		clusterPort = prometheusBackendPort
		clusterName = "prometheus_backend"
	}

	clusterJSON := `{
		"name": "` + clusterName + `",
		"ignore_health_on_host_removal": false,
		"connect_timeout": "5s",
		"type": "STATIC",
		"http_protocol_options": {},
		"loadAssignment": {
			"clusterName": "` + clusterName + `",
			"endpoints": [
				{
					"lbEndpoints": [
						{
							"endpoint": {
								"address": {
									"socket_address": {
										"address": "` + clusterAddress + `",
										"port_value": ` + clusterPort + `
									}
								}
							}
						}
					]
				}
			]
		}
	}`

	// Enable TLS on the prometheus listener if cert/private key are provided.
	var tlsConfig string
	if args.PrometheusCertFile != "" {
		tlsConfig = `,
				"transportSocket": {
					"name": "tls",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext",
						"commonTlsContext": {
							"tlsCertificateSdsSecretConfigs": [
								{
									"name": "prometheus_cert"
								}
							],
							"validationContextSdsSecretConfig": {
								"name": "prometheus_validation_context"
							}
						}
					}
				}`
	}

	listenerJSON := `{
		"name": "` + name + `_listener",
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
						"name": "envoy.filters.network.http_connection_manager",
						 "typedConfig": {
							"@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
							"stat_prefix": "` + name + `",
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
													"` + matchType + `": "` + matchValue + `"
												},
												"route": {
													"cluster": "` + clusterName + `",
													"prefix_rewrite": "` + prefixRewrite + `"
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
									"name": "envoy.filters.http.router",
									"typedConfig": {
									"@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router"
									}
								}
							]
						}
					}
				]` + tlsConfig + `
			}
		]
	}`

	secretsTemplate := `{
		"name": "prometheus_cert",
		"tlsCertificate": {
			"certificateChain": {
				"filename": "%s"
			},
			"privateKey": {
				"filename": "%s"
			}
		}	
	},
	{
		"name": "prometheus_validation_context",
		"validationContext": {
			%s
		}
	}`
	var validationContext string
	if args.PrometheusCAPath != "" {
		validationContext = fmt.Sprintf(`"watchedDirectory": {
			"path": "%s"
		}`, args.PrometheusCAPath)
	} else {
		validationContext = fmt.Sprintf(`"trustedCa": {
			"filename": "%s"
		}`, args.PrometheusCAFile)
	}
	var secretsJSON string
	if args.PrometheusCertFile != "" {
		secretsJSON = fmt.Sprintf(secretsTemplate, args.PrometheusCertFile, args.PrometheusKeyFile, validationContext)
	}

	// Make sure we do not append the same cluster multiple times, as that will
	// cause envoy startup to fail.
	selfAdminClusterExists, err := containsSelfAdminCluster(args.StaticClustersJSON)
	if err != nil {
		return err
	}

	if args.StaticClustersJSON == "" {
		args.StaticClustersJSON = clusterJSON
	} else if !selfAdminClusterExists {
		args.StaticClustersJSON += ",\n" + clusterJSON
	}

	if args.StaticListenersJSON != "" {
		listenerJSON = ",\n" + listenerJSON
	}
	args.StaticListenersJSON += listenerJSON

	if args.StaticSecretsJSON != "" {
		secretsJSON = ",\n" + secretsJSON
	}
	args.StaticSecretsJSON += secretsJSON

	return nil
}

// appendTelemetryCollectorConfig generates config to enable a socket at path: <TelemetryCollectorBindSocketDir>/<hash of compound proxy ID>.sock
// We take the hash of the compound proxy ID for a few reasons:
//
//   - The proxy ID is included because this socket path must be unique per proxy. Each Envoy proxy will ship
//     its metrics to the collector using its own loopback listener at this path.
//
//   - The hash is needed because UNIX domain socket paths must be less than 104 characters. By using a b64 encoded
//     SHA1 hash we end up with 27 chars for the name, 5 chars for the extension, and the remainder is saved for
//     the configurable socket dir. The length of the directory's path is validated on writes to avoid going over.
func appendTelemetryCollectorConfig(args *BootstrapTplArgs, telemetryCollectorBindSocketDir string) {
	// Normalize namespace to "default". This ensures we match the namespace behaviour in proxycfg package,
	// where a dynamic listener will be created at the same socket path via xDS.
	ns := args.Namespace
	if ns == "" {
		ns = "default"
	}
	id := ns + "_" + args.ProxyID

	h := sha1.New()
	h.Write([]byte(id))
	hash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	path := path.Join(telemetryCollectorBindSocketDir, hash+".sock")

	if args.StatsSinksJSON != "" {
		args.StatsSinksJSON += ",\n"
	}
	args.StatsSinksJSON += `{
		"name": "envoy.stat_sinks.metrics_service",
		"typed_config": {
		  "@type": "type.googleapis.com/envoy.config.metrics.v3.MetricsServiceConfig",
		  "transport_api_version": "V3",
		  "grpc_service": {
			"envoy_grpc": {
			  "cluster_name": "consul_telemetry_collector_loopback"
			}
		  },
		  "emit_tags_as_labels": true
		}
	  }`

	if args.StaticClustersJSON != "" {
		args.StaticClustersJSON += ",\n"
	}
	args.StaticClustersJSON += fmt.Sprintf(`{
		"name": "consul_telemetry_collector_loopback",
		"type": "STATIC",
		"http2_protocol_options": {},
		"loadAssignment": {
		  "clusterName": "consul_telemetry_collector_loopback",
		  "endpoints": [
			{
			  "lbEndpoints": [
				{
				  "endpoint": {
					"address": {
					  "pipe": {
						"path": "%s"
					  }
					}
				  }
				}
			  ]
			}
		  ]
		}
	  }`, path)
}

func containsSelfAdminCluster(clustersJSON string) (bool, error) {
	clusterNames := []struct {
		Name string
	}{}

	// StaticClustersJSON is defined as a comma-separated list of clusters, so we
	// need to wrap it in JSON array brackets
	err := json.Unmarshal([]byte("["+clustersJSON+"]"), &clusterNames)
	if err != nil {
		return false, fmt.Errorf("failed to parse static clusters: %s", err)
	}

	for _, cluster := range clusterNames {
		if cluster.Name == selfAdminName {
			return true, nil
		}
	}

	return false, nil
}
