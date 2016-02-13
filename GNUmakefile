GOTOOLS = github.com/mitchellh/gox golang.org/x/tools/cmd/stringer \
	github.com/jteeuwen/go-bindata/... github.com/elazarl/go-bindata-assetfs/...
PACKAGES=$(shell go list ./... | grep -v '^github.com/hashicorp/consul/vendor/')
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
VERSION?=$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)

all: format tools
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

# bin generates the releasable binaries
bin: generate
	@sh -c "'$(CURDIR)/scripts/build.sh'"

# dev creates binaries for testing locally - these are put into ./bin and $GOPATH
dev: generate
	@CONSUL_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

# dist creates the binaries for distibution
dist: bin
	@sh -c "'$(CURDIR)/scripts/dist.sh' $(VERSION)"

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test:
	@$(MAKE) vet
	@./scripts/verify_no_uuid.sh
	@./scripts/test.sh

cover:
	./scripts/verify_no_uuid.sh
	go list ./... | xargs -n1 go test --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

vet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@echo "--> Running go tool vet $(VETARGS) ."
	@go tool vet $(VETARGS) . ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
	fi

# generate runs `go generate` to build the dynamically generated source files
generate:
	find . -type f -name '.DS_Store' -delete
	go generate ./...

# generates the static web ui
static-assets:
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix pkg ./pkg/web_ui/...
	@mv bindata_assetfs.go command/agent
	$(MAKE) format

tools:
	go get -u -v $(GOTOOLS)

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all bin dev dist cov test vet web web-push generate static-assets tools
