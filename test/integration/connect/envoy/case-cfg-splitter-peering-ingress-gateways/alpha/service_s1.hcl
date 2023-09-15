# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s1"
  port = 8080
  checks = []
  connect {
    sidecar_service {
      checks = []
    }
  }
}
