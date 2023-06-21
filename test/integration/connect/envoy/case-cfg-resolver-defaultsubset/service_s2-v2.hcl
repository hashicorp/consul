# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  id   = "s2-v2"
  name = "s2"
  port = 8183

  meta {
    version = "v2"
  }

  connect {
    sidecar_service {}
  }
}
