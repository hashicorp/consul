# For documentation on building consul from source, refer to:
# https://www.consul.io/docs/install#compiling-from-source

SHELL = bash
GOTOOLS = \
	github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@master \
	github.com/hashicorp/go-bindata/go-bindata@master \
	golang.org/x/tools/cmd/cover@master \
	golang.org/x/tools/cmd/stringer@master \
	github.com/vektra/mockery/cmd/mockery@master \
	github.com/golangci/golangci-lint/cmd/golangci-lint@v1.40.1 \
	github.com/hashicorp/lint-consul-retry@master

PROTOC_VERSION=3.12.3
PROTOC_OS := $(shell if test "$$(uname)" == "Darwin"; then echo osx; else echo linux; fi)
PROTOC_ZIP := protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-x86_64.zip
PROTOC_URL := https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ZIP)
PROTOC_ROOT := .protobuf/protoc-$(PROTOC_OS)-$(PROTOC_VERSION)
PROTOC_BIN := $(PROTOC_ROOT)/bin/protoc
GOPROTOVERSION?=$(shell grep github.com/golang/protobuf go.mod | awk '{print $$2}')
GOPROTOTOOLS = \
	github.com/golang/protobuf/protoc-gen-go@$(GOPROTOVERSION) \
	github.com/hashicorp/protoc-gen-go-binary@master \
	github.com/favadi/protoc-go-inject-tag@v1.3.0 \
	github.com/hashicorp/mog@v0.1.1

GOTAGS ?=
GOPATH=$(shell go env GOPATH)
MAIN_GOPATH=$(shell go env GOPATH | cut -d: -f1)

export PATH := $(PWD)/bin:$(PATH)

ASSETFS_PATH?=agent/uiserver/bindata_assetfs.go
# Get the git commit
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
GIT_COMMIT_YEAR?=$(shell git show -s --format=%cd --date=format:%Y HEAD)
GIT_DIRTY?=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=-X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)

PROTOFILES?=$(shell find . -name '*.proto' | grep -v 'vendor/' | grep -v '.protobuf' )
PROTOGOFILES=$(PROTOFILES:.proto=.pb.go)
PROTOGOBINFILES=$(PROTOFILES:.proto=.pb.binary.go)

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
BUILD_CONTAINER_NAME?=consul-builder
CONSUL_IMAGE_VERSION?=latest

################
# CI Variables #
################
CI_DEV_DOCKER_NAMESPACE?=hashicorpdev
CI_DEV_DOCKER_IMAGE_NAME?=consul
CI_DEV_DOCKER_WORKDIR?=bin/
################

TEST_MODCACHE?=1
TEST_BUILDCACHE?=1

# You can only use as many CPUs as you have allocated to docker
ifdef TEST_DOCKER_CPUS
TEST_DOCKER_RESOURCE_CONSTRAINTS=--cpus $(TEST_DOCKER_CPUS)
TEST_PARALLELIZATION=-e GOMAXPROCS=$(TEST_DOCKER_CPUS)
else
TEST_DOCKER_RESOURCE_CONSTRAINTS=
TEST_PARALLELIZATION=
endif

ifeq ($(TEST_MODCACHE), 1)
TEST_MODCACHE_VOL=-v $(MAIN_GOPATH)/pkg/mod:/go/pkg/mod
else
TEST_MODCACHE_VOL=
endif

ifeq ($(TEST_BUILDCACHE), 1)
TEST_BUILDCACHE_VOL=-v $(shell go env GOCACHE):/root/.caches/go-build
else
TEST_BUILDCACHE_VOL=
endif

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


export GO_BUILD_TAG
export UI_BUILD_TAG
export BUILD_CONTAINER_NAME
export GIT_COMMIT
export GIT_COMMIT_YEAR
export GIT_DIRTY
export GOTAGS
export GOLDFLAGS

# Allow skipping docker build during integration tests in CI since we already
# have a built binary
ENVOY_INTEG_DEPS?=dev-docker
ifdef SKIP_DOCKER_BUILD
ENVOY_INTEG_DEPS=noop
endif

all: dev-build

# used to make integration dependencies conditional
noop: ;

# dev creates binaries for testing locally - these are put into ./bin
dev: dev-build

