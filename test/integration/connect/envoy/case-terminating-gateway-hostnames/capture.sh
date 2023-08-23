#!/bin/bash

snapshot_envoy_admin localhost:20000 terminating-gateway primary || true
snapshot_envoy_admin localhost:19000 s1 primary || true
