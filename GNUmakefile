GOTOOLS = \
	github.com/elazarl/go-bindata-assetfs/... \
	github.com/jteeuwen/go-bindata/... \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/stringer
TEST ?= ./...
GOTAGS ?= consul
GOFILES ?= $(shell go list $(TEST) | grep -v /vendor/)
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Get the git commit
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_DESCRIBE=$(shell git describe --tags --always)
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=-X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY) -X $(GIT_IMPORT).GitDescribe=$(GIT_DESCRIBE)

export GOLDFLAGS

# all builds binaries for all targets
all: bin

bin: tools
	@mkdir -p bin/
	@GOTAGS='$(GOTAGS)' sh -c "'$(CURDIR)/scripts/build.sh'"

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev:
	mkdir -p pkg/$(GOOS)_$(GOARCH)
	go install -ldflags '$(GOLDFLAGS)' -tags '$(GOTAGS)'
	cp $(GOPATH)/bin/consul bin
	cp $(GOPATH)/bin/consul pkg/$(GOOS)_$(GOARCH)

# dist builds binaries for all platforms and packages them for distribution
dist:
	@GOTAGS='$(GOTAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

cov:
	gocov test ${GOFILES} | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test:
	@./scripts/verify_no_uuid.sh
	@env \
		GOTAGS="${GOTAGS}" \
		GOFILES="${GOFILES}" \
		TESTARGS="${TESTARGS}" \
		sh -c "'$(CURDIR)/scripts/test.sh'"

cover:
	go test ${GOFILES} --cover

format:
	@echo "--> Running go fmt"
	@go fmt ${GOFILES}

vet:
	@echo "--> Running go vet"
	@go vet ${GOFILES}; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

# build the static web ui and build static assets inside a Docker container, the
# same way a release build works
ui:
	@sh -c "'$(CURDIR)/scripts/ui.sh'"

# generates the static web ui that's compiled into the binary
static-assets:
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix pkg ./pkg/web_ui/...
	@mv bindata_assetfs.go command/agent
	$(MAKE) format

tools:
	go get -u -v $(GOTOOLS)

.PHONY: all ci bin dev dist cov test cover format vet ui static-assets tools
