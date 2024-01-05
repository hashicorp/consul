# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

#
# NOTE: This file is meant to be sourced from other bash scripts/shells
#
# It provides all the scripting around building Consul and the release process

readonly FUNC_DIR="$(dirname "$(dirname "${BASH_SOURCE[0]}")")/functions"

func_sources=$(find ${FUNC_DIR} -mindepth 1 -maxdepth 1 -name "*.sh" -type f | sort -n)

for src in $func_sources
do
   source $src
done
