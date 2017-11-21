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

# Build the standalone version of the web assets for the sanity check.
pushd ui
bundle
make dist
popd

# Fixup the timestamps to match what's checked in. This will allow us to cleanly
# verify that the checked-in content is up to date without spurious diffs of the
# file mod times.
pushd pkg
cat ../agent/bindata_assetfs.go | ../scripts/fixup_times.sh
popd

# Regenerate the built-in web assets. If there are any diffs after doing this
# then we know something is up.
make static-assets
if ! git diff --quiet agent/bindata_assetfs.go; then
   echo "Checked-in web assets are out of date, build aborted"
   exit 1
fi

# Now we are ready to do a clean build of everything. We no longer distribute the
# web UI so it's ok that gets blown away as part of this.
rm -rf pkg
make all

exit 0
