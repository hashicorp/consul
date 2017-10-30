# Makefile for building CoreDNS
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
CHECKS:=check godeps

all: coredns

.PHONY: coredns
coredns: $(CHECKS)
	CGO_ENABLED=0 $(SYSTEM) go build -v -ldflags="-s -w -X github.com/coredns/coredns/coremain.gitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: check
check: linter core/zplugin.go core/dnsserver/zdirectives.go godeps

.PHONY: test
test: check
	go test -race -v ./test ./plugin/...

.PHONY: testk8s
testk8s: check
	go test -race -v -tags=k8s -run 'TestKubernetes' ./test ./plugin/kubernetes/...

.PHONY: godeps
godeps:
	go get github.com/mholt/caddy
	go get github.com/miekg/dns
	go get golang.org/x/net/context
	go get golang.org/x/text

.PHONY: travis
travis: check
ifeq ($(TEST_TYPE),core)
	( cd request ; go test -v  -tags 'etcd' -race ./... )
	( cd core ; go test -v  -tags 'etcd' -race  ./... )
	( cd coremain go test -v  -tags 'etcd' -race ./... )
endif
ifeq ($(TEST_TYPE),integration)
	( cd test ; go test -v  -tags 'etcd' -race ./... )
endif
ifeq ($(TEST_TYPE),plugin)
	( cd plugin ; go test -v  -tags 'etcd' -race ./... )
endif
ifeq ($(TEST_TYPE),coverage)
	for d in `go list ./... | grep -v vendor`; do \
		t=$$(date +%s); \
		go test -i -tags 'etcd' -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		go test -v -tags 'etcd' -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		echo "Coverage test $$d took $$(($$(date +%s)-t)) seconds"; \
		if [ -f cover.out ]; then \
			cat cover.out >> coverage.txt; \
			rm cover.out; \
		fi; \
	done
endif

core/zplugin.go core/dnsserver/zdirectives.go: plugin.cfg
	go generate coredns.go

.PHONY: gen
gen:
	go generate coredns.go

.PHONY: linter
linter:
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install golint
	gometalinter --deadline=1m --disable-all --enable=gofmt --enable=golint --enable=vet --exclude=^vendor/ --exclude=^pb/ ./...

.PHONY: clean
clean:
	go clean
	rm -f coredns
