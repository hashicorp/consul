# Note this arg has to be before the first FROM
ARG ENVOY_VERSION

FROM consul-dev as consul

FROM envoyproxy/envoy:v${ENVOY_VERSION}
COPY --from=consul /bin/consul /bin/consul
