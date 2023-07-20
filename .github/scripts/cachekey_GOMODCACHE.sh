#!/bin/bash

set -euo pipefail

# TODO: do we care about go version? I don't think so?
echo "cachekey_GOMODCACHE=GOMODCACHEv0:${GOMODULENAME}" >> "$GITHUB_OUTPUT"