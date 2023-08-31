# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "ingress-gateway"
  kind = "ingress-gateway"

  proxy {
    config {
      # Note that http2_protocol_options is a deprecated field and Envoy 1.17
      # and up would prefer:
      #     typed_extension_protocol_options:
      #       envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
      #         "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
      #         explicit_http_config:
      #           http2_protocol_options:
      #
      # But that breaks 1.15 and 1.16. For now use this which is supported by
      # all our supported versions to avoid needing to setup different
      # bootstrap based on the envoy version.
      envoy_extra_static_clusters_json = <<EOF
{
  "name": "sds-cluster",
  "connect_timeout": "5s",
  "http2_protocol_options": {},
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
