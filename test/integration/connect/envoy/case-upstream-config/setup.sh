#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
