#!/bin/bash

# There is no sidecar proxy for s2-v1, since the terminating gateway acts as the proxy
export REQUIRED_SERVICES="
s1 s1-sidecar-proxy
s2-v1
terminating-gateway-primary
"
