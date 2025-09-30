ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
CONSUL_UI_INSTALL_FLAGS?=

all: build

deps: ../../node_modules clean

# incase we ever need to clean anything again
clean:

build-staging: deps
	pnpm run build:staging

build-ci: deps
	pnpm run build:ci --output-path=dist

build: deps
	pnpm run build

build-debug:
	BROCCOLI_DEBUG=* $(MAKE) build

start: deps
	pnpm run start

start-consul: deps
	pnpm run start:consul

start-api: deps
	pnpm run start:api

# testing

# things with 'view' will open your browser for you
# otherwise its headless

# TODO(spatel): CE refactor
# oss is currently with namespace support disabled
test: deps test-node
	pnpm run test

test-view: deps test-node
	pnpm run test:view

test-oss: deps test-node
	pnpm run test:oss

test-oss-view: deps test-node
	pnpm run test:oss:view

# these run from CI, both with and without namespaces
test-ci: deps test-node
	pnpm run test:ci

test-oss-ci: deps test-node
	pnpm run test:oss:ci

# these tests are only possible to run outside of ember
# and use node + tape for a test harness
test-node:
	pnpm run test:node

test-coverage: deps
	pnpm run test:coverage

test-coverage-view: deps
	pnpm run test:coverage:view

test-coverage-ci: deps
	pnpm run test:coverage:ci

test-parallel: deps
	pnpm run test:parallel

lint: deps
	pnpm run lint

format: deps
	pnpm run format

steps:
	pnpm run steps:list

../../node_modules: ../../pnpm-lock.yaml package.json
	pnpm install $(CONSUL_UI_INSTALL_FLAGS)

.PHONY: all deps build start test test-view lint format clean
