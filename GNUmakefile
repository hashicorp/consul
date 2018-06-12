SHELL = bash
GOTOOLS = \
	github.com/elazarl/go-bindata-assetfs/... \
	github.com/hashicorp/go-bindata/... \
	github.com/magiconair/vendorfmt/cmd/vendorfmt \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/stringer \
	github.com/axw/gocov/gocov \
	gopkg.in/matm/v1/gocov-html

GOTAGS ?=
GOFILES ?= $(shell go list ./... | grep -v /vendor/)
ifeq ($(origin GOTEST_PKGS_EXCLUDE), undefined)
GOTEST_PKGS ?= "./..."
else
GOTEST_PKGS=$(shell go list ./... | sed 's/github.com\/hashicorp\/consul/./' | egrep -v "^($(GOTEST_PKGS_EXCLUDE))$$")
endif
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
GOPATH=$(shell go env GOPATH)

ASSETFS_PATH?=agent/bindata_assetfs.go
# Get the git commit
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
GIT_DIRTY?=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_DESCRIBE?=$(shell git describe --tags --always)
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=-X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY) -X $(GIT_IMPORT).GitDescribe=$(GIT_DESCRIBE)

GO_BUILD_TAG?=consul-build-go
UI_BUILD_TAG?=consul-build-ui
UI_LEGACY_BUILD_TAG?=consul-build-ui-legacy
BUILD_CONTAINER_NAME?=consul-builder

DIST_TAG?=1
DIST_BUILD?=1
DIST_SIGN?=1

export GO_BUILD_TAG
export UI_BUILD_TAG
export UI_LEGACY_BUILD_TAG
export BUILD_CONTAINER_NAME
export GIT_COMMIT
export GIT_DIRTY
export GIT_DESCRIBE
export GOTAGS
export GOLDFLAGS

# all builds binaries for all targets
all: bin

bin: tools
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh consul-local

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev: changelogfmt vendorfmt dev-build

dev-build:
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh consul-local -o '$(GOOS)' -a '$(GOARCH)'

vendorfmt:
	@echo "--> Formatting vendor/vendor.json"
	test -x $(GOPATH)/bin/vendorfmt || go get -u github.com/magiconair/vendorfmt/cmd/vendorfmt
	vendorfmt

changelogfmt:
	@echo "--> Making [GH-xxxx] references clickable..."
	@sed -E 's|([^\[])\[GH-([0-9]+)\]|\1[[GH-\2](https://github.com/hashicorp/consul/issues/\2)]|g' CHANGELOG.md > changelog.tmp && mv changelog.tmp CHANGELOG.md

# linux builds a linux package independent of the source platform
linux:
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh consul-local -o linux -a amd64

# dist builds binaries for all platforms and packages them for distribution
dist:
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh release -t '$(DIST_TAG)' -b '$(DIST_BUILD)' -S '$(DIST_SIGN)'

cov:
	gocov test $(GOFILES) | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test: other-consul dev-build vet
	@echo "--> Running go test"
	@rm -f test.log exit-code
	go test -tags '$(GOTAGS)' -i $(GOTEST_PKGS)
	@# Dump verbose output to test.log so we can surface test names on failure but
	@# hide it from travis as it exceeds their log limits and causes job to be
	@# terminated (over 4MB and over 10k lines in the UI). We need to output
	@# _something_ to stop them terminating us due to inactivity...
	{ go test $(GOTEST_FLAGS) -tags '$(GOTAGS)' -timeout 5m $(GOTEST_PKGS) 2>&1 ; echo $$? > exit-code ; } | tee test.log | egrep '^(ok|FAIL)\s*github.com/hashicorp/consul'
	@echo "Exit code: $$(cat exit-code)" >> test.log
	@grep -A5 'DATA RACE' test.log || true
	@grep -A10 'panic: test timed out' test.log || true
	@grep -A1 -- '--- SKIP:' test.log || true
	@grep -A1 -- '--- FAIL:' test.log || true
	@grep '^FAIL' test.log || true
	@if [ "$$(cat exit-code)" == "0" ] ; then echo "PASS" ; exit 0 ; else exit 1 ; fi

test-race:
	$(MAKE) GOTEST_FLAGS=-race

other-consul:
	@echo "--> Checking for other consul instances"
	@if ps -ef | grep 'consul agent' | grep -v grep ; then \
		echo "Found other running consul agents. This may affect your tests." ; \
		exit 1 ; \
	fi

cover:
	go test $(GOFILES) --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(GOFILES)

vet:
	@echo "--> Running go vet"
	@go vet -tags '$(GOTAGS)' $(GOFILES); if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

# Build the static web ui and build static assets inside a Docker container, the
# same way a release build works. This implicitly does a "make static-assets" at
# the end.
ui: ui-legacy-docker ui-docker static-assets

# If you've run "make ui" manually then this will get called for you. This is
# also run as part of the release build script when it verifies that there are no
# changes to the UI assets that aren't checked in.
static-assets:
	@go-bindata-assetfs -pkg agent -prefix pkg -o $(ASSETFS_PATH) ./pkg/web_ui/...
	$(MAKE) format

tools:
	go get -u -v $(GOTOOLS)

version:
	@echo -n "Version without release: "
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh version 
	@echo -n "Version with release:    "
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh version -R

docker-images:
	@$(MAKE) -C build-support/docker images

go-build-image:
	@$(MAKE) -C build-support/docker go-build-image
	
ui-build-image:
	@$(MAKE) -C build-support/docker ui-build-image
	
ui-legacy-build-image:
	@$(MAKE) -C build-support/docker ui-legacy-build-image

static-assets-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh assetfs	
	
consul-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh consul
	
ui-docker: ui-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh ui
	
ui-legacy-docker: ui-legacy-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build.sh ui-legacy
	
	
.PHONY: all ci bin dev dist cov test cover format vet ui static-assets tools vendorfmt 
.PHONY: docker-images go-build-image ui-build-image ui-legacy-build-image static-assets-docker consul-docker ui-docker ui-legacy-docker version
