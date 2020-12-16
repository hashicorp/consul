services {
  name = "s2"
  port = 8181
  connect {
    sidecar_service {
      proxy {
        config {
          protocol = "http"
          envoy_tracing_json = <<EOF
{
  "http": {
    "name": "envoy.zipkin",
    "config": {
      "collector_cluster": "zipkin",
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