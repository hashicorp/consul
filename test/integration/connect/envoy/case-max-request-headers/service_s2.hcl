# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s2"
  port = 8181
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s1"
            local_bind_port = 5000
          }
        ]
        config {
          protocol = "http"
          max_request_headers_kb = 96
        }
      }
    }
  }
}