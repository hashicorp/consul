DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)

all: deps
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps:
	go get -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

test: deps
	go list ./... | xargs -n1 go test

integ:
	go list ./... | INTEG_TESTS=yes xargs -n1 go test

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PNONY: all cov deps integ test web web-push
