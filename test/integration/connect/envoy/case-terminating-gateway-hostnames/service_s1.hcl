# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s4"
            local_bind_port = 5000
          }
        ]
      }
    }
  }
}