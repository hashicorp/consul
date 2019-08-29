#!/bin/bash

export REQUIRED_SERVICES="
s1 s1-sidecar-proxy
s2 s2-sidecar-proxy
s2-secondary s2-sidecar-proxy-secondary
gateway-secondary
"
export REQUIRE_SECONDARY=1
