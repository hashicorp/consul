BUILD_VERBOSE := -v

TEST_VERBOSE := -v

all: coredns

# Phony this to ensure we always build the binary.
# TODO: Add .go file dependencies.
.PHONY: coredns
coredns: check
	go build $(BUILD_VERBOSE) -ldflags="-s -w"

.PHONY: deps
deps: core/zmiddleware.go core/dnsserver/zdirectives.go
	go get ${BUILD_VERBOSE}
	go get -u github.com/golang/lint/golint

.PHONY: check
check: fmt deps

.PHONY: test
test: check
	go test -race $(TEST_VERBOSE) ./test ./middleware/...

.PHONY: testk8s
testk8s: check
	go test -race $(TEST_VERBOSE) -tags=k8s -run 'TestKubernetes' ./test ./middleware/kubernetes/...

.PHONY: coverage
coverage: check
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

core/zmiddleware.go core/dnsserver/zdirectives.go: middleware.cfg
	go generate coredns.go

.PHONY: gen
gen:
	go generate coredns.go

.PHONY: fmt
fmt:
	## run go fmt
	@test -z "$$(gofmt -s -l . | grep -v vendor/ | tee /dev/stderr)" || \
		(echo "please format Go code with 'gofmt -s -w'" && false)

.PHONY: lint
lint: deps
	## run go lint, suggestion only (not enforced)
	@test -z "$$(golint ./... | grep -v vendor/ | grep -v ".pb.go:" | grep -vE "context\.Context should be the first parameter of a function" | tee /dev/stderr)"

.PHONY: distclean
distclean: clean
	# Clean all dependencies and build artifacts
	find $(GOPATH)/pkg -maxdepth 1 -mindepth 1 | xargs rm -rf
	find $(GOPATH)/bin -maxdepth 1 -mindepth 1 | xargs rm -rf

	find $(GOPATH)/src -maxdepth 1 -mindepth 1 | grep -v github | xargs rm -rf
	find $(GOPATH)/src -maxdepth 2 -mindepth 2 | grep -v miekg | xargs rm -rf
	find $(GOPATH)/src/github.com/miekg -maxdepth 1 -mindepth 1 \! -name \*coredns\* | xargs rm -rf
