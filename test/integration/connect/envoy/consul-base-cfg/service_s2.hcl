# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "s2"
  port = 8181
  connect { sidecar_service {} }
}