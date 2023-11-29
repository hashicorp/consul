name: Check Copywrite Headers

on:
  pull_request:
    types: [opened, synchronize, reopened, ready_for_review]
  push:
    branches:
      - main
      - release/**

jobs:
  copywrite:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8ade135a41bc03ea155e62e844d188df1ea18608 # v4.1.0
      - uses: hashicorp/setup-copywrite@867a1a2a064a0626db322392806428f7dc59cb3e # v1.1.2
        name: Setup Copywrite
        with:
          version: v0.16.4
          archive-checksum: c299f830e6eef7e126a3c6ef99ac6f43a3c132d830c769e0d36fa347fa1af254
      - name: Check Header Compliance
        run: make copywrite-headers
permissions:
  contents: read
