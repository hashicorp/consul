ARG CONSUL_IMAGE_VERSION=latest
FROM consul:${CONSUL_IMAGE_VERSION}
RUN apk update && apk add iptables
COPY consul /bin/consul
