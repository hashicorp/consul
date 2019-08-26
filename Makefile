# Makefile for building CoreDNS
GITCOMMIT:=$(shell git describe --dirty --always)
BINARY:=coredns
SYSTEM:=
CHECKS:=check
BUILDOPTS:=-v
GOPATH?=$(HOME)/go
PRESUBMIT:=core coremain plugin test request
MAKEPWD:=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CGO_ENABLED:=0

.PHONY: all
all: coredns

.PHONY: coredns
coredns: $(CHECKS)
	GO111MODULE=on CGO_ENABLED=$(CGO_ENABLED) $(SYSTEM) go build $(BUILDOPTS) -ldflags="-s -w -X github.com/coredns/coredns/coremain.GitCommit=$(GITCOMMIT)" -o $(BINARY)

.PHONY: check
check: presubmit core/plugin/zplugin.go core/dnsserver/zdirectives.go

.PHONY: travis
travis:
ifeq ($(TEST_TYPE),core)
	( cd request ; GO111MODULE=on go test -v -race ./... )
	( cd core ; GO111MODULE=on go test -v -race  ./... )
	( cd coremain ; GO111MODULE=on go test -v -race ./... )
endif
ifeq ($(TEST_TYPE),integration)
	( cd test ; GO111MODULE=on go test -v -race ./... )
endif
ifeq ($(TEST_TYPE),plugin)
	( cd plugin ; GO111MODULE=on go test -v -race ./... )
endif
ifeq ($(TEST_TYPE),coverage)
	for d in `go list ./... | grep -v vendor`; do \
		t=$$(date +%s); \
		GO111MODULE=on go test -i -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		GO111MODULE=on go test -v -coverprofile=cover.out -covermode=atomic $$d || exit 1; \
		echo "Coverage test $$d took $$(($$(date +%s)-t)) seconds"; \
		if [ -f cover.out ]; then \
			cat cover.out >> coverage.txt; \
			rm cover.out; \
		fi; \
	done
endif
ifeq ($(TEST_TYPE),fuzzit)
	# skip fuzzing for PR
	if [ "$(TRAVIS_PULL_REQUEST)" = "false" ] || [ "$(FUZZIT_TYPE)" = "local-regression" ] ; then \
		export GO111MODULE=off; \
		go get -u github.com/dvyukov/go-fuzz/go-fuzz-build; \
		go get -u -v .; \
		cd ../../go-acme/lego && git checkout v2.5.0; \
		cd ../../coredns/coredns; \
		LIBFUZZER=YES $(MAKE) -f Makefile.fuzz all; \
		$(MAKE) -sf Makefile.fuzz fuzzit; \
		for i in `$(MAKE) -sf Makefile.fuzz echo`; do echo $$i; \
			./fuzzit create job --type $(FUZZIT_TYPE) coredns/$$i ./$$i; \
		done; \
	fi;
endif

core/plugin/zplugin.go core/dnsserver/zdirectives.go: plugin.cfg
	GO111MODULE=on go generate coredns.go

.PHONY: gen
gen:
	GO111MODULE=on go generate coredns.go

.PHONY: pb
pb:
	$(MAKE) -C pb

# Presubmit runs all scripts in .presubmit; any non 0 exit code will fail the build.
.PHONY: presubmit
presubmit:
	@for pre in $(MAKEPWD)/.presubmit/* ; do "$$pre" $(PRESUBMIT) || exit 1 ; done

.PHONY: clean
clean:
	GO111MODULE=on go clean
	rm -f coredns
