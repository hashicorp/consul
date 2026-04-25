# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

services {
  name = "s4"

  // EDS cannot resolve localhost to an IP address
  address = "localhost"
  port = 8382
}