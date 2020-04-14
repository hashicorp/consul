#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap terminating-gateway 19000 primary true
