// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package envoy

// BootstrapTplArgs is the set of arguments that may be interpolated into the
// Envoy bootstrap template.
type BootstrapTplArgs struct {
	GRPC

	// ProxyCluster is the cluster name for the Envoy `node` specification and
	// is typically the same as the ProxyID.
	ProxyCluster string

	// ProxyID is the ID of the proxy service instance as registered with the
	// local Consul agent. This must be used as the Envoy `node.id` in order for
	// the agent to deliver the correct configuration.
	ProxyID string

	// NodeName is the name of the node on which the proxy service instance is registered.
	NodeName string

	// ProxySourceService is the Consul service name to report for this proxy
	// instance's source service label. For sidecars it should be the
	// Proxy.DestinationServiceName. For gateways and similar it is the service
	// name of the proxy service itself.
	ProxySourceService string

	// AgentCAPEM is the CA to use to verify the local agent gRPC service if
	// TLS is enabled.
	AgentCAPEM string

	// AdminAccessLogConfig string representations of Envoy access log
	// configurations for the admin interface.
	AdminAccessLogConfig []string

	// AdminAccessLogPath The path to write the access log for the
	// administration server. If no access log is desired specify
	// "/dev/null". By default it will use "/dev/null". Will be overriden by
	// AdminAccessLogConfig.
	// DEPRECATED: use AdminAccessLogConfig
	AdminAccessLogPath string

	// AdminBindAddress is the address the Envoy admin server should bind to.
	AdminBindAddress string

	// AdminBindPort is the port the Envoy admin server should bind to.
	AdminBindPort string

	// LocalAgentClusterName is the name reserved for the local Consul agent gRPC
	// service and is expected to be used for that purpose.
	LocalAgentClusterName string

	// Token is the Consul ACL token provided which is required to make gRPC
	// discovery requests. If non-empty, this must be configured as the gRPC
	// service "initial_metadata" with the key "x-consul-token" in order to
	// authorize the discovery streaming RPCs.
	Token string

	// StaticClustersJSON is JSON string, each is expected to be a valid Cluster
	// definition. They are appended to the "static_resources.clusters" list. Note
	// that cluster names should be chosen in such a way that they won't collide
	// with service names since we use plain service names as cluster names in xDS
	// to make metrics population simpler and cluster names mush be unique. See
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/api/v2/cds.proto.
	StaticClustersJSON string

	// StaticListenersJSON is a JSON string containing zero or more Listener
	// definitions. They are appended to the "static_resources.listeners" list. A
	// single listener should be given as a plain object, if more than one is to
	// be added, they should be separated by a comma suitable for direct injection
	// into a JSON array.
	// See https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/api/v2/lds.proto.
	StaticListenersJSON string

	// StatsSinksJSON is a JSON string containing an array in the right format
	// to be rendered as the body of the `stats_sinks` field at the top level of
	// the bootstrap config. It's format may vary based on Envoy version used. See
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/metrics/v2/stats.proto#config-metrics-v2-statssink.
	StatsSinksJSON string

	// StatsConfigJSON is a JSON string containing an object in the right format
	// to be rendered as the body of the `stats_config` field at the top level of
	// the bootstrap config. It's format may vary based on Envoy version used. See
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/metrics/v2/stats.proto#envoy-api-msg-config-metrics-v2-statsconfig.
	StatsConfigJSON string

	// StaticSecretsJSON is a JSON string containing zero or more Secret definitions.
	// See https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/secret.proto#envoy-v3-api-msg-extensions-transport-sockets-tls-v3-secret
	StaticSecretsJSON string

	// StatsFlushInterval is the time duration between Envoy stats flushes. It is
	// in proto3 "duration" string format for example "1.12s" See
	// https://developers.google.com/protocol-buffers/docs/proto3#json and
	// https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/bootstrap/v2/bootstrap.proto#bootstrap
	StatsFlushInterval string

	// TracingConfigJSON is a JSON string containing an object in the right format
	// to be rendered as the body of the `tracing` field at the top level of
	// the bootstrap config. It's format may vary based on Envoy version used.
	// See https://www.envoyproxy.io/docs/envoy/v1.9.0/api-v2/config/trace/v2/trace.proto.
	TracingConfigJSON string

	// Namespace is the Consul Enterprise Namespace of the proxy service instance
	// as registered with the Consul agent.
	Namespace string

	// Partition is the Consul Enterprise Partition of the proxy service instance
	// as registered with the Consul agent.
	Partition string

	// Datacenter is the datacenter where the proxy service instance is registered.
	Datacenter string

	// PrometheusBackendPort will configure a "prometheus_backend" cluster which
	// envoy_prometheus_bind_addr will point to.
	PrometheusBackendPort string

	// PrometheusScrapePath will configure the path where metrics are exposed on
	// the envoy_prometheus_bind_addr listener.
	PrometheusScrapePath string

	// PrometheusCAFile is the path to a CA file for Envoy to use when serving TLS on the Prometheius metrics
	// endpoint. Only applicable when envoy_prometheus_bind_addr is set in the proxy config.
	PrometheusCAFile string

	// PrometheusCAPath is the path to a directory of CA certificates for Envoy to use when serving the Prometheus
	// metrics endpoint. Only applicable when envoy_prometheus_bind_addr is set in the proxy config.
	PrometheusCAPath string

	// PrometheusCertFile is the path to a certificate file for Envoy to use when serving TLS on the Prometheus
	// metrics endpoint. Only applicable when envoy_prometheus_bind_addr is set in the proxy config.
	PrometheusCertFile string

	// PrometheusKeyFile is the path to a private key file Envoy to use when serving TLS on the Prometheus metrics
	// endpoint. Only applicable when envoy_prometheus_bind_addr is set in the proxy config.
	PrometheusKeyFile string
}

