# For documentation on building consul from source, refer to:
# https://www.consul.io/docs/install#compiling-from-source

SHELL = bash

###
# These version variables can either be a valid string for "go install <module>@<version>"
# or the string @DEV to imply use what is currently installed locally.
###
GOLANGCI_LINT_VERSION='v1.51.1'
MOCKERY_VERSION='v2.20.0'
BUF_VERSION='v1.4.0'
PROTOC_GEN_GO_GRPC_VERSION="v1.2.0"
MOG_VERSION='v0.4.0'
PROTOC_GO_INJECT_TAG_VERSION='v1.3.0'
PROTOC_GEN_GO_BINARY_VERSION="v0.0.1"
DEEP_COPY_VERSION='bc3f5aa5735d8a54961580a3a24422c308c831c2'

MOCKED_PB_DIRS= pbdns

GOTAGS ?=
GOPATH=$(shell go env GOPATH)
GOARCH?=$(shell go env GOARCH)
MAIN_GOPATH=$(shell go env GOPATH | cut -d: -f1)

export PATH := $(PWD)/bin:$(GOPATH)/bin:$(PATH)

# Get the git commit
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
GIT_COMMIT_YEAR?=$(shell git show -s --format=%cd --date=format:%Y HEAD)
GIT_DIRTY?=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_IMPORT=github.com/hashicorp/consul/version
DATE_FORMAT="%Y-%m-%dT%H:%M:%SZ" # it's tricky to do an RFC3339 format in a cross platform way, so we hardcode UTC
GIT_DATE=$(shell $(CURDIR)/build-support/scripts/build-date.sh) # we're using this for build date because it's stable across platform builds
GOLDFLAGS=-X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY) -X $(GIT_IMPORT).BuildDate=$(GIT_DATE)

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
ENVOY_INTEG_DEPS?=docker-envoy-integ
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
	# rm needed due to signature caching (https://apple.stackexchange.com/a/428388)
	rm -f ./bin/consul
	cp ${MAIN_GOPATH}/bin/consul ./bin/consul

dev-docker: linux dev-build
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building Consul Development container - $(CONSUL_DEV_IMAGE)"
	@#  'consul:local' tag is needed to run the integration tests
	@#  'consul-dev:latest' is needed by older workflows
	@docker buildx use default && docker buildx build -t 'consul:local' -t '$(CONSUL_DEV_IMAGE)' \
       --platform linux/$(GOARCH) \
	   --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
       --load \
       -f $(CURDIR)/build-support/docker/Consul-Dev-Multiarch.dockerfile $(CURDIR)/pkg/bin/

check-remote-dev-image-env:
ifndef REMOTE_DEV_IMAGE
	$(error REMOTE_DEV_IMAGE is undefined: set this image to <your_docker_repo>/<your_docker_image>:<image_tag>, e.g. hashicorp/consul-k8s-dev:latest)
endif

remote-docker: check-remote-dev-image-env
	$(MAKE) GOARCH=amd64 linux
	$(MAKE) GOARCH=arm64 linux
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building and Pushing Consul Development container - $(REMOTE_DEV_IMAGE)"
	@docker buildx use default && docker buildx build -t '$(REMOTE_DEV_IMAGE)' \
       --platform linux/amd64,linux/arm64 \
	   --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
       --push \
       -f $(CURDIR)/build-support/docker/Consul-Dev-Multiarch.dockerfile $(CURDIR)/pkg/bin/

# In CI, the linux binary will be attached from a previous step at bin/. This make target
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

# linux builds a linux binary compatible with the source platform
linux:
	@mkdir -p ./pkg/bin/linux_$(GOARCH)
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o ./pkg/bin/linux_$(GOARCH) -ldflags "$(GOLDFLAGS)" -tags "$(GOTAGS)"

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

lint: lint-tools
	@echo "--> Running golangci-lint"
	@golangci-lint run --build-tags '$(GOTAGS)' && \
		(cd api && golangci-lint run --build-tags '$(GOTAGS)') && \
		(cd sdk && golangci-lint run --build-tags '$(GOTAGS)')
	@echo "--> Running lint-consul-retry"
	@lint-consul-retry
	@echo "--> Running enumcover"
	@enumcover ./...

# Build the static web ui inside a Docker container. For local testing only; do not commit these assets.
ui: ui-docker

# Build the static web ui with yarn. This is the version to commit.
.PHONY: ui-regen
ui-regen:
	cd $(CURDIR)/ui && make && cd ..
	rm -rf $(CURDIR)/agent/uiserver/dist
	mv $(CURDIR)/ui/packages/consul-ui/dist $(CURDIR)/agent/uiserver/

tools:
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh

.PHONY: lint-tools
lint-tools:
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -lint

