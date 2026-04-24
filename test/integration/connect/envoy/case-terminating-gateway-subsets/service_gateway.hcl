# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "terminating-gateway"
  kind = "terminating-gateway"
  port = 8443

  meta {
    version = "v1"
  }
}
