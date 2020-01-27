#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
