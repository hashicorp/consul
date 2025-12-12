# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

services {
  id   = "s3"
  name = "s3"
  port = 8182

  connect {
    sidecar_service {}
  }
}
