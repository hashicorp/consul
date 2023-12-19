# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

services {
  name = "s4"

  // EDS cannot resolve localhost to an IP address
  address = "localhost"
  port = 8382
}