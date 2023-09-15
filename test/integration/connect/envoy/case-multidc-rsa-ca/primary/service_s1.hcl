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
            datacenter = "secondary"
            local_bind_port = 5000
          }
        ]
      }
    }
  }
}