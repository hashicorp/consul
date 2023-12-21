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
	alpine_secdb = true

	secrets {
		all = true
	}

	# Triage items that are _safe_ to ignore here. Note that this list should be
	# periodically cleaned up to remove items that are no longer found by the scanner.
	triage {
		suppress {
			# N.b. `vulnerabilites` is the correct spelling for this tool.
			vulnerabilites = [
				"CVE-2023-46218", # curl@8.4.0-r0
				"CVE-2023-46219", # curl@8.4.0-r0
				"CVE-2023-5678",  # openssl@3.1.4-r0
			]
		}
	}
}

binary {
	go_modules   = true
	osv          = true
	# We can't enable npm for binary targets today because we don't yet embed the relevant file
	# (yarn.lock) in the Consul binary. This is something we may investigate in the future.
	
	secrets {
		all = true
	}
}
