# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# These scan results are run as part of CRT workflows.

# Un-triaged results will block release. See `security-scanner` docs for more
# information on how to add `triage` config to unblock releases for specific results.
# In most cases, we should not need to disable the entire scanner to unblock a release.

# To run manually, install scanner and then from the repository root run
# `SECURITY_SCANNER_CONFIG_FILE=.release/security-scan.hcl scan ...`
# To scan a local container, add `local_daemon = true` to the `container` block below.
# See `security-scanner` docs or run with `--help` for scan target syntax.

container {
  dependencies    = true
  osv             = true
  alpine_security = true
  go_modules      = true

  secrets {
    matchers {
      // Use most of default list, minus Vault (`hashicorp`), which has experienced false positives.
      // See https://github.com/hashicorp/security-scanner/blob/v0.0.2/pkg/scanner/secrets.go#L130C2-L130C2
      known = [
        // "hashicorp",
        "aws",
        "google",
        "slack",
        "github",
        "azure",
        "npm",
      ]
    }
  }

  # Triage items that are _safe_ to ignore here. Note that this list should be
  # periodically cleaned up to remove items that are no longer found by the scanner.
  triage {
    suppress {
      vulnerabilities = [
        "CVE-2025-14524", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2025-14017", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1 
        "CVE-2026-1965", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2026-3783", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2026-3784", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2026-3805", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2025-14819", //Alpine Linux's Security Issue Tracker in curl@8.17.0-r1
        "CVE-2025-30258", //Alpine Linux's Security Issue Tracker in gnupg@2.4.9-r0
        "CVE-2026-27171", //Alpine Linux's Security Issue Tracker in zlib@1.3.1-r2
        "CVE-2026-41989", //Alpine Linux's Security Issue Tracker in libgcrypt@1.11.2-r0
      ]

      paths = [
        "internal/tools/proto-gen-rpc-glue/e2e/consul/*",
        "test/integration/connect/envoy/test-sds-server/*",
        "test/integration/consul-container/*",
        "testing/deployer/*",
        "test-integ/*",
        // The OSV scanner will trip on several packages that are included in the
        // the UBI images. This is due to RHEL using the same base version in the
        // package name for the life of the distro regardless of whether or not
        // that version has been patched for security. Rather than enumate ever
        // single CVE that the OSV scanner will find (several tens) we'll ignore
        // the base UBI packages.
        "usr/lib/sysimage/rpm/*",
        "var/lib/rpm/*",
      ]
    }
  }
}

binary {
  go_modules = true
  osv        = true
  go_stdlib  = true

  # We can't enable npm for binary targets today because we don't yet embed the relevant file
  # (pnpm-lock.yaml) in the Consul binary. This is something we may investigate in the future.

  secrets {
    matchers {
      // Use most of default list, minus Vault (`hashicorp`), which has experienced false positives.
      // See https://github.com/hashicorp/security-scanner/blob/v0.0.2/pkg/scanner/secrets.go#L130C2-L130C2
      known = [
        // "hashicorp",
        "aws",

        "google",
        "slack",
        "github",
        "azure",
        "npm",
      ]
    }
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
      ]
    }
  }
}
