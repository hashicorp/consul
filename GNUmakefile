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
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
GOPATH=$(shell go env GOPATH)

ASSETFS_PATH?=agent/bindata_assetfs.go
# Get the git commit
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
GIT_DIRTY?=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_DESCRIBE?=$(shell git describe --tags --always)
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=-X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY) -X $(GIT_IMPORT).GitDescribe=$(GIT_DESCRIBE)

ifeq ($(FORCE_REBUILD),1)
NOCACHE=--no-cache
else
NOCACHE=
endif

DOCKER_BUILD_QUIET?=1
ifeq (${DOCKER_BUILD_QUIET},1)
QUIET=-q
else
QUIET=
endif

CONSUL_DEV_IMAGE?=consul-dev
GO_BUILD_TAG?=consul-build-go
UI_BUILD_TAG?=consul-build-ui
UI_LEGACY_BUILD_TAG?=consul-build-ui-legacy
BUILD_CONTAINER_NAME?=consul-builder

DIST_TAG?=1
DIST_BUILD?=1
DIST_SIGN?=1

ifdef DIST_VERSION
DIST_VERSION_ARG=-v "$(DIST_VERSION)"
else
DIST_VERSION_ARG=
endif

ifdef DIST_RELEASE_DATE
DIST_DATE_ARG=-d "$(DIST_RELEASE_DATE)"
else
DIST_DATE_ARG=
endif

ifdef DIST_PRERELEASE
DIST_REL_ARG=-r "$(DIST_PRERELEASE)"
else
DIST_REL_ARG=
endif

PUB_GIT?=1
PUB_WEBSITE?=1

ifeq ($(PUB_GIT),1)
PUB_GIT_ARG=-g
else
PUB_GIT_ARG=
endif

ifeq ($(PUB_WEBSITE),1)
PUB_WEBSITE_ARG=-w
else
PUB_WEBSITE_ARG=
endif

NOGOX?=1

export NOGOX
export GO_BUILD_TAG
export UI_BUILD_TAG
export UI_LEGACY_BUILD_TAG
export BUILD_CONTAINER_NAME
export GIT_COMMIT
export GIT_DIRTY
export GIT_DESCRIBE
export GOTAGS
export GOLDFLAGS


DEV_PUSH?=0
ifeq ($(DEV_PUSH),1)
DEV_PUSH_ARG=
else
DEV_PUSH_ARG=--no-push
endif

# all builds binaries for all targets
all: bin

bin: tools
	@$(SHELL) $(CURDIR)/build-support/scripts/build-local.sh

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev: changelogfmt vendorfmt dev-build

dev-build:
	@$(SHELL) $(CURDIR)/build-support/scripts/build-local.sh -o $(GOOS) -a $(GOARCH)

dev-docker:
	@docker build -t '$(CONSUL_DEV_IMAGE)' --build-arg 'GIT_COMMIT=$(GIT_COMMIT)' --build-arg 'GIT_DIRTY=$(GIT_DIRTY)' --build-arg 'GIT_DESCRIBE=$(GIT_DESCRIBE)' -f $(CURDIR)/build-support/docker/Consul-Dev.dockerfile $(CURDIR)

vendorfmt:
	@echo "--> Formatting vendor/vendor.json"
	test -x $(GOPATH)/bin/vendorfmt || go get -u github.com/magiconair/vendorfmt/cmd/vendorfmt
	vendorfmt

changelogfmt:
	@echo "--> Making [GH-xxxx] references clickable..."
	@sed -E 's|([^\[])\[GH-([0-9]+)\]|\1[[GH-\2](https://github.com/hashicorp/consul/issues/\2)]|g' CHANGELOG.md > changelog.tmp && mv changelog.tmp CHANGELOG.md

# linux builds a linux package independent of the source platform
linux:
	@$(SHELL) $(CURDIR)/build-support/scripts/build-local.sh -o linux -a amd64

# dist builds binaries for all platforms and packages them for distribution
dist:
	@$(SHELL) $(CURDIR)/build-support/scripts/release.sh -t '$(DIST_TAG)' -b '$(DIST_BUILD)' -S '$(DIST_SIGN)' $(DIST_VERSION_ARG) $(DIST_DATE_ARG) $(DIST_REL_ARG)

verify:
	@$(SHELL) $(CURDIR)/build-support/scripts/verify.sh	

publish:
	@$(SHELL) $(CURDIR)/build-support/scripts/publish.sh $(PUB_GIT_ARG) $(PUB_WEBSITE_ARG)

dev-tree:
	@$(SHELL) $(CURDIR)/build-support/scripts/dev.sh $(DEV_PUSH_ARG)

