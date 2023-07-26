#!/bin/bash

set -euo pipefail

package_dir=$1

GODEBUG=gocachehash=1 go test -c -tags="$GOTAGS" -o "$package_dir/test.bin" "$package_dir" 2>&1 | tee "$package_dir/test.bin.buildlog"
grep '^HASH ' "$package_dir/test.bin.buildlog" > "$package_dir/test.bin.hashlog"