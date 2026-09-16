#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

# The default s1 registration declares an explicit upstream to s2, and s2 is a
# connect-enabled service, so the server assigns s2 a virtual IP. That VIP is
# what the inline virtual DNS listener (127.0.0.1:8653) advertises for s2's
# expanded FQDN. Recursors (see recursors.hcl) drive the egress DNS listener.
register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
