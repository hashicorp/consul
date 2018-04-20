#!/usr/bin/env bash
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that.
cd $DIR

# Make sure build tools are available.
make tools

# Build the web assets.
echo "Building the V1 UI"
pushd ui
bundle
make dist
popd

echo "Building the V2 UI"
pushd ui-v2
yarn install
make dist
popd

# Make the static assets using the container version of the builder
make static-assets

exit 0
