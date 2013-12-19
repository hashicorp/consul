#!/bin/bash
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that
cd $DIR

# Determine the version that we're building based on the contents
# of packer/version.go.
VERSION=$(grep "const Version " version.go | sed -E 's/.*"(.+)"$/\1/')
VERSIONDIR="${VERSION}"
PREVERSION=$(grep "const VersionPrerelease " version.go | sed -E 's/.*"(.*)"$/\1/')
if [ ! -z $PREVERSION ]; then
    PREVERSION="${PREVERSION}.$(date -u +%s)"
    VERSIONDIR="${VERSIONDIR}-${PREVERSION}"
fi

echo "Version: ${VERSION} ${PREVERSION}"

# Determine the arch/os combos we're building for
XC_ARCH=${XC_ARCH:-"386 amd64 arm"}
XC_OS=${XC_OS:-linux darwin windows freebsd openbsd}

echo "Arch: ${XC_ARCH}"
echo "OS: ${XC_OS}"

# Make sure that if we're killed, we kill all our subprocseses
trap "kill 0" SIGINT SIGTERM EXIT

# This function builds whatever directory we're in...
goxc \
    -arch="$XC_ARCH" \
    -os="$XC_OS" \
    -d="${DIR}/pkg" \
    -pv="${VERSION}" \
    -pr="${PREVERSION}" \
    $XC_OPTS \
    go-install \
    xc

# Zip all the packages
mkdir -p ./pkg/${VERSIONDIR}/dist
for PLATFORM in $(find ./pkg/${VERSIONDIR} -mindepth 1 -maxdepth 1 -type d); do
    PLATFORM_NAME=$(basename ${PLATFORM})
    ARCHIVE_NAME="${VERSIONDIR}_${PLATFORM_NAME}"

    if [ $PLATFORM_NAME = "dist" ]; then
        continue
    fi

    pushd ${PLATFORM}
    zip ${DIR}/pkg/${VERSIONDIR}/dist/${ARCHIVE_NAME}.zip ./*
    popd
done

# Make the checksums
pushd ./pkg/${VERSIONDIR}/dist
shasum -a256 * > ./${VERSIONDIR}_SHA256SUMS
popd

exit 0
