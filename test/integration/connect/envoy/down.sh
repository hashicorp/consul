#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

set -x
exec ./run-tests.sh suite_teardown