.PHONY: proto-tools
proto-tools:
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -protobuf

.PHONY: codegen-tools
codegen-tools:
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -codegen

.PHONY: deep-copy
deep-copy:
	@$(SHELL) $(CURDIR)/agent/structs/deep-copy.sh
	@$(SHELL) $(CURDIR)/agent/proxycfg/deep-copy.sh

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
	@docker build $(NOCACHE) $(QUIET) -t $(GO_BUILD_TAG) - < build-support/docker/Build-Go.dockerfile

ui-build-image:
	@echo "Building UI build container"
	@docker build $(NOCACHE) $(QUIET) -t $(UI_BUILD_TAG) - < build-support/docker/Build-UI.dockerfile

# Builds consul in a docker container and then dumps executable into ./pkg/bin/...
consul-docker: go-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh consul

ui-docker: ui-build-image
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh ui

# Build image used to run integration tests locally.
docker-envoy-integ:
	$(MAKE) GOARCH=amd64 linux
	docker build \
      --platform linux/amd64 $(NOCACHE) $(QUIET) \
      -t 'consul:local' \
      --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
      $(CURDIR)/pkg/bin/linux_amd64 \
      -f $(CURDIR)/build-support/docker/Consul-Dev.dockerfile

# Run integration tests.
# Use GO_TEST_FLAGS to run specific tests:
#      make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/case-basic"
# NOTE: Always uses amd64 images, even when running on M1 macs, to match CI/CD environment.
test-envoy-integ: $(ENVOY_INTEG_DEPS)
	@go test -v -timeout=30m -tags integration $(GO_TEST_FLAGS) ./test/integration/connect/envoy

.PHONY: test-compat-integ
test-compat-integ: dev-docker
ifeq ("$(GOTAGS)","")
	@docker tag consul-dev:latest consul:local
	@docker run --rm -t consul:local consul version
	@cd ./test/integration/consul-container && \
		go test -v -timeout=30m ./... --target-version local --latest-version latest
else
	@docker tag consul-dev:latest hashicorp/consul-enterprise:local
	@docker run --rm -t hashicorp/consul-enterprise:local consul version
	@cd ./test/integration/consul-container && \
		go test -v -timeout=30m ./... --tags $(GOTAGS) --target-version local --latest-version latest
endif

.PHONY: test-metrics-integ
test-metrics-integ: dev-docker
	@docker tag consul-dev:latest consul:local
	@docker run --rm -t consul:local consul version
	@cd ./test/integration/consul-container && \
		go test -v -timeout=7m ./metrics --target-version local

test-connect-ca-providers:
	@echo "Running /agent/connect/ca tests in verbose mode"
	@go test -v ./agent/connect/ca
	@go test -v ./agent/consul -run Vault
	@go test -v ./agent -run Vault

.PHONY: proto
proto: proto-tools proto-gen proto-mocks

.PHONY: proto-gen
proto-gen: proto-tools
	@$(SHELL) $(CURDIR)/build-support/scripts/protobuf.sh

.PHONY: proto-mocks
proto-mocks:
	for dir in $(MOCKED_PB_DIRS) ; do \
		cd proto-public && \
		rm -f $$dir/mock*.go && \
		mockery --dir $$dir --inpackage --all --recursive --log-level trace ; \
	done

.PHONY: proto-format
proto-format: proto-tools
	@buf format -w

.PHONY: proto-lint
proto-lint: proto-tools
	@buf lint --config proto/buf.yaml --path proto
	@buf lint --config proto-public/buf.yaml --path proto-public
	@for fn in $$(find proto -name '*.proto'); do \
		if [[ "$$fn" = "proto/pbsubscribe/subscribe.proto" ]]; then \
			continue ; \
		elif [[ "$$fn" = "proto/pbpartition/partition.proto" ]]; then \
			continue ; \
		fi ; \
		pkg=$$(grep "^package " "$$fn" | sed 's/^package \(.*\);/\1/'); \
		if [[ "$$pkg" != hashicorp.consul.internal.* ]]; then \
			echo "ERROR: $$fn: is missing 'hashicorp.consul.internal' package prefix: $$pkg" >&2; \
			exit 1; \
		fi \
	done

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

# Point your web browser to http://localhost:3000/consul to live render docs from ./website/
.PHONY: docs
docs:
	make -C website

.PHONY: help
help:
	$(info available make targets)
	$(info ----------------------)
	@grep "^[a-z0-9-][a-z0-9.-]*:" GNUmakefile  | cut -d':' -f1 | sort

.PHONY: all bin dev dist cov test test-internal cover lint ui tools
.PHONY: docker-images go-build-image ui-build-image consul-docker ui-docker
.PHONY: version test-envoy-integ
