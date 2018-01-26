# Makefile for building CoreDNS
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
CHECKS:=check godeps
VERBOSE:=-v

all: coredns

.PHONY: coredns
coredns: $(CHECKS)
	CGO_ENABLED=0 $(SYSTEM) go build $(VERBOSE) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: check
check: linter core/zplugin.go core/dnsserver/zdirectives.go godeps

.PHONY: test
test: check
	go test -race $(VERBOSE) ./test ./plugin/...

.PHONY: testk8s
testk8s: check
	go test -race $(VERBOSE) -tags=k8s -run 'TestKubernetes' ./test ./plugin/kubernetes/...

.PHONY: godeps
godeps:
	(cd $(GOPATH)/src/github.com/mholt/caddy 2>/dev/null              && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/github.com/miekg/dns 2>/dev/null                && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/github.com/prometheus/client_golang 2>/dev/null && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/golang.org/x/net 2>/dev/null                    && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/golang.org/x/text 2>/dev/null                   && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/github.com/coredns/forward 2>/dev/null          && git checkout -q master 2>/dev/null || true)
	go get -u github.com/mholt/caddy
	go get -u github.com/miekg/dns
	go get -u github.com/prometheus/client_golang/prometheus/promhttp
	go get -u github.com/prometheus/client_golang/prometheus
	go get -u golang.org/x/net/context
	go get -u golang.org/x/text
	-go get -f -u github.com/coredns/forward
	(cd $(GOPATH)/src/github.com/mholt/caddy              && git checkout -q v0.10.10)
	(cd $(GOPATH)/src/github.com/miekg/dns                && git checkout -q v1.0.4)
	(cd $(GOPATH)/src/github.com/prometheus/client_golang && git checkout -q v0.8.0)
	(cd $(GOPATH)/src/golang.org/x/net                    && git checkout -q release-branch.go1.9)
	(cd $(GOPATH)/src/golang.org/x/text                   && git checkout -q e19ae1496984b1c655b8044a65c0300a3c878dd3)
	(cd $(GOPATH)/src/github.com/coredns/forward          && git checkout -q v0.0.2)

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
