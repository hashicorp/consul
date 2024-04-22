# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

ARG GOLANG_VERSION
FROM golang:${GOLANG_VERSION}-alpine3.19

WORKDIR /consul
