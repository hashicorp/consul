# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

connect {
  enabled = true
  ca_config {
    private_key_type = "rsa"
    private_key_bits = 2048
  }
}