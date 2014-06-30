DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES = $(shell go list ./...)

all: deps format
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./...
	@echo $(DEPS) | xargs -n1 go get -d

test: deps
	go list ./... | xargs -n1 go test

integ:
	go list ./... | INTEG_TESTS=yes xargs -n1 go test

format: deps
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all cov deps integ test web web-push
