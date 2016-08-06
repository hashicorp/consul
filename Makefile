#BUILD_VERBOSE :=
BUILD_VERBOSE := -v

TEST_VERBOSE :=
TEST_VERBOSE := -v

DOCKER_IMAGE_NAME := $$USER/coredns


all:
	go build $(BUILD_VERBOSE) -ldflags="-s -w"

.PHONY: docker
docker: all
	GOOS=linux go build -a -tags netgo -installsuffix netgo -ldflags="-s -w"
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: deps
deps:
	go get ${BUILD_VERBOSE}

.PHONY: test
test:
	go test $(TEST_VERBOSE) ./...

.PHONY: testk8s
testk8s:
	# With -args --v=100 the k8s API response data will be printed in the log:
	#go test $(TEST_VERBOSE) -tags=k8s -run 'TestK8sIntegration' ./test -args --v=100
	# Without the k8s API response data:
	go test $(TEST_VERBOSE) -tags=k8s -run 'TestK8sIntegration' ./test

.PHONY: testk8s-setup
testk8s-setup:
	go test -v ./core/setup -run TestKubernetes

.PHONY: clean
clean:
	go clean
