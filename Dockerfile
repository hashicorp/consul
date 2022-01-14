# This Dockerfile creates a production release image for the project using crt release flow.
FROM alpine:3.15.0 as default

ARG VERSION
ARG BIN_NAME

# PRODUCT_NAME and PRODUCT_VERSION are the name of the software in releases.hashicorp.com
# and the version to download. Example: PRODUCT_NAME=consul PRODUCT_VERSION=1.2.3.
ENV BIN_NAME=$BIN_NAME
ENV VERSION=$VERSION
#ARG CONSUL_VERSION=$VERSION
#ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
ARG PRODUCT_NAME=$BIN_NAME
# TARGETOS and TARGETARCH are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL org.opencontainers.image.authors="Consul Team <consul@hashicorp.com>" \
      org.opencontainers.image.url="https://www.consul.io/" \
      org.opencontainers.image.documentation="https://www.consul.io/docs" \
      org.opencontainers.image.source="https://github.com/hashicorp/consul" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.title="consul" \
      org.opencontainers.image.description="Consul is a datacenter runtime that provides service discovery, configuration, and orchestration."

# Set up certificates and base tools.
# libc6-compat is needed to symlink the shared libraries for ARM builds
RUN apk update
RUN apk add -v --no-cache \
		dumb-init \
		libc6-compat \
		iptables \
		tzdata \
		curl \
		ca-certificates \
		gnupg \
		iputils \ 
		libcap \
		openssl \
		su-exec \
		jq 

# Create a consul user and group first so the IDs get set the same way, even as
# the rest of this may change over time.
RUN addgroup $BIN_NAME && \
    adduser -S -G $BIN_NAME $BIN_NAME
COPY dist/$TARGETOS/$TARGETARCH/$BIN_NAME /bin/


RUN mkdir -p /consul/data && \
    mkdir -p /consul/config && \
    chown -R consul:consul /consul

# Set up nsswitch.conf for Go's "netgo" implementation which is used by Consul,
# otherwise DNS supercedes the container's hosts file, which we don't want.
RUN test -e /etc/nsswitch.conf || echo 'hosts: files dns' > /etc/nsswitch.conf

# Expose the consul data directory as a volume since there's mutable state in there.
VOLUME /consul/data

# Server RPC is used for communication between Consul clients and servers for internal
# request forwarding.
EXPOSE 8300

# Serf LAN and WAN (WAN is used only by Consul servers) are used for gossip between
# Consul agents. LAN is within the datacenter and WAN is between just the Consul
# servers in all datacenters.
EXPOSE 8301 8301/udp 8302 8302/udp

# HTTP and DNS (both TCP and UDP) are the primary interfaces that applications
# use to interact with Consul.
EXPOSE 8500 8600 8600/udp

# Consul doesn't need root privileges so we run it as the consul user from the
# entry point script. The entry point script also uses dumb-init as the top-level
# process to reap any zombie processes created by Consul sub-processes.

COPY .release/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT ["docker-entrypoint.sh"]

# By default you'll get an insecure single-node development server that stores
# everything in RAM, exposes a web UI and HTTP endpoints, and bootstraps itself.
# Don't use this configuration for production.
CMD ["agent", "-dev", "-client", "0.0.0.0"]
