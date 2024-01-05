# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s2"
  # Advertise gRPC port
  port = 8179
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