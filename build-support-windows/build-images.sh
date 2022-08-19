#!/usr/bin/env bash

readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

ENVOY_VERSION=${ENVOY_VERSION:-"1.22.1"}
export ENVOY_VERSION

echo "Building Images"

# Build Windows Consul Image
echo " "
echo "Build Windows Consul Image"
docker build -t "windows/consul" -f ../Dockerfile-windows ../

# Build Windows Consul-Dev Image
echo " "
echo "Build Windows Consul-Dev Image"
./Dockerfile-consul-dev-windows.sh

# Pull Windows Nanoserver image
echo " "
echo "Pull Windows Nanoserver image"
docker pull mcr.microsoft.com/windows/nanoserver:1809
# Tag Windows Nanoserver image
echo " "
echo "Tag Windows Nanoserver image"
docker tag mcr.microsoft.com/windows/nanoserver:1809 "${HASHICORP_DOCKER_PROXY}/windows/nanoserver"

# Pull Windows Envoy Image
echo " "
echo "Pull Windows Envoy Image"
docker pull "envoyproxy/envoy-windows:v${ENVOY_VERSION}"
# Tag Windows Envoy image
echo " "
echo "Tag Windows Envoy image"
docker tag "envoyproxy/envoy-windows:v${ENVOY_VERSION}" "${HASHICORP_DOCKER_PROXY}/windows/envoy-windows:v${ENVOY_VERSION}"

# Pull Kubernetes/pause image
echo " "
echo "Pull Kubernetes/pause image"
docker pull mcr.microsoft.com/oss/kubernetes/pause:3.6
# Tag Kubernetes/pause image
echo " "
echo "Tag Kubernetes/pause image"
docker tag mcr.microsoft.com/oss/kubernetes/pause:3.6 "${HASHICORP_DOCKER_PROXY}/windows/kubernetes/pause"

# Build Bats-Core-Windows Image
echo " "
echo "Build Bats-Core-Windows Image"
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/bats:1.7.0" -f Dockerfile-bats-core-windows .

# Build Windows Fortio Image
echo " "
echo "Build Windows Fortio Image"
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/fortio" -f Dockerfile-fortio-windows .

# Build Windows Jaegertracing Image
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/jaegertracing" -f Dockerfile-jaegertracing-windows .

# Build Windows Openzipkin Image
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/openzipkin" -f Dockerfile-openzipkin-windows .

# Build Windows Socat Image
echo " "
echo "Build Windows Socat Image"
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/socat" -f Dockerfile-socat-windows .

echo "Building Complete!"
