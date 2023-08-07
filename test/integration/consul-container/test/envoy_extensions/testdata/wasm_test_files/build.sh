#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

cd /wasm
tinygo build -o /wasm/wasm_add_header.wasm -scheduler=none -target=wasi /wasm/wasm_add_header.go