#!/bin/bash

# Deploys CoreDNS to a cluster currently running Kube-DNS.

SERVICE_CIDR=$1
CLUSTER_DOMAIN=${2:-cluster.local}
YAML_TEMPLATE=${3:-`pwd`/coredns.yaml.sed}
YAML=${4:-`pwd`/coredns.yaml}

if [[ -z $SERVICE_CIDR ]]; then
	echo "Usage: $0 SERVICE-CIDR [ CLUSTER-DOMAIN ] [ YAML-TEMPLATE ] [ YAML ]"
	exit 1
fi

CLUSTER_DNS_IP=$(kubectl get service --namespace kube-system kube-dns -o jsonpath="{.spec.clusterIP}")

sed -e s/CLUSTER_DNS_IP/$CLUSTER_DNS_IP/g -e s/CLUSTER_DOMAIN/$CLUSTER_DOMAIN/g -e s?SERVICE_CIDR?$SERVICE_CIDR?g $YAML_TEMPLATE


