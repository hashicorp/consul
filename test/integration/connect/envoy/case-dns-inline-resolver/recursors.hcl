# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# Configuring recursors makes the xDS server emit the egress recursor DNS
# listener (dns_external_egress, 127.0.0.1:8654) that forwards non-Consul
# queries to these upstream resolvers via Envoy's c-ares resolver.
#
# These addresses are never dialled by this test (only their presence in the
# generated Envoy config is asserted), so unreachable public resolvers are fine.
recursors = ["8.8.8.8", "8.8.4.4"]
