# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "api-gateway"
  port = 9999
  kind = "api-gateway"
  connect {
    sidecar_service {
      proxy {
        destinations = []
      }
    }
  }
} 