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
                //curl
				"CVE-2025-14819",
				"CVE-2025-14524",
				"CVE-2025-14017",
				//gnupg
				"CVE-2025-30258",
				//ubi-rpmdb
				"CVE-2006-1174",
				"CVE-2015-3194",
				"CVE-2015-4000",
				"CVE-2015-7575",
				"CVE-2016-2177",
				"CVE-2016-8610",
				"CVE-2019-1551",
				"CVE-2021-3449",
				"CVE-2022-4203",
				"CVE-2022-4304",
				"CVE-2023-2975",
				"CVE-2024-12797",
				"CVE-2024-52533",
				"CVE-2025-31115",
				"CVE-2025-32414",
				"CVE-2025-3277",
				"CVE-2025-4598",
				"CVE-2025-47268",
				"CVE-2025-5702",
				"CVE-2025-6965",
				"CVE-2021-3712",
				"CVE-2016-7056",
				"CVE-2025-25724",
				"CVE-2014-3513",
				"CVE-2024-2511",
				"CVE-2024-4067",
				"CVE-2023-0286",
				"CVE-2024-6119",
				"CVE-2022-0778",
				"CVE-2025-11187",
				"CVE-2023-4641",
				"CVE-2021-23840",
				"CVE-2014-3505",
				"CVE-2014-3570",
				"CVE-2017-3736",
				"CVE-2025-13601",
				"CVE-2025-6395",
				"CVE-2007-4131",
				"CVE-2024-23337",
				"CVE-2025-9230",
				"CVE-2025-3576",
				"CVE-2025-45582",
				"CVE-2007-4476",
				"CVE-2025-6021",
				"CVE-2022-3358",
				"CVE-2025-5914",
				"CVE-2024-57970",
				"CVE-2024-40896",
				"CVE-2025-9086",
				"CVE-2026-0861",
				"CVE-2022-48303",
				"CVE-2025-68973",
				"CVE-2014-8176",
				"CVE-2015-0209",
				"CVE-2015-3197",
				"CVE-2021-43618",
				"CVE-2010-5298",
				"CVE-2026-24882",
				"CVE-2023-3446",
				"CVE-2018-0735",
				"CVE-2020-1971",
				"CVE-2017-3735",
				"CVE-2023-5363",
				"CVE-2025-9714",
				"CVE-2022-1292",
				"CVE-2023-0464",
				"CVE-2024-5535",
				"CVE-2024-56433",
				"CVE-2022-3602",
				"CVE-2019-1547",
				"CVE-2018-0734",
				"CVE-2016-0799",
				"CVE-2025-15467",
				"CVE-2025-8058",
				"CVE-2025-14104",
				"CVE-2025-5702",
				"CVE-2024-12797",
				"CVE-2015-7575",
				"CVE-2025-4598",
				"CVE-2025-31115",
				"CVE-2022-4304",
				"CVE-2016-2177",
				"CVE-2021-3449",
				"CVE-2015-3194",
				"CVE-2019-1551",
				"CVE-2023-2975",
				"CVE-2025-3277",
				"CVE-2025-15281"
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
			vulnerabilities = []
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
