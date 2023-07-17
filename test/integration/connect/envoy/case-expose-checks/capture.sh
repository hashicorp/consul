#!/bin/bash

snapshot_envoy_admin localhost:19000 s1 || true
snapshot_envoy_admin localhost:19001 s2 || true
