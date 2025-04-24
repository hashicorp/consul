# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "static-server"
  port = 8080
  connect {
    sidecar_service {}
  }
} 