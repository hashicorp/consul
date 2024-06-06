# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This manifest file describes active releases and is consumed by the backport tooling.
# It is only consumed from the default branch, so backporting changes to this file is not necessary.

schema = 1
active_versions {
  version "1.19" {
    ce_active = true
  }
  version "1.18" {
    # This release should remain active until 1.19 GA
    ce_active = true
    lts       = true
  }
  version "1.17" {}
  version "1.16" {}
  version "1.15" {
    lts = true
  }
}
