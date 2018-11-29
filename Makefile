# Makefile for building CoreDNS
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
CHECKS:=check godeps
BUILDOPTS:=-v
GOPATH?=$(HOME)/go
PRESUBMIT:=core coremain plugin test request
MAKEPWD:=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CGO_ENABLED:=0

all: coredns

.PHONY: coredns
coredns: $(CHECKS)
	CGO_ENABLED=$(CGO_ENABLED) $(SYSTEM) go build $(BUILDOPTS) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)

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
	(cd $(GOPATH)/src/github.com/mholt/caddy              && git checkout -q v0.11.1)
	(cd $(GOPATH)/src/github.com/miekg/dns                && git checkout -q v1.1.0)
	(cd $(GOPATH)/src/github.com/prometheus/client_golang && git checkout -q v0.8.0)

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

.PHONY: dep-ensure
dep-ensure:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v
	dep prune -v
	find vendor -name '*_test.go' -delete
