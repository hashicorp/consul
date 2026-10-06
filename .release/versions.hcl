# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# This manifest file describes active releases and is consumed by the backport tooling.
# It is only consumed from the default branch, so backporting changes to this file is not necessary.

schema = 1
active_versions {
  version "2.1" {
    ce_active = true
  }
  version "2.0" {
  }
  version "1.21" {
    lts       = true
  }
}
