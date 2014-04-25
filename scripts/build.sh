#!/bin/bash
#
# This script builds the application from source.
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd $DIR

# Get the git commit
GIT_COMMIT=$(git rev-parse HEAD)
GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)

# If we're building on Windows, specify an extension
EXTENSION=""
if [ "$(go env GOOS)" = "windows" ]; then
    EXTENSION=".exe"
fi

# Install dependencies
echo "--> Installing dependencies to speed up builds..."
go get ./...

# Build!
echo "--> Building..."
go build \
    -ldflags "-X main.GitCommit ${GIT_COMMIT}${GIT_DIRTY}" \
    -v \
    -o bin/consul${EXTENSION}
cp bin/consul${EXTENSION} ${GOPATH%:*}/bin
