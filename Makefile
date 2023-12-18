# For documentation on building consul from source, refer to:
# https://www.consul.io/docs/install#compiling-from-source

SHELL = bash


GO_MODULES := $(shell find . -name go.mod -exec dirname {} \; | grep -v "proto-gen-rpc-glue/e2e" | sort)

###
# These version variables can either be a valid string for "go install <module>@<version>"
# or the string @DEV to imply use what is currently installed locally.
###
GOLANGCI_LINT_VERSION='v1.51.1'
MOCKERY_VERSION='v2.37.1'
BUF_VERSION='v1.26.0'

PROTOC_GEN_GO_GRPC_VERSION='v1.2.0'
MOG_VERSION='v0.4.1'
PROTOC_GO_INJECT_TAG_VERSION='v1.3.0'
PROTOC_GEN_GO_BINARY_VERSION='v0.1.0'
DEEP_COPY_VERSION='bc3f5aa5735d8a54961580a3a24422c308c831c2'
COPYWRITE_TOOL_VERSION='v0.16.4'
LINT_CONSUL_RETRY_VERSION='v1.4.0'
# Go imports formatter
GCI_VERSION='v0.11.2'

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

GOTESTSUM_PATH?=$(shell command -v gotestsum)

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

ifeq ("$(GOTAGS)","")
CONSUL_COMPAT_TEST_IMAGE=hashicorp/consul
else
CONSUL_COMPAT_TEST_IMAGE=hashicorp/consul-enterprise
endif

CONSUL_DEV_IMAGE?=consul-dev
GO_BUILD_TAG?=consul-build-go
UI_BUILD_TAG?=consul-build-ui
BUILD_CONTAINER_NAME?=consul-builder
CONSUL_IMAGE_VERSION?=latest
ENVOY_VERSION?='1.25.4'
CONSUL_DATAPLANE_IMAGE := $(or $(CONSUL_DATAPLANE_IMAGE),"docker.io/hashicorppreview/consul-dataplane:1.3-dev-ubi")
DEPLOYER_CONSUL_DATAPLANE_IMAGE := $(or $(DEPLOYER_CONSUL_DATAPLANE_IMAGE), "docker.io/hashicorppreview/consul-dataplane:1.3-dev")

CONSUL_VERSION?=$(shell cat version/VERSION)

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

##@ Build

.PHONY: all
all: dev-build ## Command running by default

# used to make integration dependencies conditional
noop: ;

.PHONY: dev
dev: dev-build ## Dev creates binaries for testing locally - these are put into ./bin

.PHONY: dev-build
dev-build: ## Same as dev
	mkdir -p bin
	CGO_ENABLED=0 go install -ldflags "$(GOLDFLAGS)" -tags "$(GOTAGS)"
	# rm needed due to signature caching (https://apple.stackexchange.com/a/428388)
	rm -f ./bin/consul
	cp ${MAIN_GOPATH}/bin/consul ./bin/consul

.PHONY: dev-docker-dbg
dev-docker-dbg: dev-docker ## Build containers for debug mode
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull hashicorp/consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building Consul Development container - $(CONSUL_DEV_IMAGE)"
	@#  'consul-dbg:local' tag is needed to run the integration tests
	@#  'consul-dev:latest' is needed by older workflows
	@docker buildx use default && docker buildx build -t $(CONSUL_COMPAT_TEST_IMAGE)-dbg:local \
       --platform linux/$(GOARCH) \
	   --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
       --load \
       -f $(CURDIR)/build-support/docker/Consul-Dev-Dbg.dockerfile $(CURDIR)/pkg/bin/

.PHONY: dev-docker
dev-docker: linux dev-build ## Build and tag docker images in dev env
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull hashicorp/consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building Consul Development container - $(CONSUL_DEV_IMAGE)"
	@#  'consul:local' tag is needed to run the integration tests
	@#  'consul-dev:latest' is needed by older workflows
	@docker buildx use default && docker buildx build -t 'consul:local' -t '$(CONSUL_DEV_IMAGE)' \
       --platform linux/$(GOARCH) \
	   --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
		--label org.opencontainers.image.version=$(CONSUL_VERSION) \
		--label version=$(CONSUL_VERSION) \
       --load \
       -f $(CURDIR)/build-support/docker/Consul-Dev-Multiarch.dockerfile $(CURDIR)/pkg/bin/
	docker tag 'consul:local'  '$(CONSUL_COMPAT_TEST_IMAGE):local'

