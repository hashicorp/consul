#!/bin/bash

set -euo pipefail

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy fake-statsd"