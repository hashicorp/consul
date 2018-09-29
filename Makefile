# Makefile for building CoreDNS
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
CHECKS:=check godeps
VERBOSE:=-v
GOPATH?=$(HOME)/go
PRESUBMIT:=core coremain plugin test request
MAKEPWD:=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))

all: coredns

.PHONY: coredns
coredns: $(CHECKS)
	CGO_ENABLED=0 $(SYSTEM) go build $(VERBOSE) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: check
check: presubmit core/zplugin.go core/dnsserver/zdirectives.go godeps

.PHONY: godeps
godeps:
	@ # Not vendoring these, so external plugins compile, avoiding:
	@ # cannot use c (type *"github.com/mholt/caddy".Controller) as type
	@ # *"github.com/coredns/coredns/vendor/github.com/mholt/caddy".Controller like errors.
	(cd $(GOPATH)/src/github.com/mholt/caddy 2>/dev/null              && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/github.com/miekg/dns 2>/dev/null                && git checkout -q master 2>/dev/null || true)
	(cd $(GOPATH)/src/github.com/prometheus/client_golang 2>/dev/null && git checkout -q master 2>/dev/null || true)
	go get -u github.com/mholt/caddy
	go get -u github.com/miekg/dns
	go get -u github.com/prometheus/client_golang/prometheus/promhttp
	go get -u github.com/prometheus/client_golang/prometheus
	(cd $(GOPATH)/src/github.com/mholt/caddy              && git checkout -q v0.10.11)
	(cd $(GOPATH)/src/github.com/miekg/dns                && git checkout -q v1.0.12)
	(cd $(GOPATH)/src/github.com/prometheus/client_golang && git checkout -q v0.8.0)
	@ # for travis only, if this fails we don't care, but don't see benchmarks
	 go get -u golang.org/x/tools/cmd/benchcmp || true

.PHONY: travis
travis:
ifeq ($(TEST_TYPE),core)
	( cd request ; go test -v  -tags 'etcd' -race ./... )
	( cd core ; go test -v  -tags 'etcd' -race  ./... )
	( cd coremain ; go test -v  -tags 'etcd' -race ./... )
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
ifeq ($(TEST_TYPE),benchmark)
	> new
	( cd plugin; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> new
	( cd request; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> new
	( cd core; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> new
	( cd coremain; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> new
	git checkout master
	> old
	( cd plugin; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> old
	( cd request; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> old
	( cd core; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> old
	( cd coremain; go test -run=NONE -bench=. -benchmem=true -tags 'etcd' ./... ) >> old
	if command -v benchcmp; then benchcmp old new > .benchmark.log ; cat .benchmark.log ; fi
	git checkout -
endif

core/zplugin.go core/dnsserver/zdirectives.go: plugin.cfg
	go generate coredns.go

.PHONY: gen
gen:
	go generate coredns.go

.PHONY: pb
pb:
	$(MAKE) -C pb

# Presubmit runs all scripts in .presubmit; any non 0 exit code will fail the build.
.PHONY: presubmit
presubmit:
	@for pre in $(MAKEPWD)/.presubmit/* ; do "$$pre" $(PRESUBMIT) || exit 1 ; done

.PHONY: clean
clean:
	go clean
	rm -f coredns
