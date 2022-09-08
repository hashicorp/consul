#!/usr/bin/env bash

readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

ENVOY_VERSION=${ENVOY_VERSION:-"1.19.5"}
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

# TODO: Check if this image is required
# Pull Windows Nanoserver image
echo " "
echo "Pull Windows Nanoserver image"
docker pull mcr.microsoft.com/windows/nanoserver:1809
# Tag Windows Nanoserver image
echo " "
echo "Tag Windows Nanoserver image"
# docker tag mcr.microsoft.com/windows/nanoserver:1809 "${HASHICORP_DOCKER_PROXY}/windows/nanoserver"

# Pull Kubernetes/pause image
echo " "
echo "Pull Kubernetes/pause image"
docker pull mcr.microsoft.com/oss/kubernetes/pause:3.6
# Tag Kubernetes/pause image
echo " "
echo "Tag Kubernetes/pause image"
docker tag mcr.microsoft.com/oss/kubernetes/pause:3.6 "${HASHICORP_DOCKER_PROXY}/windows/kubernetes/pause"

# Build Windows Openzipkin Image
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/openzipkin" -f Dockerfile-openzipkin-windows .

# Build Windows Socat Image
echo " "
echo "Build Windows Socat Image"
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/socat" -f Dockerfile-socat-windows .

echo "Building Complete!"
