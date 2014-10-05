DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES = $(shell go list ./...)
VERSION = $(shell grep "const Version =" version.go | awk -F'"' '{print $$2}')
PRERELEASE = $(shell grep "const VersionPre(r|R)elease =" version.go | awk -F'"' '{print $$2}')

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

debcommon:
	./scripts/deb_version.sh $(VERSION) $(PRERELEASE)

debsource: debcommon
	debuild -I -us -uc -tc -S

deb: debcommon
	debuild -I -us -uc -tc

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all cov deps integ test web web-push debsource deb
