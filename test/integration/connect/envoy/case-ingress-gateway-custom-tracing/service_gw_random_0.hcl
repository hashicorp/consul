service {
  name = "ingress-gateway-random-sampling-0"
  kind = "ingress-gateway"
  proxy {
    config {
      envoy_tracing_json = <<-JSON
        {
          "http": {
            "name": "envoy.tracers.zipkin",
            "typedConfig": {
              "@type": "type.googleapis.com/envoy.config.trace.v2.ZipkinConfig",
              "collector_cluster": "zipkin",
              "collector_endpoint_version": "HTTP_JSON",
              "collector_endpoint": "/api/v1/spans",
              "shared_span_context": false
            }
          }
        }
      JSON
      envoy_extra_static_clusters_json = <<-JSON
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
      JSON
    }
  }
}