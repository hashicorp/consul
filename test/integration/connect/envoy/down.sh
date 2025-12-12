#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

cd "$(dirname "$0")"

set -x
exec ./run-tests.sh suite_teardown

