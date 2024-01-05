# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

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