cov:
	gocov test $(GOFILES) | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test: other-consul dev-build vet test-install-deps test-internal

test-install-deps:
	go test -tags '$(GOTAGS)' -i $(GOTEST_PKGS)

test-internal:
	@echo "--> Running go test"
	@rm -f test.log exit-code
	@# Dump verbose output to test.log so we can surface test names on failure but
	@# hide it from travis as it exceeds their log limits and causes job to be
	@# terminated (over 4MB and over 10k lines in the UI). We need to output
	@# _something_ to stop them terminating us due to inactivity...
	{ go test $(GOTEST_FLAGS) -tags '$(GOTAGS)' $(GOTEST_PKGS) 2>&1 ; echo $$? > exit-code ; } | tee test.log | egrep '^(ok|FAIL|panic:|--- FAIL)'
	@echo "Exit code: $$(cat exit-code)"
	@# This prints all the race report between ====== lines
	@awk '/^WARNING: DATA RACE/ {do_print=1; print "=================="} do_print==1 {print} /^={10,}/ {do_print=0}' test.log || true
	@grep -A10 'panic: ' test.log || true
	@# Prints all the failure output until the next non-indented line - testify
	@# helpers often output multiple lines for readability but useless if we can't
	@# see them. Un-intuitive order of matches is necessary. No || true because
	@# awk always returns true even if there is no match and it breaks non-bash
	@# shells locally.
	@awk '/^[^[:space:]]/ {do_print=0} /--- SKIP/ {do_print=1} do_print==1 {print}' test.log
	@awk '/^[^[:space:]]/ {do_print=0} /--- FAIL/ {do_print=1} do_print==1 {print}' test.log
	@grep '^FAIL' test.log || true
	@if [ "$$(cat exit-code)" == "0" ] ; then echo "PASS" ; exit 0 ; else exit 1 ; fi

test-race:
	$(MAKE) GOTEST_FLAGS=-race

# Run tests with config for CI so `make test` can still be local-dev friendly.
test-ci: other-consul dev-build vet test-install-deps
	@ if ! GOTEST_FLAGS="-short -timeout 8m -p 3 -parallel 4" make test-internal; then \
	    echo "    ============"; \
	    echo "      Retrying 1/2"; \
	    echo "    ============"; \
	    if ! GOTEST_FLAGS="-timeout 8m -p 1 -parallel 1" make test-internal; then \
	       echo "    ============"; \
	       echo "      Retrying 2/2"; \
	       echo "    ============"; \
	       GOTEST_FLAGS="-timeout 9m -p 1 -parallel 1" make test-internal; \
	    fi \
	fi

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

# If you've run "make ui" manually then this will get called for you. This is
# also run as part of the release build script when it verifies that there are no
# changes to the UI assets that aren't checked in.
static-assets:
	@go-bindata-assetfs -pkg agent -prefix pkg -o $(ASSETFS_PATH) ./pkg/web_ui/...
	$(MAKE) format


# Build the static web ui and build static assets inside a Docker container
ui: ui-legacy-docker ui-docker static-assets-docker

tools:
	go get -u -v $(GOTOOLS)

version:
	@echo -n "Version:                    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh
	@echo -n "Version + release:          "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r
	@echo -n "Version + git:              "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh  -g
	@echo -n "Version + release + git:    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r -g
		

docker-images: go-build-image ui-build-image ui-legacy-build-image

go-build-image:
	@echo "Building Golang build container"
	@docker build $(NOCACHE) $(QUIET) --build-arg 'GOTOOLS=$(GOTOOLS)' -t $(GO_BUILD_TAG) - < build-support/docker/Build-Go.dockerfile
	
ui-build-image:
	@echo "Building UI build container"
	@docker build $(NOCACHE) $(QUIET) -t $(UI_BUILD_TAG) - < build-support/docker/Build-UI.dockerfile
	
ui-legacy-build-image:
	@echo "Building Legacy UI build container"
	@docker build $(NOCACHE) $(QUIET) -t $(UI_LEGACY_BUILD_TAG) - < build-support/docker/Build-UI-Legacy.dockerfile

static-assets-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh static-assets
	
consul-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh consul
	
ui-docker: ui-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh ui
	
ui-legacy-docker: ui-legacy-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh ui-legacy
	
	
.PHONY: all ci bin dev dist cov test test-ci test-internal test-install-deps cover format vet ui static-assets tools vendorfmt
.PHONY: docker-images go-build-image ui-build-image ui-legacy-build-image static-assets-docker consul-docker ui-docker ui-legacy-docker version
