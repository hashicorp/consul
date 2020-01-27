DEPS = $(go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
ENV  = $(shell go env GOPATH)
GO_VERSION  = $(shell go version)
GOLANG_CI_VERSION = v1.19.0

# Look for versions prior to 1.10 which have a different fmt output
# and don't lint with gofmt against them.
ifneq (,$(findstring go version go1.8, $(GO_VERSION)))
	FMT=
else ifneq (,$(findstring go version go1.9, $(GO_VERSION)))
	FMT=
else
    FMT=--enable gofmt
endif

TEST_RESULTS_DIR?=/tmp/test-results

test:
	go test $(TESTARGS) -timeout=60s -race .
	go test $(TESTARGS) -timeout=60s -tags batchtest -race .

integ: test
	INTEG_TESTS=yes go test $(TESTARGS) -timeout=25s -run=Integ .
	INTEG_TESTS=yes go test $(TESTARGS) -timeout=25s -tags batchtest -run=Integ .

ci.test-norace:
	gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-test.xml -- -timeout=60s
	gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-test.xml -- -timeout=60s -tags batchtest

ci.test:
	gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-test.xml -- -timeout=60s -race .
	gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-test.xml -- -timeout=60s -race -tags batchtest .

ci.integ: ci.test
	INTEG_TESTS=yes gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-integ.xml -- -timeout=25s -run=Integ .
	INTEG_TESTS=yes gotestsum --format=short-verbose --junitfile $(TEST_RESULTS_DIR)/gotestsum-report-integ.xml -- -timeout=25s -run=Integ -tags batchtest .

fuzz:
	go test $(TESTARGS) -timeout=20m ./fuzzy
	go test $(TESTARGS) -timeout=20m -tags batchtest ./fuzzy

deps:
	go get -t -d -v ./...
	echo $(DEPS) | xargs -n1 go get -d

lint:
	gofmt -s -w .
	golangci-lint run -c .golangci-lint.yml $(FMT) .

dep-linter:
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(ENV)/bin $(GOLANG_CI_VERSION)

cov:
	INTEG_TESTS=yes gocov test github.com/hashicorp/raft | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

.PHONY: test cov integ deps dep-linter lint
