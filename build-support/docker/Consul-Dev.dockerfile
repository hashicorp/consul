# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ARG CONSUL_IMAGE_VERSION=latest
FROM hashicorp/consul:${CONSUL_IMAGE_VERSION}
RUN apk update && apk add iptables
COPY consul /bin/consul
