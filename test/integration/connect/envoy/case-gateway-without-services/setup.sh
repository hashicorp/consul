#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap mesh-gateway 19000 primary true
