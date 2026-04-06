#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0
#
# Apply runtime patches to consul-ui-testing so it works on GitHub Actions CI.
# Must be run from within the consul-ui-testing checkout directory.

set -euo pipefail

# Fix architecture: GitHub-hosted Ubuntu runners are amd64, not arm64.
find . -name 'Dockerfile*' -type f -print0 | xargs -0 sed -i 's|linux/arm64|linux/amd64|g'

# Remove interactive TTY flag and make copied certs world-readable.
# GitHub Actions has no TTY, so docker run -it will hang.
sed -i -e 's|docker run -it --rm|docker run --rm|g' \
       -e 's|cp /certificates/\* /tmp"|cp /certificates/* /tmp \&\& chmod 644 /tmp/*.pem"|g' build_images.sh

# Reduce retry window from 100s to 60s which is sufficient in CI.
sed -i "s/retry(20, '5s'/retry(12, '5s'/g" scripts/start-docker.mjs

# HTTPie reads stdin when there's no TTY; --ignore-stdin prevents the conflict.
sed -i 's/--verify no/--verify no --ignore-stdin/g' scripts/setup-peerings.mjs

# Two structural multi-line edits — done in Python for readability.
python3 << 'EOF'
import re, sys

# Remove the secondary DC exported-services write from setup-peerings.mjs.
# Secondary DCs reject these config writes, causing the setup to fail.
with open("scripts/setup-peerings.mjs", "r+") as f:
    content = f.read()
    patched, n = re.subn(
        r'\n\s*await createConfigurationEntry\(\{'
        r'\n\s*host:\s*SERVER_2,'
        r'\n\s*kind:\s*["\']exported-services["\'],'
        r'\n\s*config:\s*\{'
        r'\n\s*Services:\s*\{'
        r'\n\s*Name:\s*["\']redis["\'],'
        r'\n\s*Namespace:\s*["\']default["\'],'
        r'\n\s*Consumers:\s*\['
        r'\n\s*\{'
        r'\n\s*Peer:\s*["\']from-dc1["\'],'
        r'\n\s*\},'
        r'\n\s*\],'
        r'\n\s*\},'
        r'\n\s*\},'
        r'\n\s*\}\);',
        "\n      // Secondary DC exported-services writes are rejected in Consul; skip in CI.\n",
        content,
    )
    if n == 0:
        print("ERROR: exported-services block not found in setup-peerings.mjs", file=sys.stderr)
        sys.exit(1)
    f.seek(0); f.write(patched); f.truncate()

# Skip the duplicate product-db seed in install.sh.
# The container already seeds via docker-entrypoint-initdb.d/products.sql on init;
# the manual psql import causes "relation already exists" errors.
with open("install.sh", "r+") as f:
    content = f.read()
    patched, n = re.subn(
        r'\n\s*echo "Populate table\.\."\n'
        r'\s*psql postgres://postgres:password@localhost:5432/products\?sslmode=disable'
        r' -f /docker-entrypoint-initdb\.d/products\.sql\n'
        r'\s*if \[ \$\? -eq 0 \]; then\n'
        r'\s*consul services register /tmp/svc_db\.hcl\n'
        r'\s*consul config write /tmp/product-db\.hcl\n'
        r'\s*consul config write /tmp/intention\.hcl\n'
        r'\s*sudo nohup consul connect envoy -sidecar-for \$SERVICE'
        r' -token=\$CONSUL_HTTP_TOKEN >/tmp/proxy\.log 2>&1\n'
        r'\s*else\n'
        r'\s*sleep 2\n'
        r'\s*psql postgres://postgres:password@localhost:5432/products\?sslmode=disable'
        r' -f /docker-entrypoint-initdb\.d/products\.sql\n'
        r'\s*consul services register /tmp/svc_db\.hcl\n'
        r'\s*consul config write /tmp/product-db\.hcl\n'
        r'\s*consul config write /tmp/intention\.hcl\n'
        r'\s*sudo nohup consul connect envoy -sidecar-for \$SERVICE'
        r' -token=\$CONSUL_HTTP_TOKEN >/tmp/proxy\.log 2>&1\n'
        r'\s*fi',
        '\n      consul services register /tmp/svc_db.hcl\n'
        '      consul config write /tmp/product-db.hcl\n'
        '      consul config write /tmp/intention.hcl\n'
        '      sudo nohup consul connect envoy -sidecar-for $SERVICE -token=$CONSUL_HTTP_TOKEN >/tmp/proxy.log 2>&1',
        content,
    )
    if n == 0:
        print("ERROR: product-db seed block not found in install.sh", file=sys.stderr)
        sys.exit(1)
    f.seek(0); f.write(patched); f.truncate()
EOF
