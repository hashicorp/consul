# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# The inline virtual DNS listener and egress recursor DNS listener are gated
# behind the "localized-dns" feature gate (see agent/featuregate). Bootstrap
# it enabled here so the server's leader routine commits an initial
# FeatureGatePolicy with this feature turned on, letting sidecars in this
# datacenter pick up the gated xDS listener behavior.
feature_gates {
  bootstrap = { "localized-dns" = true }
}