dev-build:
	mkdir -p bin
	CGO_ENABLED=0 go install -ldflags "$(GOLDFLAGS)" -tags "$(GOTAGS)"
	cp -f ${MAIN_GOPATH}/bin/consul ./bin/consul

dev-docker: linux
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building Consul Development container - $(CONSUL_DEV_IMAGE)"
	@docker build $(NOCACHE) $(QUIET) -t '$(CONSUL_DEV_IMAGE)' --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) $(CURDIR)/pkg/bin/linux_amd64 -f $(CURDIR)/build-support/docker/Consul-Dev.dockerfile

# In CircleCI, the linux binary will be attached from a previous step at bin/. This make target
# should only run in CI and not locally.
ci.dev-docker:
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building Consul Development container - $(CI_DEV_DOCKER_IMAGE_NAME)"
	@docker build $(NOCACHE) $(QUIET) -t '$(CI_DEV_DOCKER_NAMESPACE)/$(CI_DEV_DOCKER_IMAGE_NAME):$(GIT_COMMIT)' \
	--build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
	--label COMMIT_SHA=$(CIRCLE_SHA1) \
	--label PULL_REQUEST=$(CIRCLE_PULL_REQUEST) \
	--label CIRCLE_BUILD_URL=$(CIRCLE_BUILD_URL) \
	$(CI_DEV_DOCKER_WORKDIR) -f $(CURDIR)/build-support/docker/Consul-Dev.dockerfile
	@echo $(DOCKER_PASS) | docker login -u="$(DOCKER_USER)" --password-stdin
	@echo "Pushing dev image to: https://cloud.docker.com/u/hashicorpdev/repository/docker/hashicorpdev/consul"
	@docker push $(CI_DEV_DOCKER_NAMESPACE)/$(CI_DEV_DOCKER_IMAGE_NAME):$(GIT_COMMIT)
ifeq ($(CIRCLE_BRANCH), main)
	@docker tag $(CI_DEV_DOCKER_NAMESPACE)/$(CI_DEV_DOCKER_IMAGE_NAME):$(GIT_COMMIT) $(CI_DEV_DOCKER_NAMESPACE)/$(CI_DEV_DOCKER_IMAGE_NAME):latest
	@docker push $(CI_DEV_DOCKER_NAMESPACE)/$(CI_DEV_DOCKER_IMAGE_NAME):latest
endif

# linux builds a linux binary independent of the source platform
linux:
	@mkdir -p ./pkg/bin/linux_amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./pkg/bin/linux_amd64 -ldflags "$(GOLDFLAGS)" -tags "$(GOTAGS)"

# dist builds binaries for all platforms and packages them for distribution
dist:
	@$(SHELL) $(CURDIR)/build-support/scripts/release.sh -t '$(DIST_TAG)' -b '$(DIST_BUILD)' -S '$(DIST_SIGN)' $(DIST_VERSION_ARG) $(DIST_DATE_ARG) $(DIST_REL_ARG)

cover: cov
cov: other-consul dev-build
	go test -tags '$(GOTAGS)' ./... -coverprofile=coverage.out
	cd sdk && go test -tags '$(GOTAGS)' ./... -coverprofile=../coverage.sdk.part
	cd api && go test -tags '$(GOTAGS)' ./... -coverprofile=../coverage.api.part
	grep -h -v "mode: set" coverage.{sdk,api}.part >> coverage.out
	rm -f coverage.{sdk,api}.part
	go tool cover -html=coverage.out

test: other-consul dev-build lint test-internal

go-mod-tidy:
	@echo "--> Running go mod tidy"
	@cd sdk && go mod tidy
	@cd api && go mod tidy
	@go mod tidy

