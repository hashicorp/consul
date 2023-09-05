# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "mesh-gateway"
  kind = "mesh-gateway"
  port = 4431
  meta {
    consul-wan-federation = "1"
  }
}
