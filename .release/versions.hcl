# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# This file is solely for CE repos and exists in ENT repos to prevent merge conflicts

schema = 1
active_versions {
  version "1.18" {
    ce_active = true
    lts       = true
  }
  version "1.17" {}
  version "1.16" {}
  version "1.15" {
    lts = true
  }
}
