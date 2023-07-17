#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

if [[ ! -f consul-agent-ca-key.pem ]] || [[ ! -f consul-agent-ca.pem ]]; then
    echo "Regenerating CA..."
    rm -f consul-agent-ca-key.pem consul-agent-ca.pem
    consul tls ca create
fi
rm -f rootca.crt rootca.key path/rootca.crt
cp consul-agent-ca.pem rootca.crt
cp consul-agent-ca-key.pem rootca.key
cp rootca.crt path

if [[ ! -f dc1-server-consul-0.pem ]] || [[ ! -f dc1-server-consul-0-key.pem ]]; then
    echo "Regenerating server..."
    rm -f dc1-server-consul-0.pem dc1-server-consul-0-key.pem
    consul tls cert create -server -node=server0 -additional-dnsname=consul.test
fi
rm -f server.crt server.key
cp dc1-server-consul-0.pem server.crt
cp dc1-server-consul-0-key.pem server.key

if [[ ! -f dc1-client-consul-0.pem ]] || [[ ! -f dc1-client-consul-0-key.pem ]]; then
    echo "Regenerating client..."
    rm -f dc1-client-consul-0.pem dc1-client-consul-0-key.pem
    consul tls cert create -client
fi
rm -f client.crt client.key
cp dc1-client-consul-0.pem client.crt
cp dc1-client-consul-0-key.pem client.key
