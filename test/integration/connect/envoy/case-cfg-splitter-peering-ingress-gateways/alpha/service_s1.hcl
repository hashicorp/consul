# Copyright IBM Corp. 2024, 2026
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
