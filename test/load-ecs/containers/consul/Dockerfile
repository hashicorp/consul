ARG VERSION=local
ARG IMAGE=consul
FROM $IMAGE:$VERSION as consul-agent
COPY config/entrypoint.sh /usr/local/bin/
COPY config/auto-encrypt-client.hcl /consul/config/auto-encrypt.hcl
ENTRYPOINT ["entrypoint.sh"]

FROM consul-agent as consul-server
COPY config/tls.hcl /consul/config/tls.hcl
COPY config/auto-encrypt-ca.hcl /consul/config/auto-encrypt.hcl
COPY config/performance.hcl /consul/config/performance.hcl
COPY config/telemetry.hcl /consul/config/telemetry.hcl
