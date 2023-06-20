#!/usr/bin/env sh

set -ex

# HACK: UID of consul in the consul-client container
# This is conveniently also the UID of apt in the envoy container
CONSUL_UID=100
ENVOY_UID=$(id -u)

sudo consul connect redirect-traffic \
    -proxy-uid $ENVOY_UID \
    -exclude-uid $CONSUL_UID \
    $REDIRECT_TRAFFIC_ARGS

exec "$@"