.PHONY: check-remote-dev-image-env
check-remote-dev-image-env: ## Check remote dev image env
ifndef REMOTE_DEV_IMAGE
	$(error REMOTE_DEV_IMAGE is undefined: set this image to <your_docker_repo>/<your_docker_image>:<image_tag>, e.g. hashicorp/consul-k8s-dev:latest)
endif

.PHONY: remote-docker
remote-docker: check-remote-dev-image-env ## Remote docker
	$(MAKE) GOARCH=amd64 linux
	$(MAKE) GOARCH=arm64 linux
	@echo "Pulling consul container image - $(CONSUL_IMAGE_VERSION)"
	@docker pull hashicorp/consul:$(CONSUL_IMAGE_VERSION) >/dev/null
	@echo "Building and Pushing Consul Development container - $(REMOTE_DEV_IMAGE)"
	@if ! docker buildx inspect consul-builder; then \
		docker buildx create --name consul-builder --driver docker-container --bootstrap; \
	fi; 
	@docker buildx use consul-builder && docker buildx build -t '$(REMOTE_DEV_IMAGE)' \
       --platform linux/amd64,linux/arm64 \
	   --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
		--label org.opencontainers.image.version=$(CONSUL_VERSION) \
		--label version=$(CONSUL_VERSION) \
       --push \
       -f $(CURDIR)/build-support/docker/Consul-Dev-Multiarch.dockerfile $(CURDIR)/pkg/bin/

linux:  ## Linux builds a linux binary compatible with the source platform
	@mkdir -p ./pkg/bin/linux_$(GOARCH)
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o ./pkg/bin/linux_$(GOARCH) -ldflags "$(GOLDFLAGS)" -tags "$(GOTAGS)"

.PHONY: go-mod-tidy
go-mod-tidy: $(foreach mod,$(GO_MODULES),go-mod-tidy/$(mod)) ## Run go mod tidy in every module

.PHONY: mod-tidy/%
go-mod-tidy/%:
	@echo "--> Running go mod tidy ($*)"
	@cd $* && go mod tidy

##@ Checks

.PHONY: fmt
fmt: $(foreach mod,$(GO_MODULES),fmt/$(mod)) ## Format go modules

.PHONY: fmt/%
fmt/%:
	@echo "--> Running go fmt ($*)"
	@cd $* && gofmt -s -l -w .

.PHONY: lint
lint: $(foreach mod,$(GO_MODULES),lint/$(mod)) lint-container-test-deps ## Lint go modules and test deps

.PHONY: lint/%
lint/%:
	@echo "--> Running golangci-lint ($*)"
	@cd $* && GOWORK=off golangci-lint run --build-tags '$(GOTAGS)'
	@echo "--> Running lint-consul-retry ($*)"
	@cd $* && GOWORK=off lint-consul-retry
	@echo "--> Running enumcover ($*)"
	@cd $* && GOWORK=off enumcover ./...

.PHONY: lint-consul-retry
lint-consul-retry: $(foreach mod,$(GO_MODULES),lint-consul-retry/$(mod))

.PHONY: lint-consul-retry/%
lint-consul-retry/%: lint-tools
	@echo "--> Running lint-consul-retry ($*)"
	@cd $* && GOWORK=off lint-consul-retry


# check that the test-container module only imports allowlisted packages
# from the root consul module. Generally we don't want to allow these imports.
# In a few specific instances though it is okay to import test definitions and
# helpers from some of the packages in the root module.
.PHONY: lint-container-test-deps
lint-container-test-deps: ## Check that the test-container module only imports allowlisted packages from the root consul module.
	@echo "--> Checking container tests for bad dependencies"
	@cd test/integration/consul-container && \
		$(CURDIR)/build-support/scripts/check-allowed-imports.sh \
			github.com/hashicorp/consul \
			"internal/catalog/catalogtest" \
			"internal/resource/resourcetest"

##@ Testing

