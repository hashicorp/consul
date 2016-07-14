#BUILD_VERBOSE :=
BUILD_VERBOSE := -v

TEST_VERBOSE :=
TEST_VERBOSE := -v

all:
	go build $(BUILD_VERBOSE)

.PHONY: docker
docker:
	GOOS=linux go build -a -tags netgo -installsuffix netgo
	docker build -t $$USER/coredns .

.PHONY: deps
deps:
	go get ${BUILD_VERBOSE}

.PHONY: test
test:
	go test $(TEST_VERBOSE) ./...

.PHONY: testk8s
testk8s:
#	go test $(TEST_VERBOSE) -tags=k8sIntegration ./...
	go test $(TEST_VERBOSE) -tags=k8sIntegration -run 'TestK8sIntegration' ./test

.PHONY: clean
clean:
	go clean
