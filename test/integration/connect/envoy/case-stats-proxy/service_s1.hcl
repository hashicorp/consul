# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port = 5000
            config {
              protocol = "http"
            }
          }
        ]
        config {
          protocol = "http"
          envoy_stats_bind_addr = "0.0.0.0:1239"
        }
      }
    }
  }
}
