# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  id   = "s3-v2"
  name = "s3"
  port = 8284

  meta {
    version = "v2"
  }

  connect {
    sidecar_service {}
  }
}
