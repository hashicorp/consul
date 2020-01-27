#!/bin/bash

set -eEuo pipefail

# Force rebuild of the exec container since this doesn't happen if only the
# version argument changed which means we end up testing the wrong version of
# Envoy.
docker-compose build s1-sidecar-proxy-consul-exec consul-primary
