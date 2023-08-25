# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  id = "s3"
  name = "s3"
  port = 8184
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "s2"
            local_bind_port = 8185
          }
        ]
      }
    }
  }
}
