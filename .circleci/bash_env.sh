#!/bin/bash

export GIT_COMMIT=$(git rev-parse --short HEAD)
export GIT_COMMIT_YEAR=$(git show -s --format=%cd --date=format:%Y HEAD)
export GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
export GIT_IMPORT=github.com/hashicorp/consul/version
# we're using this for build date because it's stable across platform builds
# the env -i and -noprofile are used to ensure we don't try to recursively call this profile when starting bash
export GIT_DATE=$(env -i /bin/bash --noprofile -norc ${CIRCLE_WORKING_DIRECTORY}/build-support/scripts/build-date.sh)
export GOLDFLAGS="-X ${GIT_IMPORT}.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X ${GIT_IMPORT}.BuildDate=${GIT_DATE}"
