#!/bin/bash

# initialize the outputs for each dc
for dc in primary secondary; do
    rm -rf "workdir/${dc}/tls"
    mkdir -p "workdir/${dc}/tls"
done

container="consul-envoy-integ-tls-init--${CASE_NAME}"

scriptlet="
mkdir /out ;
cd /out ;
consul tls ca create ;
consul tls cert create -dc=primary -server -node=pri ;
consul tls cert create -dc=secondary -server -node=sec
"

docker rm -f "$container" &>/dev/null || true
docker run -i --net=none --name="$container" consul:local sh -c "${scriptlet}"

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

# Private keys have 600 perms but tests are run as another user
chmod 666 workdir/primary/tls/primary-server-consul-0-key.pem
chmod 666 workdir/secondary/tls/secondary-server-consul-0-key.pem

docker rm -f "$container" >/dev/null || true
