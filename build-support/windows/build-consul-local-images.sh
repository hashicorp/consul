#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

# Build Consul Version 1.13.3 / 1.12.6 / 1.11.11
VERSION=${VERSION:-"1.16.0"}
export VERSION

# Build Windows Envoy Version 1.23.1 / 1.21.5 / 1.20.7
ENVOY_VERSION=${ENVOY_VERSION:-"1.27.0"}
export ENVOY_VERSION

echo "Building Images"


# Pull Windows Servercore image
echo " "
echo "Pull Windows Servercore image"
docker pull mcr.microsoft.com/windows/servercore:1809
# Tag Windows Servercore image
echo " "
echo "Tag Windows Servercore image"
docker tag mcr.microsoft.com/windows/servercore:1809 "${HASHICORP_DOCKER_PROXY}/windows/servercore:1809"


# Pull Windows Nanoserver image
echo " "
echo "Pull Windows Nanoserver image"
docker pull mcr.microsoft.com/windows/nanoserver:1809
# Tag Windows Nanoserver image
echo " "
echo "Tag Windows Nanoserver image"
docker tag mcr.microsoft.com/windows/nanoserver:1809 "${HASHICORP_DOCKER_PROXY}/windows/nanoserver:1809"


# Pull Windows OpenJDK image
echo " "
echo "Pull Windows OpenJDK image"
docker pull openjdk:windowsservercore-1809
# Tag Windows OpenJDK image
echo " "
echo "Tag Windows OpenJDK image"
docker tag openjdk:windowsservercore-1809 "${HASHICORP_DOCKER_PROXY}/windows/openjdk:1809"

# Pull Windows Golang image
echo " "
echo "Pull Windows Golang image"
docker pull golang:1.18.1-nanoserver-1809
# Tag Windows Golang image
echo " "
echo "Tag Windows Golang image"
docker tag golang:1.18.1-nanoserver-1809 "${HASHICORP_DOCKER_PROXY}/windows/golang:1809"


# Pull Kubernetes/pause image
echo " "
echo "Pull Kubernetes/pause image"
docker pull mcr.microsoft.com/oss/kubernetes/pause:3.6
# Tag Kubernetes/pause image
echo " "
echo "Tag Kubernetes/pause image"
docker tag mcr.microsoft.com/oss/kubernetes/pause:3.6 "${HASHICORP_DOCKER_PROXY}/windows/kubernetes/pause"

# Pull envoy-windows image
echo " "
echo "Pull envoyproxy/envoy-windows image"
docker pull envoyproxy/envoy-windows:v${ENVOY_VERSION}
# Tag envoy-windows image
echo " "
echo "Tag envoyproxy/envoy-windows image"
docker tag envoyproxy/envoy-windows:v${ENVOY_VERSION} "${HASHICORP_DOCKER_PROXY}/windows/envoy-windows:v${ENVOY_VERSION}"

# Build Windows Openzipkin Image
docker build -t "${HASHICORP_DOCKER_PROXY}/windows/openzipkin" -f Dockerfile-openzipkin-windows .


# Build Windows Test sds server Image
./build-test-sds-server-image.sh


# Build windows/consul:${VERSION} Image
echo " "
echo "Build windows/consul:${VERSION} Image"
docker build -t "windows/consul:${VERSION}" -f ../../Dockerfile-windows ../../ --build-arg VERSION=${VERSION}


# Build windows/consul:${VERSION}-local Image
echo " "
echo "Build windows/consul:${VERSION}-local Image"
docker build -t windows/consul:${VERSION}-local -f ./Dockerfile-consul-local-windows . --build-arg VERSION=${VERSION}

echo "Building Complete!"
