ROOT:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all: build

deps: node_modules clean

clean:
	rm -rf ./tmp

# target for netlify ui previews
# Netlify Settings
# base-directory: ui-v2
# build command: make netlify
# publish directory: ui-v2/ui-dist
netlify: build-staging
	mkdir -p ui-dist/ui \
		&& mv dist/* ui-dist/ui/ \
		&& cp _redirects ui-dist/_redirects

build-staging: deps
	yarn run build:staging

build-ci: deps
	yarn run build:ci --output-path=dist

build: deps
	yarn run build

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

test-parallel: deps
	yarn run test:parallel


lint: deps
	yarn run lint:hbs && yarn run lint:js

format: deps
	yarn run format:js

steps:
	yarn run steps:list

node_modules: yarn.lock package.json
	yarn install

.PHONY: all deps build start test test-view lint format clean
