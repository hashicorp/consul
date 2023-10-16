#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

register_services primary

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
