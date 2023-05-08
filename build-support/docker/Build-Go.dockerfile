# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

ARG GOLANG_VERSION=1.20.4
FROM golang:${GOLANG_VERSION}

WORKDIR /consul
