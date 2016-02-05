GOTOOLS = github.com/mitchellh/gox golang.org/x/tools/cmd/stringer \
	github.com/jteeuwen/go-bindata/... github.com/elazarl/go-bindata-assetfs/...
DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES = $(shell go list ./...)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
VERSION?=$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)

all: deps format
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

deps:
	@echo "--> Installing build dependencies"
	@go get -v $(GOTOOLS)
	@go get -d -v ./... $(DEPS)

updatedeps: deps
	go get -u -v $(GOTOOLS)
	go list ./... \
		| xargs go list -f '{{join .Deps "\n"}}' \
		| grep -v github.com/hashicorp/consul \
		| grep -v '/internal/' \
		| sort -u \
		| xargs go get -f -u -v

test: deps
	@$(MAKE) vet
	@./scripts/verify_no_uuid.sh
	@./scripts/test.sh

cover: deps
	./scripts/verify_no_uuid.sh
	go list ./... | xargs -n1 go test --cover

format: deps
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
generate: deps
	find . -type f -name '.DS_Store' -delete
	go generate ./...

# generates the static web ui
static-assets: deps
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix pkg ./pkg/web_ui/...
	@mv bindata_assetfs.go command/agent
	$(MAKE) format

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all bin dev dist cov deps test vet web web-push generate test-nodep static-assets
