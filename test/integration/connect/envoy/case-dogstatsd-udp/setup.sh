#!/bin/bash

set -euo pipefail

docker_consul connect envoy -bootstrap \
  -proxy-id s1-sidecar-proxy \
  > etc/envoy/s1-bootstrap.json

docker_consul connect envoy -bootstrap \
  -proxy-id s2-sidecar-proxy \
  -admin-bind 127.0.0.1:19001 \
  > etc/envoy/s2-bootstrap.json

export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy fake-statsd"