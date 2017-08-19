GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=

all: coredns

# Phony this to ensure we always build the binary.
# TODO: Add .go file dependencies.
.PHONY: coredns
coredns: check godeps
	CGO_ENABLED=0 $(SYSTEM) go build -v -ldflags="-s -w -X github.com/coredns/coredns/coremain.gitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: check
check: fmt core/zmiddleware.go core/dnsserver/zdirectives.go godeps

.PHONY: test
test: check
	go test -race -v ./test ./middleware/...

.PHONY: testk8s
testk8s: check
	go test -race -v -tags=k8s -run 'TestKubernetes' ./test ./middleware/kubernetes/...

.PHONY: godeps
godeps:
	go get github.com/mholt/caddy
	go get github.com/miekg/dns
	go get golang.org/x/net/context
	go get golang.org/x/text

.PHONY: travis
travis: check
ifeq ($(TEST_TYPE),core)
	( cd request ; go test -v  -tags 'etcd k8s' -race ./... )
	( cd core ; go test -v  -tags 'etcd k8s' -race  ./... )
	( cd coremain go test -v  -tags 'etcd k8s' -race ./... )
endif
ifeq ($(TEST_TYPE),integration)
	( cd test ; go test -v  -tags 'etcd k8s' -race ./... )
endif
ifeq ($(TEST_TYPE),middleware)
	( cd middleware ; go test -v  -tags 'etcd k8s' -race ./... )
endif
ifeq ($(TEST_TYPE),coverage)
	for d in `go list ./... | grep -v vendor`; do \
		t=$$(date +%s); \
		go test -i -tags 'etcd k8s' -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		go test -v -tags 'etcd k8s' -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		echo "Coverage test $$d took $$(($$(date +%s)-t)) seconds"; \
		if [ -f cover.out ]; then \
			cat cover.out >> coverage.txt; \
			rm cover.out; \
		fi; \
	done
endif


core/zmiddleware.go core/dnsserver/zdirectives.go: middleware.cfg
	go generate coredns.go

.PHONY: gen
gen:
	go generate coredns.go

.PHONY: fmt
fmt:
	## run go fmt
	@test -z "$$(find . -type d | grep -vE '(/vendor|^\.$$|/.git|/.travis)' | xargs gofmt -s -l  | tee /dev/stderr)" || \
		(echo "please format Go code with 'gofmt -s -w'" && false)

.PHONY: lint
lint:
	go get -u github.com/golang/lint/golint
	@test -z "$$(find . -type d | grep -vE '(/vendor|^\.$$|/.git|/.travis)' | grep -vE '(^\./pb)' | xargs golint \
		| grep -vE "context\.Context should be the first parameter of a function" | tee /dev/stderr)"

.PHONY: clean
clean:
	go clean
	rm -f coredns
