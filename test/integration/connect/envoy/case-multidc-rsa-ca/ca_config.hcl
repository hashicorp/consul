# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

connect {
  enabled = true
  ca_config {
    private_key_type = "rsa"
    private_key_bits = 2048
  }
}