# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
