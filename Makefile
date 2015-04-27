TEST_DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

all: deps format
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps: godep
	@echo "--> Installing build dependencies (resets versions using godep)"
	@godep restore ./...

updatedeps:
	@echo "--> Updating build dependencies"
	@go get -u github.com/tools/godep
	@go get -d -f -u ./... $(TEST_DEPS)
	@rm -rf ./Godeps/_workspace
	@godep save ./...

test: deps
	@./scripts/verify_no_uuid.sh
	@./scripts/test.sh
	@$(MAKE) vet

integ:
	go list ./... | INTEG_TESTS=yes xargs -n1 go test

cover: deps
	./scripts/verify_no_uuid.sh
	go list ./... | xargs -n1 go test --cover

format: deps
	@echo "--> Running go fmt"
	@go fmt ./...

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

godep:
	@go get github.com/tools/godep

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

.PHONY: all cov deps integ test vet web web-push test-nodep godep
