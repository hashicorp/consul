# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

ARG CONSUL_IMAGE_VERSION=latest
FROM docker.mirror.hashicorp.services/hashicorp/consul:${CONSUL_IMAGE_VERSION}
RUN apk add --no-cache iptables bind-tools
ARG TARGETARCH
COPY linux_${TARGETARCH}/consul /bin/consul
