#!/bin/sh
cd /wasm
tinygo build -o /wasm/wasm_add_header.wasm -scheduler=none -target=wasi /wasm/wasm_add_header.go