#!/bin/bash

export REQUIRED_SERVICES="$DEFAULT_REQUIRED_SERVICES ingress-gateway-primary test-sds-server"

if is_set $TEST_V2_XDS; then
  export SKIP_CASE="test SDS server doesn't support V2"
fi