// GRPC settings used in the bootstrap template.
type GRPC struct {
	// AgentAddress is the IP address of the local agent where the proxy instance
	// is registered.
	AgentAddress string

	// AgentPort is the gRPC port exposed on the local agent.
	AgentPort string

	// AgentTLS is true if the local agent gRPC service should be accessed over
	// TLS.
	AgentTLS bool

	// AgentSocket is the path to a Unix Socket for communicating with the
	// local agent's gRPC endpoint. Disabled if the empty (the default),
	// but overrides AgentAddress and AgentPort if set.
	AgentSocket string
}

// bootstrapTemplate sets '"ignore_health_on_host_removal": false' JUST to force this to be detected as a v3 bootstrap
// config.
const bootstrapTemplate = `{
  "admin": {
	{{- if (not .AdminAccessLogConfig) }}
    "access_log_path": "{{ .AdminAccessLogPath }}",
	{{- end}}
	{{- if .AdminAccessLogConfig }}
    "access_log": [
	{{- range $index, $element := .AdminAccessLogConfig}}
        {{if $index}},{{end}}
        {{$element}}
    {{end}}],
	{{- end}}
    "address": {
      "socket_address": {
        "address": "{{ .AdminBindAddress }}",
        "port_value": {{ .AdminBindPort }}
      }
    }
  },
  "node": {
    "cluster": "{{ .ProxyCluster }}",
    "id": "{{ .ProxyID }}",
    "metadata": {
      {{- if .NodeName }}
      "node_name": "{{ .NodeName }}",
      {{- end }}
      "namespace": "{{if ne .Namespace ""}}{{ .Namespace }}{{else}}default{{end}}",
      "partition": "{{if ne .Partition ""}}{{ .Partition }}{{else}}default{{end}}"
    }
  },
  "layered_runtime": {
    "layers": [
      {
        "name": "base",
        "static_layer": {
          "re2.max_program_size.error_level": 1048576
        }
      }
    ]
  },
  "static_resources": {
    "clusters": [
      {
        "name": "{{ .LocalAgentClusterName }}",
        "ignore_health_on_host_removal": false,
        "connect_timeout": "1s",
        "type": "STATIC",
        {{- if .AgentTLS -}}
        "transport_socket": {
          "name": "tls",
          "typed_config": {
            "@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
            "common_tls_context": {
              "validation_context": {
                "trusted_ca": {
                  "inline_string": "{{ .AgentCAPEM }}"
                }
              }
            }
          }
        },
        {{- end }}
        "http2_protocol_options": {},
        "loadAssignment": {
          "clusterName": "{{ .LocalAgentClusterName }}",
          "endpoints": [
            {
              "lbEndpoints": [
                {
                  "endpoint": {
                    "address": {
                      {{- if .AgentSocket -}}
                      "pipe": {
                        "path": "{{ .AgentSocket }}"
                      }
                      {{- else -}}
                      "socket_address": {
                        "address": "{{ .AgentAddress }}",
                        "port_value": {{ .AgentPort }}
                      }
                      {{- end -}}
                    }
                  }
                }
              ]
            }
          ]
        }
      }
      {{- if .StaticClustersJSON -}}
      ,
      {{ .StaticClustersJSON }}
      {{- end }}
    ]{{- if .StaticListenersJSON -}}
    ,
    "listeners": [
      {{ .StaticListenersJSON }}
    ]
    {{- end }}
    {{- if .StaticSecretsJSON -}}
    ,
    "secrets": [
      {{ .StaticSecretsJSON }}
    ]
    {{- end }}
  },
  {{- if .StatsSinksJSON }}
  "stats_sinks": [
    {{ .StatsSinksJSON }}
  ],
  {{- end }}
  {{- if .StatsConfigJSON }}
  "stats_config": {{ .StatsConfigJSON }},
  {{- end }}
  {{- if .StatsFlushInterval }}
  "stats_flush_interval": "{{ .StatsFlushInterval }}",
  {{- end }}
  {{- if .TracingConfigJSON }}
  "tracing": {{ .TracingConfigJSON }},
  {{- end }}
  "dynamic_resources": {
    "lds_config": {
      "ads": {},
      "initial_fetch_timeout": "0s",
      "resource_api_version": "V3"
    },
    "cds_config": {
      "ads": {},
      "initial_fetch_timeout": "0s",
      "resource_api_version": "V3"
    },
    "ads_config": {
      "api_type": "DELTA_GRPC",
      "transport_api_version": "V3",
      "grpc_services": {
        "initial_metadata": [
          {
            "key": "x-consul-token",
            "value": "{{ .Token }}"
          }
        ],
        "envoy_grpc": {
          "cluster_name": "{{ .LocalAgentClusterName }}"
        }
      }
    }
  }
}
`
