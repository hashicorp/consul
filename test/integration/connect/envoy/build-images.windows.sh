#!/usr/bin/env bash

readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

ENVOY_VERSION=${ENVOY_VERSION:-"1.22-latest"}
export ENVOY_VERSION

echo "Building Images"

# Pull Windows Nanoserver image
docker.exe pull mcr.microsoft.com/windows/nanoserver:1809
# Re tag Pulled image
docker.exe tag mcr.microsoft.com/windows/nanoserver:1809 "${HASHICORP_DOCKER_PROXY}/windows/nanoserver"

# Build Fortio Windows Image
docker.exe build -t "${HASHICORP_DOCKER_PROXY}/windows/fortio" -f Dockerfile-fortio-windows .

# Pull Envoy-Windows Image
docker.exe pull "envoyproxy/envoy-windows:v${ENVOY_VERSION}"
# Re tag Pulled image
docker.exe tag "envoyproxy/envoy-windows:v${ENVOY_VERSION}" "${HASHICORP_DOCKER_PROXY}/windows/envoy-windows:v${ENVOY_VERSION}"

# Build Socat-Windows Image
docker.exe build -t "${HASHICORP_DOCKER_PROXY}/windows/socat" -f Dockerfile-socat-windows .

echo "Building Complete!"
