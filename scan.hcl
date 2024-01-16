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
  go_modules   = true
  npm          = true
  osv          = true

  secrets {
    all = true
  }
}
