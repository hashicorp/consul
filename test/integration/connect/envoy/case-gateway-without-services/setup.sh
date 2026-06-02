#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

register_services primary

gen_envoy_bootstrap mesh-gateway 19000 primary true