.PHONY: cover
cover: cov ## Run tests and generate coverage report

.PHONY: cov
cov: other-consul dev-build
	go test -tags '$(GOTAGS)' ./... -coverprofile=coverage.out
	cd sdk && go test -tags '$(GOTAGS)' ./... -coverprofile=../coverage.sdk.part
	cd api && go test -tags '$(GOTAGS)' ./... -coverprofile=../coverage.api.part
	grep -h -v "mode: set" coverage.{sdk,api}.part >> coverage.out
	rm -f coverage.{sdk,api}.part
	go tool cover -html=coverage.out

.PHONY: test
test: other-consul dev-build lint test-internal

.PHONY: test-internal
test-internal: ## Test internal
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

.PHONY: test-all
test-all: other-consul dev-build lint $(foreach mod,$(GO_MODULES),test-module/$(mod)) ## Test all

.PHONY: test-module/%
test-module/%:
	@echo "--> Running go test ($*)"
	cd $* && go test $(GOTEST_FLAGS) -tags '$(GOTAGS)' ./...

.PHONY: test-race
test-race: ## Test race
	$(MAKE) GOTEST_FLAGS=-race

.PHONY: other-consul
other-consul: ## Checking for other consul instances
	@echo "--> Checking for other consul instances"
	@if ps -ef | grep 'consul agent' | grep -v grep ; then \
		echo "Found other running consul agents. This may affect your tests." ; \
		exit 1 ; \
	fi

# Use GO_TEST_FLAGS to run specific tests:
#      make test-envoy-integ GO_TEST_FLAGS="-run TestEnvoy/case-basic"
# NOTE: Always uses amd64 images, even when running on M1 macs, to match CI/CD environment.
#       You can also specify the envoy version (example: 1.27.0) setting the environment variable: ENVOY_VERSION=1.27.0
.PHONY: test-envoy-integ
test-envoy-integ: $(ENVOY_INTEG_DEPS) ## Run envoy integration tests.
	@go test -v -timeout=30m -tags integration $(GO_TEST_FLAGS) ./test/integration/connect/envoy

# NOTE: Use DOCKER_BUILDKIT=0, if docker build fails to resolve consul:local base image
.PHONY: test-compat-integ-setup
test-compat-integ-setup: test-deployer-setup
	@#  'consul-envoy:target-version' is needed by compatibility integ test
	@docker build -t consul-envoy:target-version --build-arg CONSUL_IMAGE=$(CONSUL_COMPAT_TEST_IMAGE):local --build-arg ENVOY_VERSION=${ENVOY_VERSION} -f ./test/integration/consul-container/assets/Dockerfile-consul-envoy ./test/integration/consul-container/assets
	@docker build -t consul-dataplane:local --build-arg CONSUL_IMAGE=$(CONSUL_COMPAT_TEST_IMAGE):local --build-arg CONSUL_DATAPLANE_IMAGE=${CONSUL_DATAPLANE_IMAGE} -f ./test/integration/consul-container/assets/Dockerfile-consul-dataplane ./test/integration/consul-container/assets

# NOTE: Use DOCKER_BUILDKIT=0, if docker build fails to resolve consul:local base image
.PHONY: test-deployer-setup
test-deployer-setup: dev-docker
	@docker tag consul-dev:latest $(CONSUL_COMPAT_TEST_IMAGE):local
	@docker run --rm -t $(CONSUL_COMPAT_TEST_IMAGE):local consul version

.PHONY: test-deployer
test-deployer: test-deployer-setup ## Run deployer-based integration tests (skipping peering_commontopo).
	@cd ./test-integ && \
		NOLOGBUFFER=1 \
		TEST_LOG_LEVEL=debug \
		DEPLOYER_CONSUL_DATAPLANE_IMAGE=$(DEPLOYER_CONSUL_DATAPLANE_IMAGE) \
		gotestsum \
		--raw-command \
		--format=standard-verbose \
		--debug \
		-- \
		go test \
		-tags "$(GOTAGS)" \
		-timeout=20m \
		-json \
		$(shell sh -c "cd test-integ ; go list -tags \"$(GOTAGS)\" ./... | grep -v peering_commontopo") \
		--target-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--target-version local \
		--latest-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--latest-version latest

