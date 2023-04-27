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
              limits {
                max_connections = 3
                max_pending_requests = 4
                max_concurrent_requests = 5
              }
              passive_health_check {
                interval = "22s"
                max_failures = 4
                enforcing_consecutive_5xx = 99
                max_ejection_percent = 50
                base_ejection_time = "60s"
              }
            }
          }
        ]
      }
    }
  }
}
