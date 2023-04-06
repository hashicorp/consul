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
            destination_peer = "primary-to-alpha"
            local_bind_port  = 5000
          }
        ]
      }
    }
  }
}
