# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

FROM fedora:36
RUN dnf install -y dnf-plugins-core
RUN dnf config-manager --add-repo https://rpm.releases.hashicorp.com/fedora/hashicorp.repo
ARG PACKAGE=consul \
ARG VERSION \
ARG SUFFIX=1
RUN dnf install -y ${PACKAGE}-${VERSION}-${SUFFIX}