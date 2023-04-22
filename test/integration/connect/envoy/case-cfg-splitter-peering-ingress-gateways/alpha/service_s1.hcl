# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

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