test-internal:
	@echo "--> Running go test"
	@rm -f test.log exit-code
	@# Dump verbose output to test.log so we can surface test names on failure but
	@# hide it from travis as it exceeds their log limits and causes job to be
	@# terminated (over 4MB and over 10k lines in the UI). We need to output
	@# _something_ to stop them terminating us due to inactivity...
	@echo "===================== submodule: sdk =====================" | tee -a test.log
	cd sdk && { go test -v $(GOTEST_FLAGS) -tags '$(GOTAGS)' ./... 2>&1 ; echo $$? >> ../exit-code ; } | tee -a ../test.log | egrep '^(ok|FAIL|panic:|--- FAIL|--- PASS)'
	@echo "===================== submodule: api =====================" | tee -a test.log
	cd api && { go test -v $(GOTEST_FLAGS) -tags '$(GOTAGS)' ./... 2>&1 ; echo $$? >> ../exit-code ; } | tee -a ../test.log | egrep '^(ok|FAIL|panic:|--- FAIL|--- PASS)'
	@echo "===================== submodule: root =====================" | tee -a test.log
	{           go test -v $(GOTEST_FLAGS) -tags '$(GOTAGS)' ./... 2>&1 ; echo $$? >> exit-code    ; } | tee -a test.log    | egrep '^(ok|FAIL|panic:|--- FAIL|--- PASS)'
	@# if everything worked fine then all 3 zeroes will be collapsed to a single zero here
	@exit_codes="$$(sort -u exit-code)" ; echo "$$exit_codes" > exit-code
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

test-docker: linux go-build-image
	@# -ti run in the foreground showing stdout
	@# --rm removes the container once its finished running
	@# GO_MODCACHE_VOL - args for mapping in the go module cache
	@# GO_BUILD_CACHE_VOL - args for mapping in the go build cache
	@# All the env vars are so we pass through all the relevant bits of information
	@# Needed for running the tests
	@# We map in our local linux_amd64 bin directory as thats where the linux dep
	@#   target dropped the binary. We could build the binary in the container too
	@#   but that might take longer as caching gets weird
	@# Lastly we map the source dir here to the /consul workdir
	@echo "Running tests within a docker container"
	@docker run -ti --rm \
		-e 'GOTEST_FLAGS=$(GOTEST_FLAGS)' \
		-e 'GOTAGS=$(GOTAGS)' \
		-e 'GIT_COMMIT=$(GIT_COMMIT)' \
		-e 'GIT_COMMIT_YEAR=$(GIT_COMMIT_YEAR)' \
		-e 'GIT_DIRTY=$(GIT_DIRTY)' \
		$(TEST_PARALLELIZATION) \
		$(TEST_DOCKER_RESOURCE_CONSTRAINTS) \
		$(TEST_MODCACHE_VOL) \
		$(TEST_BUILDCACHE_VOL) \
		-v $(MAIN_GOPATH)/bin/linux_amd64/:/go/bin \
		-v $(shell pwd):/consul \
		$(GO_BUILD_TAG) \
		make test-internal

other-consul:
	@echo "--> Checking for other consul instances"
	@if ps -ef | grep 'consul agent' | grep -v grep ; then \
		echo "Found other running consul agents. This may affect your tests." ; \
		exit 1 ; \
	fi

lint:
	@echo "--> Running go golangci-lint"
	@golangci-lint run --build-tags '$(GOTAGS)' && \
		(cd api && golangci-lint run --build-tags '$(GOTAGS)') && \
		(cd sdk && golangci-lint run --build-tags '$(GOTAGS)')

# If you've run "make ui" manually then this will get called for you. This is
# also run as part of the release build script when it verifies that there are no
# changes to the UI assets that aren't checked in.
static-assets:
	@go-bindata-assetfs -pkg uiserver -prefix pkg -o $(ASSETFS_PATH) ./pkg/web_ui/...
	@go fmt $(ASSETFS_PATH)


# Build the static web ui and build static assets inside a Docker container
ui: ui-docker static-assets-docker

tools: proto-tools
	@if [[ -d .gotools ]]; then rm -rf .gotools ; fi
	@for TOOL in $(GOTOOLS); do \
		echo "=== TOOL: $$TOOL" ; \
		go install -v $$TOOL ; \
	done

proto-tools:
	@if [[ -d .gotools ]]; then rm -rf .gotools ; fi
	@for TOOL in $(GOPROTOTOOLS); do \
		echo "=== TOOL: $$TOOL" ; \
		go install -v $$TOOL ; \
	done

version:
	@echo -n "Version:                    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh
	@echo -n "Version + release:          "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r
	@echo -n "Version + git:              "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh  -g
	@echo -n "Version + release + git:    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r -g


docker-images: go-build-image ui-build-image

go-build-image:
	@echo "Building Golang build container"
	@docker build $(NOCACHE) $(QUIET) --build-arg 'GOTOOLS=$(GOTOOLS)' -t $(GO_BUILD_TAG) - < build-support/docker/Build-Go.dockerfile

