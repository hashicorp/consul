# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port  = 5000
            config {
              protocol = "http"
            }
          }
        ]
        config {
          protocol                         = "http"
          envoy_tracing_json               = <<EOF
{
  "http": {
    "name": "envoy.tracers.zipkin",
    "typedConfig": {
      "@type": "type.googleapis.com/envoy.config.trace.v3.ZipkinConfig",
      "collector_cluster": "zipkin",
      "collector_endpoint_version": "HTTP_JSON",
      "collector_endpoint": "/api/v1/spans",
      "shared_span_context": false
    }
  }
}
EOF
          envoy_extra_static_clusters_json = <<EOF2
{
  "name": "zipkin",
  "type": "STRICT_DNS",
  "connect_timeout": "5s",
  "load_assignment": {
    "cluster_name": "zipkin",
    "endpoints": [
      {
        "lb_endpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "127.0.0.1",
                  "port_value": 9411
                }
              }
            }
          }
        ]
      }
    ]
  }
}
EOF2
        }
      }
    }
  }
}
