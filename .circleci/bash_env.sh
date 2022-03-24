#!/bin/bash

export GIT_COMMIT=$(git rev-parse --short HEAD)
export GIT_COMMIT_YEAR=$(git show -s --format=%cd --date=format:%Y HEAD)
export GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
export GIT_IMPORT=github.com/hashicorp/consul/version
export GIT_DATE=$("${CIRCLE_WORKING_DIRECTORY}/build-support/scripts/build-date.sh") # we're using this for build date because it's stable across platform builds
export GOLDFLAGS="-X ${GIT_IMPORT}.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X $(GIT_IMPORT).BuildDate=$(GIT_DATE)"
