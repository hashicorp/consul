# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "terminating-gateway"
  kind = "terminating-gateway"
  port = 8443

  meta {
    version = "v1"
  }
}
