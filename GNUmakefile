GOTOOLS = \
	github.com/elazarl/go-bindata-assetfs/... \
	github.com/jteeuwen/go-bindata/... \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/stringer
PACKAGES=$(shell go list ./... | grep -v '^github.com/hashicorp/consul/vendor/')
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
VERSION?=$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)

# all builds binaries for all targets
all: tools
	@mkdir -p bin/
	@sh -c "'$(CURDIR)/scripts/build.sh'"

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev: format
	@CONSUL_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

# dist builds binaries for all platforms and packages them for distribution
dist:
	@sh -c "'$(CURDIR)/scripts/dist.sh' $(VERSION)"

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test: format
	@$(MAKE) vet
	@./scripts/verify_no_uuid.sh
	@./scripts/test.sh

cover:
	go list ./... | xargs -n1 go test --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

vet:
	@echo "--> Running go tool vet $(VETARGS) ."
	@go list ./... \
		| grep -v ^github.com/hashicorp/consul/vendor/ \
		| cut -d '/' -f 4- \
		| xargs -n1 \
			go tool vet $(VETARGS) ;\
	if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
	fi

# generates the static web ui that's compiled into the binary
static-assets:
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix pkg ./pkg/web_ui/...
	@mv bindata_assetfs.go command/agent
	$(MAKE) format

tools:
	go get -u -v $(GOTOOLS)

.PHONY: all bin dev dist cov test cover format vet static-assets tools
