# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

ARG CONSUL_IMAGE_VERSION=latest
FROM hashicorp/consul:${CONSUL_IMAGE_VERSION}
RUN apk update && apk add iptables
ARG TARGETARCH
COPY linux_${TARGETARCH}/consul /bin/consul