.PHONY: test-deployer-peering
test-deployer-peering: test-deployer-setup ## Run deployer-based integration tests (just peering_commontopo).
	@cd ./test-integ/peering_commontopo && \
		NOLOGBUFFER=1 \
		TEST_LOG_LEVEL=debug \
		DEPLOYER_CONSUL_DATAPLANE_IMAGE=$(DEPLOYER_CONSUL_DATAPLANE_IMAGE) \
		gotestsum \
		--raw-command \
		--format=standard-verbose \
		--debug \
		-- \
		go test \
		-tags "$(GOTAGS)" \
		-timeout=20m \
		-json \
		. \
		--target-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--target-version local \
		--latest-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--latest-version latest


.PHONY: test-compat-integ
test-compat-integ: test-compat-integ-setup ## Run consul-container based integration tests.
ifeq ("$(GOTESTSUM_PATH)","")
	@cd ./test/integration/consul-container && \
	go test \
		-v \
		-timeout=30m \
		./... \
		--tags $(GOTAGS) \
		--target-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--target-version local \
		--latest-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--latest-version latest
else
	@cd ./test/integration/consul-container && \
	gotestsum \
		--format=short-verbose \
		--debug \
		--rerun-fails=3 \
		--packages="./..." \
		-- \
		--tags $(GOTAGS) \
		-timeout=30m \
		./... \
		--target-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--target-version local \
		--latest-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--latest-version latest
endif

.PHONY: test-metrics-integ
test-metrics-integ: test-compat-integ-setup ## Test metrics integ
	@cd ./test/integration/consul-container && \
		go test -v -timeout=7m ./test/metrics \
		--target-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--target-version local \
		--latest-image $(CONSUL_COMPAT_TEST_IMAGE) \
		--latest-version latest

.PHONY: test-connect-ca-providers
test-connect-ca-providers: ## Running /agent/connect/ca tests in verbose mode
	@echo "Running /agent/connect/ca tests in verbose mode"
	@go test -v ./agent/connect/ca
	@go test -v ./agent/consul -run Vault
	@go test -v ./agent -run Vault

##@ UI

.PHONY: ui
ui: ui-docker ## Build the static web ui inside a Docker container. For local testing only; do not commit these assets.

.PHONY: ui-regen
ui-regen: ## Build the static web ui with yarn. This is the version to commit.
	cd $(CURDIR)/ui && make && cd ..
	rm -rf $(CURDIR)/agent/uiserver/dist
	mv $(CURDIR)/ui/packages/consul-ui/dist $(CURDIR)/agent/uiserver/

.PHONY: ui-build-image
ui-build-image: ## Building UI build container
	@echo "Building UI build container"
	@docker build $(NOCACHE) $(QUIET) -t $(UI_BUILD_TAG) - < build-support/docker/Build-UI.dockerfile

.PHONY: ui-docker
ui-docker: ui-build-image ## Builds ui within docker container and copy all the relevant artifacts out of the containers back to the source
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh ui

##@ Tools

.PHONY: tools
tools: ## Installs various supporting Go tools.
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh

.PHONY: lint-tools
lint-tools: ## Install tools for linting
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -lint

.PHONY: codegen-tools
codegen-tools: ## Install tools for codegen
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -codegen

.PHONY: codegen
codegen: codegen-tools ## Deep copy
	@$(SHELL) $(CURDIR)/agent/structs/deep-copy.sh
	@$(SHELL) $(CURDIR)/agent/proxycfg/deep-copy.sh
	@$(SHELL) $(CURDIR)/agent/consul/state/deep-copy.sh
	@$(SHELL) $(CURDIR)/agent/config/deep-copy.sh
	copywrite headers
	# Special case for MPL headers in /api and /sdk
	cd api && $(CURDIR)/build-support/scripts/copywrite-exceptions.sh
	cd sdk && $(CURDIR)/build-support/scripts/copywrite-exceptions.sh

print-%  : ; @echo $($*) ## utility to echo a makefile variable (i.e. 'make print-GOPATH')

