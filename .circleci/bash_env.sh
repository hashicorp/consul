#!/bin/bash

export GIT_COMMIT=$(git rev-parse --short HEAD)
export GIT_DIRTY=$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
export GIT_DESCRIBE=$(git describe --tags --always --match "v*")
export GIT_IMPORT=github.com/hashicorp/consul/version
export GOLDFLAGS="-X ${GIT_IMPORT}.GitCommit=${GIT_COMMIT}${GIT_DIRTY} -X ${GIT_IMPORT}.GitDescribe=${GIT_DESCRIBE}"