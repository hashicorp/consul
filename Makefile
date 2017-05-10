# Metadata about this makefile and position
MKFILE_PATH := $(lastword $(MAKEFILE_LIST))
CURRENT_DIR := $(dir $(realpath $(MKFILE_PATH)))
CURRENT_DIR := $(CURRENT_DIR:/=)

# include any other makefiles, like enterprise overrides
-include *.mk

# Get the project metadata
GOVERSION ?= 1.8.1
PROJECT ?= github.com/hashicorp/consul
OWNER ?= $(dir $(PROJECT))
OWNER ?= $(notdir $(OWNER:/=))
NAME ?= $(notdir $(PROJECT))

EXTERNAL_TOOLS ?= \
	github.com/elazarl/go-bindata-assetfs/... \
	github.com/jteeuwen/go-bindata/... \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/stringer

# The root directory of the Go source
SOURCE_DIR ?= $(CURRENT_DIR)

# Specifies the path on disk to the version.go file. This is the
# full path. The rest define the remaining version information.
VERSION_FILE ?= $(SOURCE_DIR)/version/version_oss.go
VERSIONMAJ ?= $(shell awk -F\" '/Version/ { print $$2; exit }' "${VERSION_FILE}")
VERSIONPRE ?= $(shell awk -F\" '/VersionPre.*/ { print $$2; exit }' "${VERSION_FILE}")
VERSION ?= $(shell [[ "${VERSIONPRE}x" != "x" ]] && echo "${VERSIONMAJ}-${VERSIONPRE}" || echo "${VERSIONMAJ}")

# Current system information (this is the invoking system)
ME_OS = $(shell go env GOOS)
ME_ARCH = $(shell go env GOARCH)

# Default os-arch combination to build
XC_OS ?= solaris darwin freebsd linux windows
XC_ARCH ?= 386 amd64 arm
XC_EXCLUDE ?= darwin/arm solaris/386 solaris/arm windows/arm

# GPG Signing key (blank by default, means no GPG signing)
GPG_KEY ?=

# List of tests to run
TEST ?= ./...

# List all our actual files, excluding vendor
GOFILES = $(shell go list $(TEST) | grep -v /vendor/)

# Tags specific for building
GOTAGS ?= consul

# Number of procs to use
GOMAXPROCS ?= 4

# bin builds the project by invoking the compile script inside of a Docker
# container. Invokers can override the target OS or architecture using
# environment variables.
bin:
	@echo "==> Building ${PROJECT}..."
	@docker run \
		--interactive \
		--tty \
		--rm \
		--dns=8.8.8.8 \
		--env="VERSION=${VERSION}" \
		--env="PROJECT=${PROJECT}" \
		--env="OWNER=${OWNER}" \
		--env="NAME=${NAME}" \
		--env="GOMAXPROCS=${GOMAXPROCS}" \
		--env="GOTAGS=${GOTAGS}" \
		--env="XC_OS=${XC_OS}" \
		--env="XC_ARCH=${XC_ARCH}" \
		--env="XC_EXCLUDE=${XC_EXCLUDE}" \
		--env="DIST=${DIST}" \
		--workdir="/go/src/${PROJECT}" \
		--volume="${CURRENT_DIR}:/go/src/${PROJECT}" \
		"golang:${GOVERSION}" /usr/bin/env sh -c "scripts/compile.sh"

# bin-local builds the project using the local go environment. This is only
# recommended for advanced users or users who do not wish to use the Docker
# build process.
bin-local:
	@echo "==> Building ${PROJECT} (locally)..."
	@env \
		VERSION="${VERSION}" \
		PROJECT="${PROJECT}" \
		OWNER="${OWNER}" \
		NAME="${NAME}" \
		GOMAXPROCS="${GOMAXPROCS}" \
		GOTAGS="${GOTAGS}" \
		XC_OS="${XC_OS}" \
		XC_ARCH="${XC_ARCH}" \
		XC_EXCLUDE="${XC_EXCLUDE}" \
		DIST="${DIST}" \
		/usr/bin/env sh -c "scripts/compile.sh"

# bootstrap installs the necessary go tools for development or build
bootstrap:
	@echo "==> Bootstrapping ${PROJECT}..."
	@for t in ${EXTERNAL_TOOLS}; do \
		echo "--> Installing $$t" ; \
		go get -u "$$t"; \
	done

# deps gets all the dependencies for this repository and vendors them.
deps:
	@echo "==> Updating dependencies..."
	@docker run \
		--interactive \
		--tty \
		--rm \
		--dns=8.8.8.8 \
		--env="GOMAXPROCS=${GOMAXPROCS}" \
		--workdir="/go/src/${PROJECT}" \
		--volume="${CURRENT_DIR}:/go/src/${PROJECT}" \
		"golang:${GOVERSION}" /usr/bin/env sh -c "scripts/deps.sh"

# dev builds the project for the current system as defined by go env.
dev:
	@env \
		XC_OS="${ME_OS}" \
		XC_ARCH="${ME_ARCH}" \
		$(MAKE) -f "${MKFILE_PATH}" bin
	@echo "--> Moving into bin/"
	@mkdir -p "${CURRENT_DIR}/bin/"
	@cp "${CURRENT_DIR}/pkg/${ME_OS}_${ME_ARCH}/${NAME}" "${CURRENT_DIR}/bin/"
ifdef GOPATH
	@echo "--> Moving into GOPATH/"
	@mkdir -p "${GOPATH}/bin/"
	@cp "${CURRENT_DIR}/pkg/${ME_OS}_${ME_ARCH}/${NAME}" "${GOPATH}/bin/"
endif

# dist builds the binaries and then signs and packages them for distribution
dist:
ifndef GPG_KEY
	@echo "==> WARNING: No GPG key specified! Without a GPG key, this release"
	@echo "             will not be signed. Abort now to prevent building an"
	@echo "             unsigned release, or wait 5 seconds to continue."
	@echo ""
	@echo "--> Press CTRL + C to abort..."
	@sleep 5
endif
	@${MAKE} -f "${MKFILE_PATH}" bin DIST=1
	@echo "==> Tagging release (v${VERSION})..."
ifdef GPG_KEY
	@git commit --allow-empty --gpg-sign="${GPG_KEY}" -m "Release v${VERSION}"
	@git tag -a -m "Version ${VERSION}" -s -u "${GPG_KEY}" "v${VERSION}" master
	@gpg --default-key "${GPG_KEY}" --detach-sig "${CURRENT_DIR}/pkg/dist/${NAME}_${VERSION}_SHA256SUMS"
else
	@git commit --allow-empty -m "Release v${VERSION}"
	@git tag -a -m "Version ${VERSION}" "v${VERSION}" master
endif
	@echo "--> Do not forget to run:"
	@echo ""
	@echo "    git push && git push --tags"
	@echo ""
	@echo "And then upload the binaries in dist/ to GitHub!"

# docker builds the docker container
docker:
	@echo "==> Building container..."
	@docker build \
		--pull \
		--rm \
		--file="docker/Dockerfile" \
		--tag="${OWNER}/${NAME}" \
		--tag="${OWNER}/${NAME}:${VERSION}" \
		"${CURRENT_DIR}"

# docker-push pushes the container to the registry
docker-push:
	@echo "==> Pushing to Docker registry..."
	@docker push "${OWNER}/${NAME}:latest"
	@docker push "${OWNER}/${NAME}:${VERSION}"

# format formats all the go files
format:
	@echo "==> Foratting ${PROJECT}..."
	@go fmt ${GOFILES}

# generate runs the code generator
generate:
	@echo "==> Generating ${PROJECT}..."
	@go generate ${GOFILES}

# test runs the test suite
test:
	@echo "==> Testing ${PROJECT}..."
	@go test -timeout=60s -parallel=20 -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}

# test-race runs the race checker
test-race:
	@echo "==> Testing ${PROJECT} (race)..."
	@go test -timeout=60s -race -tags="${GOTAGS}" ${GOFILES} ${TESTARGS}

# ui builds the static website and compiles it into the static binary
ui:
	@echo "==> Building UI for ${PROJECT}..."
	@docker run \
		--interactive \
		--tty \
		--rm \
		--dns=8.8.8.8 \
		--workdir="/go/src/${PROJECT}" \
		--volume="${CURRENT_DIR}:/go/src/${PROJECT}" \
		"ruby:2.3" /usr/bin/env sh -c "ui/scripts/dist.sh"
	@echo "--> Bundling into binary..."
	@go-bindata-assetfs -pkg=agent -prefix=pkg "${CURRENT_DIR}/pkg/web_ui/..."
	@mv bindata_assetfs.go command/agent
	@$(MAKE) -f "${MKFILE_PATH}" format

vet:
	@echo "==> Vetting ${PROJECT}..."
	@go vet $(GOFILES); if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

.PHONY: bin bin-local bootstrap deps dev dist docker docker-push format generate test test-race ui vet
