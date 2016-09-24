#BUILD_VERBOSE :=
BUILD_VERBOSE := -v

#TEST_VERBOSE :=
TEST_VERBOSE := -v

DOCKER_IMAGE_NAME := $$USER/coredns

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

.PHONY: deps
deps:
	go get ${BUILD_VERBOSE}

.PHONY: test
test: deps
	go test $(TEST_VERBOSE) ./...

.PHONY: testk8s
testk8s: deps
	# With -args --v=100 the k8s API response data will be printed in the log:
	#go test $(TEST_VERBOSE) -tags=k8s -run 'TestK8sIntegration' ./test -args --v=100
	# Without the k8s API response data:
	go test $(TEST_VERBOSE) -tags=k8s -run 'TestK8sIntegration' ./test

.PHONY: testk8s-setup
testk8s-setup: deps
	go test -v ./middleware/kubernetes/... -run TestKubernetes

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
