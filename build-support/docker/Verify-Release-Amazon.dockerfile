# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

FROM amazonlinux:latest
RUN yum install -y yum-utils shadow-utils
RUN yum-config-manager --add-repo https://rpm.releases.hashicorp.com/AmazonLinux/hashicorp.repo
ARG PACKAGE=consul \
ARG VERSION \
ARG SUFFIX=1
RUN yum install -y ${PACKAGE}-${VERSION}-${SUFFIX}
