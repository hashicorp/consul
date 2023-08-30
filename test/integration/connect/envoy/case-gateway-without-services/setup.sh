#!/bin/bash

set -eEuo pipefail

register_services primary

gen_envoy_bootstrap mesh-gateway 19000 primary true
