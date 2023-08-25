#!/bin/bash

set -eEuo pipefail

register_services primary

gen_envoy_bootstrap s1 19000 primary
