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
	yarn run build-ci --output-path=dist

build: deps
	yarn run build

start: deps
	yarn run start

start-api: deps
	yarn run start:api

test: deps
	yarn run test

test-view: deps
	yarn run test:view

test-parallel: deps
	yarn test-parallel

lint: deps
	yarn run lint:js

format: deps
	yarn run format:js

steps:
	yarn run steps:list

node_modules: yarn.lock package.json
	yarn install

.PHONY: all deps build start test test-view lint format clean
