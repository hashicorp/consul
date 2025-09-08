# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

service {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {}
  }
}