# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# This Dockerfile contains multiple targets.
# Use 'docker build --target=<name> .' to build one.
# e.g. `docker build --target=official .`
#
# All non-dev targets have a VERSION argument that must be provided 
# via --build-arg=VERSION=<version> when building. 
# e.g. --build-arg VERSION=1.11.2
#
# `default` is the production docker image which cannot be built locally. 
# For local dev and testing purposes, please build and use the `dev` docker image.


# Official docker image that includes binaries from releases.hashicorp.com. This
# downloads the release from releases.hashicorp.com and therefore requires that
# the release is published before building the Docker image.
FROM docker.mirror.hashicorp.services/alpine:3.18 as official

# This is the release of Consul to pull in.
ARG VERSION

LABEL org.opencontainers.image.authors="Consul Team <consul@hashicorp.com>" \
      org.opencontainers.image.url="https://www.consul.io/" \
      org.opencontainers.image.documentation="https://www.consul.io/docs" \
      org.opencontainers.image.source="https://github.com/hashicorp/consul" \
      org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.title="consul" \
      org.opencontainers.image.description="Consul is a datacenter runtime that provides service discovery, configuration, and orchestration." \
      version=${VERSION}

# This is the location of the releases.
ENV HASHICORP_RELEASES=https://releases.hashicorp.com

# Create a consul user and group first so the IDs get set the same way, even as
# the rest of this may change over time.
RUN addgroup consul && \
    adduser -S -G consul consul

# Set up certificates, base tools, and Consul.
# libc6-compat is needed to symlink the shared libraries for ARM builds
RUN set -eux && \
    apk add --no-cache ca-certificates curl dumb-init gnupg libcap openssl su-exec iputils jq libc6-compat iptables tzdata && \
    gpg --keyserver keyserver.ubuntu.com --recv-keys C874011F0AB405110D02105534365D9472D7468F && \
    mkdir -p /tmp/build && \
    cd /tmp/build && \
    apkArch="$(apk --print-arch)" && \
    case "${apkArch}" in \
        aarch64) consulArch='arm64' ;; \
        armhf) consulArch='arm' ;; \
        x86) consulArch='386' ;; \
        x86_64) consulArch='amd64' ;; \
        *) echo >&2 "error: unsupported architecture: ${apkArch} (see ${HASHICORP_RELEASES}/consul/${VERSION}/)" && exit 1 ;; \
    esac && \
    wget ${HASHICORP_RELEASES}/consul/${VERSION}/consul_${VERSION}_linux_${consulArch}.zip && \
    wget ${HASHICORP_RELEASES}/consul/${VERSION}/consul_${VERSION}_SHA256SUMS && \
    wget ${HASHICORP_RELEASES}/consul/${VERSION}/consul_${VERSION}_SHA256SUMS.sig && \
    gpg --batch --verify consul_${VERSION}_SHA256SUMS.sig consul_${VERSION}_SHA256SUMS && \
    grep consul_${VERSION}_linux_${consulArch}.zip consul_${VERSION}_SHA256SUMS | sha256sum -c && \
    unzip -d /tmp/build consul_${VERSION}_linux_${consulArch}.zip && \
    cp /tmp/build/consul /bin/consul && \
    if [ -f /tmp/build/EULA.txt ]; then mkdir -p /usr/share/doc/consul; mv /tmp/build/EULA.txt /usr/share/doc/consul/EULA.txt; fi && \
    if [ -f /tmp/build/TermsOfEvaluation.txt ]; then mkdir -p /usr/share/doc/consul; mv /tmp/build/TermsOfEvaluation.txt /usr/share/doc/consul/TermsOfEvaluation.txt; fi && \
    cd /tmp && \
    rm -rf /tmp/build && \
    gpgconf --kill all && \
    apk del gnupg openssl && \
    rm -rf /root/.gnupg && \
# tiny smoke test to ensure the binary we downloaded runs
    consul version

# The /consul/data dir is used by Consul to store state. The agent will be started
# with /consul/config as the configuration directory so you can add additional
# config files in that location.
RUN mkdir -p /consul/data && \
    mkdir -p /consul/config && \
    chown -R consul:consul /consul

# set up nsswitch.conf for Go's "netgo" implementation which is used by Consul,
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
ENTRYPOINT ["docker-entrypoint.sh"]

# By default you'll get an insecure single-node development server that stores
# everything in RAM, exposes a web UI and HTTP endpoints, and bootstraps itself.
# Don't use this configuration for production.
CMD ["agent", "-dev", "-client", "0.0.0.0"]


