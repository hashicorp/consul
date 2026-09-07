#!/bin/sh
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

cd /wasm
tinygo build -o /wasm/wasm_add_header.wasm -scheduler=none -target=wasi /wasm/wasm_add_header.go