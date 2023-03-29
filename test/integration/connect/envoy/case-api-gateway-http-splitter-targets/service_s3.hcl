# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  id   = "s3"
  name = "s3"
  port = 8182

  connect {
    sidecar_service {}
  }
}
