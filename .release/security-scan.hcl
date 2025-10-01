# Copyright (c) HashiCorp, Inc.
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
	dependencies = true
	osv          = true

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
				"CVE-2024-52533",
				"CVE-2025-5914",
				"CVE-2025-32414",
				"CVE-2025-5702",
				"CVE-2024-40896",
				"CVE-2024-12797",
				"CVE-2024-57970",
				"CVE-2025-6021",
				"CVE-2025-31115",
				"CVE-2025-3277",
				"CVE-2024-4067",
				"CVE-2025-47268",
				"CVE-2025-6965",
				"CVE-2025-25724",
				"CVE-2025-3576",
				"CVE-2025-8058",
				"CVE-2024-23337",
				"CVE-2025-6395",
				"CVE-2025-46394"
				"CVE-2024-58251",
				"CVE-2025-48964",
				"CVE-2025-10148",
				"CVE-2025-9086",
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

binary {
	go_modules   = true
	osv          = true
	go_stdlib    = true
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
