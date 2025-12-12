#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

register_services primary

gen_envoy_bootstrap terminating-gateway 19000 primary true
