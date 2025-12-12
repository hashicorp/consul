# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

service {
  name = "s1"
  port = 8080
  connect {
    sidecar_service {}
  }
}