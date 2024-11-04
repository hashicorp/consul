# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Configuration for security scanner.
# Run on PRs and pushes to `main` and `release/**` branches.
# See .github/workflows/security-scan.yml for CI config.

# To run manually, install scanner and then run `scan repository .`

# Scan results are triaged via the GitHub Security tab for this repo.
# See `security-scanner` docs for more information on how to add `triage` config
# for specific results or to exclude paths.

# .release/security-scan.hcl controls scanner config for release artifacts, which
# unlike the scans configured here, will block releases in CRT.

repository {
  go_modules              = true
  npm                     = true
  osv                     = true
  go_stdlib_version_file  = ".go-version"

  secrets {
    all = true
  }

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the scanner.
  triage {
    suppress {
      vulnerabilities = [
      ]
      paths = [
        "internal/tools/proto-gen-rpc-glue/e2e/consul/*",
        "test/integration/connect/envoy/test-sds-server/*",
        "test/integration/consul-container/*",
        "testing/deployer/*",
        "test-integ/*",
        "agent/uiserver/dist/assets/vendor-*.js",
      ]
    }
  }
}
