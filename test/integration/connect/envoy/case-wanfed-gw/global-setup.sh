#!/bin/bash

# initialize the outputs for each dc
for dc in primary secondary; do
    rm -rf "workdir/${dc}/tls"
    mkdir -p "workdir/${dc}/tls"
done

readonly container="consul-envoy-integ-tls-init--${CASE_NAME}"

readonly scriptlet="
mkdir /out ;
cd /out ;
consul tls ca create ;
consul tls cert create -dc=primary -server -additional-dnsname='pri.server.primary.consul' ;
consul tls cert create -dc=secondary -server -additional-dnsname='sec.server.secondary.consul'
"

docker rm -f "$container" &>/dev/null || true
docker run -i --net=none --name="$container" consul-dev:latest sh -c "${scriptlet}"

# primary
for f in \
    consul-agent-ca.pem \
    primary-server-consul-0-key.pem \
    primary-server-consul-0.pem \
    ; do
    docker cp "${container}:/out/$f" workdir/primary/tls
done

# secondary
for f in \
    consul-agent-ca.pem \
    secondary-server-consul-0-key.pem \
    secondary-server-consul-0.pem \
    ; do
    docker cp "${container}:/out/$f" workdir/secondary/tls
done

docker rm -f "$container" >/dev/null || true
