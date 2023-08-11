#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

cd "$(dirname "$0")"

set -x
exec ./run-tests.sh suite_teardown

