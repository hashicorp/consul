#!/bin/bash

set -euo pipefail

# Force rebuild of the exec container since this doesn't happen if only the
# version argument changed which means we end up testing the wrong version of
# Envoy.
docker-compose build s1-sidecar-proxy-consul-exec

# Bring up s1 and it's proxy as well because the check that it has a cert causes
# a proxy connection to be opened and having the backend not be available seems
# to cause Envoy to fail non-deterministically in CI (rarely on local machine).
# It might be related to this know issue
# https://github.com/envoyproxy/envoy/issues/2800 where TcpProxy will error if
# the backend is down sometimes part way through the handshake.
export REQUIRED_SERVICES="s1 s1-sidecar-proxy-consul-exec"
