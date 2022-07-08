#!/usr/bin/env bash

readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

ENVOY_VERSION=${ENVOY_VERSION:-"1.22-latest"}
export ENVOY_VERSION

echo "Building Images"

# Pull Windows Nanoserver image
docker.exe pull mcr.microsoft.com/windows/nanoserver:1809
# Re tag Pulled image
docker.exe tag mcr.microsoft.com/windows/nanoserver:1809 "${HASHICORP_DOCKER_PROXY}/windows/nanoserver"

# Pull Envoy-Windows Image
docker.exe pull "envoyproxy/envoy-windows:v${ENVOY_VERSION}"
# Re tag Pulled image
docker.exe tag "envoyproxy/envoy-windows:v${ENVOY_VERSION}" "${HASHICORP_DOCKER_PROXY}/windows/envoy-windows:v${ENVOY_VERSION}"

# Build Fortio Windows Image
docker.exe build -t "${HASHICORP_DOCKER_PROXY}/windows/fortio" -f Dockerfile-fortio-windows .

# Build Socat-Windows Image
docker.exe build -t "${HASHICORP_DOCKER_PROXY}/windows/socat" -f Dockerfile-socat-windows .

# Build Bats-Core-Windows Image
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/bats:1.7.0" . -f Dockerfile-bats-core-windows

echo "Building Complete!"
