#!/bin/sh

#
# Install dependencies that aren't available as Ubuntu packages.
#
# Everything goes into $HOME/local. 
#
# Scripts should add 
# - $HOME/local/bin to PATH 
# - $HOME/local/lib to LD_LIBRARY_PATH
#

cd
mkdir -p local

# Install swift
SWIFT_URL=https://swift.org/builds/swift-4.0-branch/ubuntu1404/swift-4.0-DEVELOPMENT-SNAPSHOT-2017-09-01-a/swift-4.0-DEVELOPMENT-SNAPSHOT-2017-09-01-a-ubuntu14.04.tar.gz
echo $SWIFT_URL
curl -fSsL $SWIFT_URL -o swift.tar.gz 
tar -xzf swift.tar.gz --strip-components=2 --directory=local

# Install protoc
PROTOC_URL=https://github.com/google/protobuf/releases/download/v3.4.0/protoc-3.4.0-linux-x86_64.zip
echo $PROTOC_URL
curl -fSsL $PROTOC_URL -o protoc.zip
unzip protoc.zip -d local

# Verify installation
find local
