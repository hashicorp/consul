#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

upsert_config_entry primary '
kind = "mesh"
tls {
  incoming {
    tls_min_version = "TLSv1_3"
  }
  outgoing {
    tls_min_version = "TLSv1_3"
    ecdh_curves     = ["P-256"]
  }
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
