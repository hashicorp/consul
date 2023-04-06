# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "terminating-gateway"
  kind = "terminating-gateway"
  port = 8443

  meta {
    version = "v1"
  }
}
