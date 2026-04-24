# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s3"
  port = 8282

  connect {
    sidecar_service {}
  }
}
