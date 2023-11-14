# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s1"
  port = 8080
  checks = []
  connect {
    sidecar_service {
      checks = []
      proxy {
        upstreams = [
          {
            destination_name = "split-s2"
            local_bind_port  = 5000
          },
          {
            destination_name = "peer-s2"
            local_bind_port  = 5001
          }
        ]
      }
    }
  }
}
