# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

ARG GOLANG_VERSION=1.20.3
FROM golang:${GOLANG_VERSION}

WORKDIR /consul
