# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "s2"
  port = 8181
  checks = []
  connect {
    sidecar_service {
      checks = []
    }
  }
}
