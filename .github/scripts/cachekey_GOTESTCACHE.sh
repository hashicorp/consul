#!/bin/bash

set -euo pipefail

# Note: while GOMODCACHE and GOCACHE are named after their respective Go environment variables, 
# GOTESTCACHE is not a real Go env var

# test flags are those built-in go test binary flags, like `-test.v`. test args on the other hand
# are those consumed by the binary itself
# kind:go-version:GOARCH:GOPACKAGENAME?:GOTAGS?:GOBUILDFLAGS?:GOTESTFLAGS?:GOTESTARGS?
echo "GOTESTCACHEv0:${GOVERSION}:${GOARCH}:${GOPACKAGENAME:-}:${GOTAGS:-}:${GOBUILDFLAGS:-}:${GOTESTFLAGS:-}:${GOTESTARGS:-}"