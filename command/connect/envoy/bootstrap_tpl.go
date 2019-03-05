package envoy

type templateArgs struct {
	ProxyCluster, ProxyID string
	AgentAddress          string
	AgentPort             string
	AgentTLS              bool
	AgentCAFile           string
	AdminBindAddress      string
	AdminBindPort         string
	LocalAgentClusterName string
	Token                 string
}

const bootstrapTemplate = `{
  "admin": {
    "access_log_path": "/dev/null",
    "address": {
      "socket_address": {
        "address": "{{ .AdminBindAddress }}",
        "port_value": {{ .AdminBindPort }}
      }
    }
  },
  "node": {
    "cluster": "{{ .ProxyCluster }}",
    "id": "{{ .ProxyID }}"
  },
  "static_resources": {
    "clusters": [
      {
        "name": "{{ .LocalAgentClusterName }}",
        "connect_timeout": "1s",
        "type": "STATIC",
        {{- if .AgentTLS -}}
        "tls_context": {
          "common_tls_context": {
            "validation_context": {
              "trusted_ca": {
                "filename": "{{ .AgentCAFile }}"
              }
            }
          }
        },
        {{- end }}
        "http2_protocol_options": {},
        "hosts": [
          {
            "socket_address": {
              "address": "{{ .AgentAddress }}",
              "port_value": {{ .AgentPort }}
            }
          }
        ]
      }
    ]
  },
  "dynamic_resources": {
    "lds_config": { "ads": {} },
    "cds_config": { "ads": {} },
    "ads_config": {
      "api_type": "GRPC",
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