ui-build-image:
	@echo "Building UI build container"
	@docker build $(NOCACHE) $(QUIET) -t $(UI_BUILD_TAG) - < build-support/docker/Build-UI.dockerfile

static-assets-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh static-assets

consul-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh consul

ui-docker: ui-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh ui

test-envoy-integ: $(ENVOY_INTEG_DEPS)
	@go test -v -timeout=30m -tags integration ./test/integration/connect/envoy

test-connect-ca-providers:
ifeq ("$(CIRCLECI)","true")
# Run in CI
	gotestsum --format=short-verbose --junitfile "$(TEST_RESULTS_DIR)/gotestsum-report.xml" -- -cover -coverprofile=coverage.txt ./agent/connect/ca
# Run leader tests that require Vault
	gotestsum --format=short-verbose --junitfile "$(TEST_RESULTS_DIR)/gotestsum-report-leader.xml" -- -cover -coverprofile=coverage-leader.txt -run Vault ./agent/consul
# Run agent tests that require Vault
	gotestsum --format=short-verbose --junitfile "$(TEST_RESULTS_DIR)/gotestsum-report-agent.xml" -- -cover -coverprofile=coverage-agent.txt -run Vault ./agent
else
# Run locally
	@echo "Running /agent/connect/ca tests in verbose mode"
	@go test -v ./agent/connect/ca
	@go test -v ./agent/consul -run Vault
	@go test -v ./agent -run Vault
endif

.PHONY: protoc-install
protoc-install:
	$(info locally installing protocol buffer compiler version if needed (expect: $(PROTOC_VERSION)))
	@if [[ ! -x $(PROTOC_ROOT)/bin/protoc ]]; then \
		mkdir -p .protobuf/tmp ; \
		if [[ ! -f .protobuf/tmp/$(PROTOC_ZIP) ]]; then \
			( cd .protobuf/tmp && curl -sSL "$(PROTOC_URL)" -o "$(PROTOC_ZIP)" ) ; \
		fi ; \
		mkdir -p $(PROTOC_ROOT) ; \
		unzip -d $(PROTOC_ROOT) .protobuf/tmp/$(PROTOC_ZIP) ; \
		chmod -R a+Xr $(PROTOC_ROOT) ; \
		chmod +x $(PROTOC_ROOT)/bin/protoc ; \
	fi

proto: protoc-install $(PROTOGOFILES) $(PROTOGOBINFILES)
	@echo "Generated all protobuf Go files"

%.pb.go %.pb.binary.go: %.proto
	@$(SHELL) $(CURDIR)/build-support/scripts/proto-gen.sh --grpc --protoc-bin "$(PROTOC_BIN)" "$<"

# utility to echo a makefile variable (i.e. 'make print-PROTOC_VERSION')
print-%  : ; @echo $($*)

.PHONY: module-versions
# Print a list of modules which can be updated.
# Columns are: module current_version date_of_current_version latest_version
module-versions:
	@go list -m -u -f '{{if .Update}} {{printf "%-50v %-40s" .Path .Version}} {{with .Time}} {{ .Format "2006-01-02" -}} {{else}} {{printf "%9s" ""}} {{end}}   {{ .Update.Version}} {{end}}' all

.PHONY: envoy-library
envoy-library:
	@$(SHELL) $(CURDIR)/build-support/scripts/envoy-library-references.sh

.PHONY: envoy-regen
envoy-regen:
	$(info regenerating envoy golden files)
	@for d in endpoints listeners routes clusters rbac; do \
		if [[ -d "agent/xds/testdata/$${d}" ]]; then \
			find "agent/xds/testdata/$${d}" -name '*.golden' -delete ; \
		fi \
	done
	@go test -tags '$(GOTAGS)' ./agent/xds -update
	@find "command/connect/envoy/testdata" -name '*.golden' -delete
	@go test -tags '$(GOTAGS)' ./command/connect/envoy -update

.PHONY: all bin dev dist cov test test-internal cover lint ui static-assets tools proto-tools protoc-check
.PHONY: docker-images go-build-image ui-build-image static-assets-docker consul-docker ui-docker
.PHONY: version proto test-envoy-integ
