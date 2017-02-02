GOTOOLS = \
	github.com/elazarl/go-bindata-assetfs/... \
	github.com/jteeuwen/go-bindata/... \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/stringer
PACKAGES=$(shell go list ./... | grep -v '/vendor/')
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
BUILD_TAGS?=consul

# all builds binaries for all targets
all: bin

ci:
	if [ "${TRAVIS_PULL_REQUEST}" = "false" ]; then \
		$(MAKE) bin ;\
	fi
	@$(MAKE) test

bin: tools
	@mkdir -p bin/
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/build.sh'"

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev: format
	@CONSUL_DEV=1 BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/build.sh'"

# dist builds binaries for all platforms and packages them for distribution
dist:
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test: format
	@$(MAKE) vet
	@./scripts/verify_no_uuid.sh
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/test.sh'"

cover:
	go list ./... | xargs -n1 go test --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

vet:
	@echo "--> Running go tool vet $(VETARGS) ."
	@go list ./... \
		| grep -v /vendor/ \
		| cut -d '/' -f 4- \
		| xargs -n1 \
			go tool vet $(VETARGS) ;\
	if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
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
