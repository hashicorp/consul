#!/bin/bash
set -e

# Get the version from the command line
VERSION=$1
if [ -z $VERSION ]; then
    echo "Please specify a version."
    exit 1
fi

# Make sure we have a bintray API key
if [ -z $BINTRAY_API_KEY ]; then
    echo "Please set your bintray API key in the BINTRAY_API_KEY env var."
    exit 1
fi

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that
cd $DIR

# Zip all the files
rm -rf ./dist/pkg
mkdir -p ./dist/pkg
for FILENAME in $(find ./dist -mindepth 1 -maxdepth 1 -type f); do
    FILENAME=$(basename $FILENAME)
    EXTENSION="${FILENAME##*.}"
    PLATFORM="${FILENAME%.*}"

    if [ "${EXTENSION}" != "exe" ]; then
        EXTENSION=""
    else
        EXTENSION=".${EXTENSION}"
    fi

    CONSULNAME="consul${EXTENSION}"

    pushd ./dist

    if [ "${FILENAME}" = "ui.zip" ]; then
        cp ${FILENAME} ./pkg/${VERSION}_web_ui.zip
    else
        if [ "${EXTENSION}" = "" ]; then
            chmod +x ${FILENAME}
        fi

        cp ${FILENAME} ${CONSULNAME}
        zip ./pkg/${VERSION}_${PLATFORM}.zip ${CONSULNAME}
        rm ${CONSULNAME}
    fi

    popd
done

# Make the checksums
pushd ./dist/pkg
shasum -a256 * > ./${VERSION}_SHA256SUMS
popd

# Upload
for ARCHIVE in ./dist/pkg/*; do
    ARCHIVE_NAME=$(basename ${ARCHIVE})

    echo Uploading: $ARCHIVE_NAME
    curl \
        -T ${ARCHIVE} \
        -umitchellh:${BINTRAY_API_KEY} \
        "https://api.bintray.com/content/mitchellh/consul/consul/${VERSION}/${ARCHIVE_NAME}"
done

exit 0
