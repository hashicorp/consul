#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0
#
# Apply runtime patches to consul-ui-testing so it works on GitHub Actions CI.
# Must be run from within the consul-ui-testing checkout directory.

set -euo pipefail

replace_all() {
    local file_path="$1"
    local old_text="$2"
    local new_text="$3"

    python3 - "$file_path" "$old_text" "$new_text" <<'EOF'
from pathlib import Path
import sys

file_path, old_text, new_text = sys.argv[1:4]
path = Path(file_path)
content = path.read_text()

if old_text not in content:
        print(f"ERROR: expected text not found in {file_path}", file=sys.stderr)
        sys.exit(1)

path.write_text(content.replace(old_text, new_text))
EOF
}

replace_regex_once() {
    local file_path="$1"
    local pattern="$2"
    local replacement="$3"

    python3 - "$file_path" "$pattern" "$replacement" <<'EOF'
from pathlib import Path
import re
import sys

file_path, pattern, replacement = sys.argv[1:4]
path = Path(file_path)
content = path.read_text()
updated, count = re.subn(pattern, replacement, content, count=1, flags=re.MULTILINE)

if count != 1:
        print(f"ERROR: expected one structural match in {file_path}, got {count}", file=sys.stderr)
        sys.exit(1)

path.write_text(updated)
EOF
}

replace_all_dockerfiles() {
  python3 <<'EOF'
from pathlib import Path
import sys

matches = list(Path('.').rglob('Dockerfile*'))
if not matches:
    print('ERROR: no Dockerfile* files found', file=sys.stderr)
    sys.exit(1)

updated_files = 0
for path in matches:
    content = path.read_text()
    if 'linux/arm64' in content:
        path.write_text(content.replace('linux/arm64', 'linux/amd64'))
        updated_files += 1

if updated_files == 0:
    print('ERROR: linux/arm64 not found in any Dockerfile*', file=sys.stderr)
    sys.exit(1)
EOF
}

# Fix architecture: GitHub-hosted Ubuntu runners are amd64, not arm64.
replace_all_dockerfiles

# Remove interactive TTY flag and make copied certs world-readable.
# GitHub Actions has no TTY, so docker run -it will hang.
replace_all build_images.sh 'docker run -it --rm' 'docker run --rm'
replace_all build_images.sh 'cp /certificates/* /tmp"' 'cp /certificates/* /tmp && chmod 644 /tmp/*.pem"'

# Reduce retry window from 100s to 60s which is sufficient in CI.
replace_all scripts/start-docker.mjs "retry(20, '5s'" "retry(12, '5s'"

# HTTPie reads stdin when there's no TTY; --ignore-stdin prevents the conflict.
replace_all scripts/setup-peerings.mjs '--verify no' '--verify no --ignore-stdin'

# Remove the secondary DC exported-services write from setup-peerings.mjs.
# Secondary DCs reject these config writes, causing the setup to fail.
replace_regex_once \
  scripts/setup-peerings.mjs \
  '(^\s*await createConfigurationEntry\(\{\n\s*host:\s*SERVER_2,\n\s*kind:\s*"exported-services",\n\s*config:\s*\{\n\s*Services:\s*\{\n\s*Name:\s*"redis",\n\s*Namespace:\s*"default",\n\s*Consumers:\s*\[\n\s*\{\n\s*Peer:\s*"from-dc1",\n\s*\},\n\s*\],\n\s*\},\n\s*\},\n\s*\}\);)' \
  '      // Secondary DC exported-services writes are rejected in Consul; skip in CI.'

# Skip the duplicate product-db seed in install.sh.
# The container already seeds via docker-entrypoint-initdb.d/products.sql on init;
# the manual psql import causes "relation already exists" errors.
replace_regex_once \
  install.sh \
  '(^\s*echo "Populate table\.\."\n\s*psql postgres://postgres:password@localhost:5432/products\?sslmode=disable -f /docker-entrypoint-initdb\.d/products\.sql\n\s*if \[ \$\? -eq 0 \]; then\n\s*consul services register /tmp/svc_db\.hcl\n\s*consul config write /tmp/product-db\.hcl\n\s*consul config write /tmp/intention\.hcl\n\s*sudo nohup consul connect envoy -sidecar-for \$SERVICE -token=\$CONSUL_HTTP_TOKEN >/tmp/proxy\.log 2>&1\n\s*else\n\s*sleep 2\n\s*psql postgres://postgres:password@localhost:5432/products\?sslmode=disable -f /docker-entrypoint-initdb\.d/products\.sql\n\s*consul services register /tmp/svc_db\.hcl\n\s*consul config write /tmp/product-db\.hcl\n\s*consul config write /tmp/intention\.hcl\n\s*sudo nohup consul connect envoy -sidecar-for \$SERVICE -token=\$CONSUL_HTTP_TOKEN >/tmp/proxy\.log 2>&1\n\s*fi)' \
  '      echo "Populate table.."\n      consul services register /tmp/svc_db.hcl\n      consul config write /tmp/product-db.hcl\n      consul config write /tmp/intention.hcl\n      sudo nohup consul connect envoy -sidecar-for $SERVICE -token=$CONSUL_HTTP_TOKEN >/tmp/proxy.log 2>&1'
