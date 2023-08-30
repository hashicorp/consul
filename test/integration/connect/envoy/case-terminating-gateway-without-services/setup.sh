#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

register_services primary

gen_envoy_bootstrap terminating-gateway 19000 primary true