# Production docker image that uses CI built binaries.
# Remember, this image cannot be built locally.
FROM docker.mirror.hashicorp.services/alpine:3.18 as default

ARG PRODUCT_VERSION
ARG BIN_NAME

# PRODUCT_NAME and PRODUCT_VERSION are the name of the software on releases.hashicorp.com
# and the version to download. Example: PRODUCT_NAME=consul PRODUCT_VERSION=1.2.3.
ENV BIN_NAME=$BIN_NAME
ENV PRODUCT_VERSION=$PRODUCT_VERSION

ARG PRODUCT_REVISION
ARG PRODUCT_NAME=$BIN_NAME

# TARGETOS and TARGETARCH are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL org.opencontainers.image.authors="Consul Team <consul@hashicorp.com>" \
      org.opencontainers.image.url="https://www.consul.io/" \
      org.opencontainers.image.documentation="https://www.consul.io/docs" \
      org.opencontainers.image.source="https://github.com/hashicorp/consul" \
      org.opencontainers.image.version=${PRODUCT_VERSION} \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.title="consul" \
      org.opencontainers.image.description="Consul is a datacenter runtime that provides service discovery, configuration, and orchestration." \
      version=${PRODUCT_VERSION}

# Set up certificates and base tools.
# libc6-compat is needed to symlink the shared libraries for ARM builds
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


# Red Hat UBI-based image
# This target is used to build a Consul image for use on OpenShift.
FROM registry.access.redhat.com/ubi9-minimal:9.2 as ubi

ARG PRODUCT_NAME
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
ARG BIN_NAME

# PRODUCT_NAME and PRODUCT_VERSION are the name of the software on releases.hashicorp.com
# and the version to download. Example: PRODUCT_NAME=consul PRODUCT_VERSION=1.2.3.
ENV BIN_NAME=$BIN_NAME
ENV PRODUCT_VERSION=$PRODUCT_VERSION

ARG PRODUCT_NAME=$BIN_NAME

# TARGETOS and TARGETARCH are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL org.opencontainers.image.authors="Consul Team <consul@hashicorp.com>" \
      org.opencontainers.image.url="https://www.consul.io/" \
      org.opencontainers.image.documentation="https://www.consul.io/docs" \
      org.opencontainers.image.source="https://github.com/hashicorp/consul" \
      org.opencontainers.image.version=${PRODUCT_VERSION} \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.title="consul" \
      org.opencontainers.image.description="Consul is a datacenter runtime that provides service discovery, configuration, and orchestration." \
      version=${PRODUCT_VERSION}

# Copy license for Red Hat certification.
COPY LICENSE /licenses/mozilla.txt

# Set up certificates and base tools.
# dumb-init is downloaded directly from GitHub because there's no RPM package.
# Its shasum is hardcoded. If you upgrade the dumb-init verion you'll need to
# also update the shasum.
RUN set -eux && \
    microdnf install -y ca-certificates shadow-utils gnupg libcap openssl iputils jq iptables wget unzip tar && \
    wget -O /usr/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_x86_64 && \
    echo 'e874b55f3279ca41415d290c512a7ba9d08f98041b28ae7c2acb19a545f1c4df /usr/bin/dumb-init' > dumb-init-shasum && \
    sha256sum --check dumb-init-shasum && \
    chmod +x /usr/bin/dumb-init
 
# Create a non-root user to run the software. On OpenShift, this
# will not matter since the container is run as a random user and group
# but this is kept for consistency with our other images.
RUN groupadd $BIN_NAME && \
    adduser --uid 100 --system -g $BIN_NAME $BIN_NAME
COPY dist/$TARGETOS/$TARGETARCH/$BIN_NAME /bin/

# The /consul/data dir is used by Consul to store state. The agent will be started
# with /consul/config as the configuration directory so you can add additional
# config files in that location.
# In addition, change the group of the /consul directory to 0 since OpenShift
# will always execute the container with group 0.
RUN mkdir -p /consul/data && \
    mkdir -p /consul/config && \
    chown -R consul /consul && \
    chgrp -R 0 /consul && chmod -R g+rwX /consul

# set up nsswitch.conf for Go's "netgo" implementation which is used by Consul,
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

COPY .release/docker/docker-entrypoint-ubi.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
ENTRYPOINT ["docker-entrypoint.sh"]

# OpenShift by default will run containers with a random user, however their
# scanner requires that containers set a non-root user.
USER 100

# By default you'll get an insecure single-node development server that stores
# everything in RAM, exposes a web UI and HTTP endpoints, and bootstraps itself.
# Don't use this configuration for production.
CMD ["agent", "-dev", "-client", "0.0.0.0"]
