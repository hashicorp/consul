.PHONY: help check
.DEFAULT_GOAL := help

SUBPKGS=cpu disk docker host internal load mem net process

help:  ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

check:  ## Check
	errcheck -ignore="Close|Run|Write" ./...
	golint ./... | egrep -v 'underscores|HttpOnly|should have comment|comment on exported|CamelCase|VM|UID' && exit 1 || exit 0

BUILD_FAIL_PATTERN=grep -v "exec format error" | grep "build failed" && exit 1 || exit 0
build_test:  ## test only buildable
	# Supported operating systems
	GOOS=linux go test ./... | $(BUILD_FAIL_PATTERN)
	GOOS=freebsd go test ./... | $(BUILD_FAIL_PATTERN)
	GOOS=openbsd go test ./... | $(BUILD_FAIL_PATTERN)
	CGO_ENABLED=0 GOOS=darwin go test ./... | $(BUILD_FAIL_PATTERN)
	CGO_ENABLED=1 GOOS=darwin go test ./... | $(BUILD_FAIL_PATTERN)
	GOOS=windows go test ./... | $(BUILD_FAIL_PATTERN)
	# Operating systems supported for building only (not implemented error if used)
	GOOS=dragonfly go test ./... | $(BUILD_FAIL_PATTERN)
	GOOS=netbsd go test ./... | $(BUILD_FAIL_PATTERN)
	GOOS=solaris go test ./... | $(BUILD_FAIL_PATTERN)
	@echo 'Successfully built on all known operating systems'
