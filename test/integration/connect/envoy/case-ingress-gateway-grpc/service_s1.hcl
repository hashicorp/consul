# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s1"
  port = 8079
  connect {
    sidecar_service {
      proxy {
        config {
          protocol = "grpc"
        }
      }
    }
  }
}