.PHONY: module-versions
module-versions: ## Print a list of modules which can be updated. Columns are: module current_version date_of_current_version latest_version
	@go list -m -u -f '{{if .Update}} {{printf "%-50v %-40s" .Path .Version}} {{with .Time}} {{ .Format "2006-01-02" -}} {{else}} {{printf "%9s" ""}} {{end}}   {{ .Update.Version}} {{end}}' all

.PHONY: docs
docs: ## Point your web browser to http://localhost:3000/consul to live render docs from ./website/
	make -C website

##@ Release

.PHONY: version
version:  ## Current Consul version
	@echo -n "Version:                    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh
	@echo -n "Version + release:          "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r
	@echo -n "Version + git:              "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh  -g
	@echo -n "Version + release + git:    "
	@$(SHELL) $(CURDIR)/build-support/scripts/version.sh -r -g

.PHONY: docker-images
docker-images: go-build-image ui-build-image

.PHONY: go-build-image
go-build-image: ## Building Golang build container
	@echo "Building Golang build container"
	@docker build $(NOCACHE) $(QUIET) -t $(GO_BUILD_TAG) - < build-support/docker/Build-Go.dockerfile

.PHONY: consul-docker
consul-docker: go-build-image ## Builds consul in a docker container and then dumps executable into ./pkg/bin/...
	@$(SHELL) $(CURDIR)/build-support/scripts/build-docker.sh consul

.PHONY: docker-envoy-integ
docker-envoy-integ: ## Build image used to run integration tests locally.
	$(MAKE) GOARCH=amd64 linux
	docker build \
      --platform linux/amd64 $(NOCACHE) $(QUIET) \
      -t 'consul:local' \
      --build-arg CONSUL_IMAGE_VERSION=$(CONSUL_IMAGE_VERSION) \
      $(CURDIR)/pkg/bin/linux_amd64 \
      -f $(CURDIR)/build-support/docker/Consul-Dev.dockerfile

##@ Proto

.PHONY: proto
proto: proto-tools proto-gen proto-mocks ## Protobuf setup command

.PHONY: proto-tools
proto-tools: ## Install tools for protobuf
	@$(SHELL) $(CURDIR)/build-support/scripts/devtools.sh -protobuf

.PHONY: proto-gen
proto-gen: proto-tools ## Regenerates all Go files from protobuf definitions
	@$(SHELL) $(CURDIR)/build-support/scripts/protobuf.sh

.PHONY: proto-mocks
proto-mocks: ## Proto mocks
	@rm -rf grpcmocks/*
	@mockery --config .grpcmocks.yaml

.PHONY: proto-format
proto-format: proto-tools ## Proto format
	@buf format -w

.PHONY: proto-lint
proto-lint: proto-tools ## Proto lint
	@buf lint 
	@for fn in $$(find proto -name '*.proto'); do \
		if [[ "$$fn" = "proto/private/pbsubscribe/subscribe.proto" ]]; then \
			continue ; \
		elif [[ "$$fn" = "proto/private/pbpartition/partition.proto" ]]; then \
			continue ; \
		fi ; \
		pkg=$$(grep "^package " "$$fn" | sed 's/^package \(.*\);/\1/'); \
		if [[ "$$pkg" != hashicorp.consul.internal.* ]]; then \
			echo "ERROR: $$fn: is missing 'hashicorp.consul.internal' package prefix: $$pkg" >&2; \
			exit 1; \
		fi \
	done

##@ Envoy

.PHONY: envoy-library
envoy-library: ## Ensures that all of the protobuf packages present in the github.com/envoyproxy/go-control-plane library are referenced in the consul codebase
	@$(SHELL) $(CURDIR)/build-support/scripts/envoy-library-references.sh

.PHONY: envoy-regen
envoy-regen: ## Regenerating envoy golden files
	$(info regenerating envoy golden files)
	@for d in endpoints listeners routes clusters rbac; do \
		if [[ -d "agent/xds/testdata/$${d}" ]]; then \
			find "agent/xds/testdata/$${d}" -name '*.golden' -delete ; \
		fi \
	done
	@go test -tags '$(GOTAGS)' ./agent/xds -update
	@find "command/connect/envoy/testdata" -name '*.golden' -delete
	@go test -tags '$(GOTAGS)' ./command/connect/envoy -update

##@ Help

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
