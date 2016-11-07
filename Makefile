BUILD_VERBOSE := -v

TEST_VERBOSE := -v

DOCKER_IMAGE_NAME := $$USER/coredns
DOCKER_VERSION := $(shell grep 'coreVersion' coremain/version.go | awk '{ print $$3 }' | tr -d '"')

all: coredns

# Phony this to ensure we always build the binary.
# TODO: Add .go file dependencies.
.PHONY: coredns
coredns: deps
	go build $(BUILD_VERBOSE) -ldflags="-s -w"

.PHONY: docker
docker: deps
	CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w"
	docker build -t $(DOCKER_IMAGE_NAME) .
	docker tag $(DOCKER_IMAGE_NAME):latest $(DOCKER_IMAGE_NAME):$(DOCKER_VERSION)

.PHONY: deps
deps:
	go get ${BUILD_VERBOSE}

.PHONY: test
test: deps
	go test -race $(TEST_VERBOSE) ./test ./middleware/...

.PHONY: testk8s
testk8s: deps
	go test -race $(TEST_VERBOSE) -tags=k8s -run 'TestKubernetes' ./test ./middleware/kubernetes/...

.PHONY: coverage
coverage: deps
	set -e -x
	echo "" > coverage.txt
	for d in `go list ./... | grep -v vendor`; do \
		go test $(TEST_VERBOSE)  -tags 'etcd k8s' -race -coverprofile=cover.out -covermode=atomic -bench=. $$d || exit 1; \
		if [ -f cover.out ]; then \
			cat cover.out >> coverage.txt; \
			rm cover.out; \
		fi; \
	done

.PHONY: clean
clean:
	go clean
	rm -f coredns

.PHONY: distclean
distclean: clean
	# Clean all dependencies and build artifacts
	find $(GOPATH)/pkg -maxdepth 1 -mindepth 1 | xargs rm -rf
	find $(GOPATH)/bin -maxdepth 1 -mindepth 1 | xargs rm -rf

	find $(GOPATH)/src -maxdepth 1 -mindepth 1 | grep -v github | xargs rm -rf
	find $(GOPATH)/src -maxdepth 2 -mindepth 2 | grep -v miekg | xargs rm -rf
	find $(GOPATH)/src/github.com/miekg -maxdepth 1 -mindepth 1 \! -name \*coredns\* | xargs rm -rf
