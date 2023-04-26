# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

FROM docker.mirror.hashicorp.services/circleci/node:16-browsers

USER root

RUN mkdir /consul-src
WORKDIR /consul-src
CMD make dist-docker
