#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


export REQUIRED_SERVICES="
$DEFAULT_REQUIRED_SERVICES
s3 s3-sidecar-proxy
s3-v1 s3-v1-sidecar-proxy
s3-v2 s3-v2-sidecar-proxy
"
