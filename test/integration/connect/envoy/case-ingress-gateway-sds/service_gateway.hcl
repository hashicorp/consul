services {
  name = "ingress-gateway"
  kind = "ingress-gateway"

  proxy {
    config {
      envoy_extra_static_clusters_json = <<EOF
{
  "name": "sds-cluster",
  "connect_timeout": "5s",
  "typed_extension_protocol_options": {
    "envoy.extensions.upstreams.http.v3.HttpProtocolOptions": {
      "@type": "type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions",
      "explicit_http_config": {
        "http2_protocol_options": {}
      }
    }
  },
  "load_assignment": {
    "cluster_name": "sds-cluster",
    "endpoints": [
      {
        "lb_endpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "127.0.0.1",
                  "port_value": 1234
                }
              }
            }
          }
        ]
      }
    ]
  }
}
EOF
    }
  }
}
