#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1


# Default s1/s2 + sidecars are sufficient: s1 has an explicit upstream to the
# connect-enabled s2, so s2 gets a virtual IP that the inline virtual DNS
# listener advertises. Recursors for the egress DNS listener are supplied via
# recursors.hcl.
export REQUIRED_SERVICES="$DEFAULT_REQUIRED_SERVICES"
