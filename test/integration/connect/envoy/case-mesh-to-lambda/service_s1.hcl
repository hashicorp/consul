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
            destination_name = "l1"
            local_bind_port = 1234
          },
          {
            destination_name = "l2"
            local_bind_port = 5678
          }
        ]
      }
    }
  }
}
