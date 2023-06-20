# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  id   = "s2-v1"
  name = "s2"
  port = 8182

  meta {
    version = "v1"
  }

  checks = [
    {
      name = "main"
      ttl  = "30m"
    },
  ]

  connect {
    sidecar_service {
      port = 21011
    }
  }
}
