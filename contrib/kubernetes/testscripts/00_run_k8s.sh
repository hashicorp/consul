#!/bin/bash

set -e

# Based on instructions at: http://kubernetes.io/docs/getting-started-guides/docker/

#K8S_VERSION=$(curl -sS https://storage.googleapis.com/kubernetes-release/release/latest.txt)
K8S_VERSION="v1.2.4"

ARCH="amd64"

export K8S_VERSION
export ARCH

#RUN_SKYDNS="yes"
RUN_SKYDNS="no"

if [ "${RUN_SKYDNS}" = "yes" ]; then
	DNS_ARGUMENTS="--cluster-dns=10.0.0.10 --cluster-domain=cluster.local"
else
	DNS_ARGUMENTS=""
fi

echo "Starting kubernetes..."

docker run -d \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/var/lib/docker/:/var/lib/docker:rw \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged \
    gcr.io/google_containers/hyperkube-${ARCH}:${K8S_VERSION} \
    /hyperkube kubelet \
    --containerized \
    --hostname-override=127.0.0.1 \
    --api-servers=http://localhost:8080 \
    --config=/etc/kubernetes/manifests \
    ${DNS_ARGUMENTS} \
    --allow-privileged --v=2
