# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This manifest file describes active releases and is consumed by the backport tooling.
# It is only consumed from the default branch, so backporting changes to this file is not necessary.

schema = 1
active_versions {
  version "1.20" {
    ce_active = true
  }
  version "1.19" {}
  version "1.18" {
    lts       = true
  }
  version "1.15" {
    lts = true
  }
}
