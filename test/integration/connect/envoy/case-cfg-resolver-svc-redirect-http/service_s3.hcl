# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s3"
  port = 8282
  connect { sidecar_service {} }
}