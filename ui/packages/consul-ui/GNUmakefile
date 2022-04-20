ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
CONSUL_UI_INSTALL_FLAGS?=

all: build

deps: ../../node_modules clean

# incase we ever need to clean anything again
clean:

build-staging: deps
	yarn run build:staging

build-ci: deps
	yarn run build:ci --output-path=dist

build: deps
	yarn run build

build-debug:
	BROCCOLI_DEBUG=* $(MAKE) build

start: deps
	yarn run start

start-consul: deps
	yarn run start:consul

start-api: deps
	yarn run start:api

# testing

# things with 'view' will open your browser for you
# otherwise its headless

# oss is currently with namespace support disabled
test: deps test-node
	yarn run test

test-view: deps test-node
	yarn run test:view

test-oss: deps test-node
	yarn run test:oss

test-oss-view: deps test-node
	yarn run test:oss:view

# these run from CI, both with and without namespaces
test-ci: deps test-node
	yarn run test:ci

test-oss-ci: deps test-node
	yarn run test:oss:ci

# these tests are only possible to run outside of ember
# and use node + tape for a test harness
test-node:
	yarn run test:node

test-coverage: deps
	yarn run test:coverage

test-coverage-view: deps
	yarn run test:coverage:view

test-coverage-ci: deps
	yarn run test:coverage:ci

test-parallel: deps
	yarn run test:parallel

lint: deps
	yarn run lint

format: deps
	yarn run format

steps:
	yarn run steps:list

../../node_modules: ../../yarn.lock package.json
	yarn install $(CONSUL_UI_INSTALL_FLAGS)

.PHONY: all deps build start test test-view lint format clean
