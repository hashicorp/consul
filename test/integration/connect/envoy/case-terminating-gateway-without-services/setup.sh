#!/bin/bash

set -eEuo pipefail

register_services primary

gen_envoy_bootstrap terminating-gateway 19000 primary true
