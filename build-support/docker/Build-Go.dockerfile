# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

ARG GOLANG_VERSION
FROM golang:${GOLANG_VERSION}

WORKDIR /consul
