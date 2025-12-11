# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

FROM docker.mirror.hashicorp.services/node:18-alpine

USER root

RUN apk update && apk add make
RUN mkdir /consul-src
WORKDIR /consul-src
CMD make dist-docker
