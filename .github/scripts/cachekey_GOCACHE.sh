#!/bin/bash

set -euo pipefail

echo "cachekey_GOCACHE=GOCACHEv0:${GOVERSION}:${GOMODULENAME}:${GOARCH}:${GOTAGS}" >> "$GITHUB_OUTPUT"